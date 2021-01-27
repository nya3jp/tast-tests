// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility
// features and settings.
package a11y

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	chromeVoxExtensionURL = "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/chromevox/background/background.html"
	googleTtsExtensionID  = "gjjabgpgjpampikjhjpfhneeoapjbjaf"
)

// Feature represents an accessibility feature in Chrome OS.
type Feature string

// List of accessibility features.
const (
	DockedMagnifier Feature = "dockedMagnifier"
	FocusHighlight  Feature = "focusHighlight"
	ScreenMagnifier Feature = "screenMagnifier"
	SelectToSpeak   Feature = "selectToSpeak"
	SpokenFeedback  Feature = "spokenFeedback"
	SwitchAccess    Feature = "switchAccess"
)

// SetFeatureEnabled sets the specified accessibility feature enabled/disabled using the provided connection to the extension.
func SetFeatureEnabled(ctx context.Context, tconn *chrome.TestConn, feature Feature, enable bool) error {
	if err := tconn.Call(ctx, nil, `(feature, enable) => {
      return tast.promisify(tast.bind(chrome.accessibilityFeatures[feature], "set"))({value: enable});
    }`, feature, enable); err != nil {
		return errors.Wrapf(err, "failed to toggle %v to %t", feature, enable)
	}
	return nil
}

// ChromeVoxConn represents a connection to the ChromeVox background page.
type ChromeVoxConn struct {
	*chrome.Conn
}

// NewChromeVoxConn returns a connection to the ChromeVox extension's background page.
// If the extension is not ready, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func NewChromeVoxConn(ctx context.Context, c *chrome.Chrome) (*ChromeVoxConn, error) {
	extConn, err := c.NewConnForTarget(ctx, chrome.MatchTargetURL(chromeVoxExtensionURL))
	if err != nil {
		return nil, err
	}

	if err := func() error {
		// Poll until ChromeVox connection finishes loading.
		if err := extConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
			return errors.Wrap(err, "timed out waiting for ChromeVox connection to be ready")
		}

		// Ensure that we don't attempt to use the extension before its APIs are
		// available: https://crbug.com/789313.
		if err := extConn.WaitForExpr(ctx, "ChromeVoxState.instance"); err != nil {
			return errors.Wrap(err, "ChromeVox unavailable")
		}

		if err := chrome.AddTastLibrary(ctx, extConn); err != nil {
			return errors.Wrap(err, "failed to introduce tast library")
		}
		return nil
	}(); err != nil {
		extConn.Close()
		return nil, err
	}

	return &ChromeVoxConn{extConn}, nil
}

// focusedNode returns the currently focused node of ChromeVox.
// The returned node should be release by the caller.
func (cv *ChromeVoxConn) focusedNode(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	obj := &chrome.JSObject{}
	if err := cv.Eval(ctx, "ChromeVoxState.instance.currentRange.start.node", obj); err != nil {
		return nil, err
	}
	return ui.NewNode(ctx, tconn, obj)
}

// WaitForFocusedNode polls until the properties of the focused node matches the given params.
// timeout specifies the timeout to use when polling.
func (cv *ChromeVoxConn) WaitForFocusedNode(ctx context.Context, tconn *chrome.TestConn, params *ui.FindParams, timeout time.Duration) error {
	// Wait for focusClassName to receive focus.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		focused, err := cv.focusedNode(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		defer focused.Release(ctx)

		if match, err := focused.Matches(ctx, *params); err != nil {
			return testing.PollBreak(err)
		} else if !match {
			return errors.Errorf("focused node is incorrect: got %v, want %v", focused, params)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to get current focus")
	}
	return nil
}

// doCommand performs a ChromeVox command.
func (cv *ChromeVoxConn) doCommand(ctx context.Context, cmd string) error {
	expr := fmt.Sprintf("CommandHandler.onCommand('%s');", cmd)
	return cv.Eval(ctx, expr, nil)
}

// DoCommandAndExpectSpeech performs a ChromeVox command and adds the utterances
// to an expected list of utterances.
// Note: Ensure that the last item in utterances is the very last utterance spoken,
// since it is used for synchronization purposes.
func (cv *ChromeVoxConn) DoCommandAndExpectSpeech(ctx context.Context, tts *GoogleTTSConn, cmd string, utterances []string) error {
	tts.expectSpeech(ctx, utterances)
	if err := cv.doCommand(ctx, cmd); err != nil {
		return errors.Wrapf(err, "failed to perform the ChromeVox command: %s", cmd)
	}

	lastUtterance := utterances[len(utterances)-1]
	if err := tts.waitForUtterance(ctx, lastUtterance); err != nil {
		return errors.Wrap(err, "error waiting for last utterance")
	}

	return nil
}

// GoogleTTSConn represents a connection to the Google TTS extension background page.
type GoogleTTSConn struct {
	*chrome.Conn
	Expected []string
}

// NewGoogleTTSConn returns a connection to the Google TTS extension's background page.
// If the extension is not ready, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func NewGoogleTTSConn(ctx context.Context, c *chrome.Chrome) (ttsConn *GoogleTTSConn, retErr error) {
	bgURL := chrome.ExtensionBackgroundPageURL(googleTtsExtensionID)
	targets, err := c.FindTargets(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return nil, err
	}
	if len(targets) > 1 {
		for _, t := range targets[1:] {
			// Close all but one instance of the Google TTS engine background page.
			// We must do this because because trying to connect when there are multiple
			// instances triggers the following error:
			// Error: X targets matched while unique match was expected.
			c.CloseTarget(ctx, t.TargetID)
		}
	}

	var extConn *chrome.Conn
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		extConn, err = c.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
		return err
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to create a connection to the Google TTS background page")
	}

	defer func() {
		if retErr != nil {
			extConn.Close()
		}
	}()

	if err := extConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "timed out waiting for the Google TTS engine background page to load")
	}

	if err := extConn.WaitForExpr(ctx, "engine.initialized_"); err != nil {
		return nil, errors.Wrap(err, "timed out waiting for the Google TTS engine to initialize")
	}

	if err := extConn.Eval(ctx, `
    window.testUtterances = [];
    chrome.ttsEngine.onSpeak.addListener(utterance => testUtterances.push(utterance));
`, nil); err != nil {
		return nil, errors.Wrap(err, "failed to inject code to accumulate utterances")
	}

	return &GoogleTTSConn{extConn, []string{}}, nil
}

// expectSpeech adds utterances to the list of expected utterances.
func (tts *GoogleTTSConn) expectSpeech(ctx context.Context, utterances []string) {
	for _, utterance := range utterances {
		tts.Expected = append(tts.Expected, utterance)
	}
}

// waitForUtterance is used for synchronization purposes. It takes the last
// utterance that should be spoken as a result of an action and waits for it,
// ensuring that all utterances are spoken before performing the next action.
// We need to wait for the last utterance since waiting for an intermediate
// utterance could cause flakes.
func (tts *GoogleTTSConn) waitForUtterance(ctx context.Context, utterance string) error {
	expr := fmt.Sprintf(`window.testUtterances.length > 0 && window.testUtterances[window.testUtterances.length - 1] === "%s"`, utterance)
	if err := tts.WaitForExpr(ctx, expr); err != nil {
		return errors.Wrapf(err, "timed out waiting for utterance: %s", utterance)
	}

	return nil
}

// VerifyUtterances ensures that all expected utterances were spoken by the
// TTS engine.
func (tts *GoogleTTSConn) VerifyUtterances(ctx context.Context) error {
	var actual []string
	if err := tts.Eval(ctx, "window.testUtterances", &actual); err != nil {
		return errors.Wrap(err, "failed to retrieve actual utterances")
	}

	for _, exp := range tts.Expected {
		found := -1
		for i, act := range actual {
			if exp == act {
				found = i
				break
			}
		}

		if found == -1 {
			return errors.Errorf("Utterance expected but not spoken: %s", exp)
		}

		actual = actual[found+1:]
	}

	return nil
}

// PressKeysAndExpectSpeech presses keys and adds utterances to the list of
// expected utterances.
// Note: Ensure that the last item in utterances is the very last utterance spoken,
// since it is used for synchronization purposes.
func PressKeysAndExpectSpeech(ctx context.Context, tts *GoogleTTSConn, keys string, utterances []string) error {
	// Open a connection to the keyboard.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error with creating EventWriter from keyboard")
	}
	defer ew.Close()

	tts.expectSpeech(ctx, utterances)
	if err := ew.Accel(ctx, keys); err != nil {
		return errors.Wrapf(err, "error when pressing the keys: %s", keys)
	}

	lastUtterance := utterances[len(utterances)-1]
	if err := tts.waitForUtterance(ctx, lastUtterance); err != nil {
		return errors.Wrap(err, "error waiting for last utterance")
	}

	return nil
}
