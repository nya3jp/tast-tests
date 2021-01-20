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
	"chromiumos/tast/local/chrome/cdputil"
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

// GoogleTtsConn represents a connection to the Google TTS extension background page.
type GoogleTtsConn struct {
	*chrome.Conn
}

// NewGoogleTtsConn returns a connection to the Google TTS extension's background page.
// If the extension is not ready, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func NewGoogleTtsConn(ctx context.Context, c *chrome.Chrome) (*GoogleTtsConn, error) {
	devsess, err := cdputil.NewSession(ctx, cdputil.DebuggingPortPath, cdputil.WaitPort)
	if err != nil {
		return nil, err
	}
	bgURL := chrome.ExtensionBackgroundPageURL(googleTtsExtensionID)
	targets, err := devsess.FindTargets(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return nil, err
	}
	if len(targets) > 1 {
		for _, t := range targets[1:] {
			// Close all but one instance of the Google TTS engine background page.
			// We must do this because because trying to connect when there are multiple
			// instances triggers the following error:
			// Error: X targets matched while unique match was expected.
			devsess.CloseTarget(ctx, t.TargetID)
		}
	}

	var extConn *chrome.Conn
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		extConn, err = c.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
		return err
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to create a connection to the Google TTS background page")
	}

	if err := extConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		extConn.Close()
		return nil, errors.Wrap(err, "timed out waiting for the Google TTS engine background page to load")
	}

	if err := extConn.WaitForExpr(ctx, "engine.initialized_"); err != nil {
		extConn.Close()
		return nil, errors.Wrap(err, "timed out waiting for the Google TTS engine to initialize")
	}

	return &GoogleTtsConn{extConn}, nil
}

// expectSpeech verifies that the given utterances are spoken by the Google TTS
// engine.
// Note: this function always runs in a separate goroutine, since we need to
// begin waiting for utterances before ChromeVox commands are performed.
// ch1 is used to communicate that precondition checks have been met and the
// goroutine is ready.
// ch2 is used to communicate errors, if any.
// Also note that utterances should always be a non-empty array.
func (tts *GoogleTtsConn) expectSpeech(ctx context.Context, ch1 chan bool, ch2 chan error, utterances []string) {
	var precondition bool
	if err := tts.Eval(ctx, fmt.Sprintf(`engine.utterance_ !== "%s"`, utterances[0]), &precondition); err != nil {
		ch1 <- false
		ch2 <- err
		return
	}

	if !precondition {
		ch1 <- false
		ch2 <- errors.New("the first expected utterance must not match the speech engine's current utterance")
		return
	}

	ch1 <- true
	for _, utterance := range utterances {
		expr := fmt.Sprintf(`engine.utterance_ === "%s"`, utterance)
		if err := tts.WaitForExpr(ctx, expr); err != nil {
			ch2 <- errors.Wrapf(err, "timed out waiting for utterance: %s", utterance)
		}
	}

	ch2 <- nil
}

// DoCommandAndExpectSpeech performs a ChromeVox command and verifies that the
// utterances are spoken by the Google TTS engine.
func (cv *ChromeVoxConn) DoCommandAndExpectSpeech(ctx context.Context, tts *GoogleTtsConn, cmd string, utterances []string) error {
	// Use a goroutine to verify that an action produces the correct speech.
	// We need to begin waiting for utterances before speech is given to avoid
	// any flakiness.
	// The first channel is used to communicate that the goroutine is ready.
	// The second channel is used to communicate errors, if any.
	ch1 := make(chan bool)
	ch2 := make(chan error)
	go tts.expectSpeech(ctx, ch1, ch2, utterances)
	if ready := <-ch1; ready != true {
		return <-ch2
	}
	if err := cv.doCommand(ctx, cmd); err != nil {
		return errors.Wrapf(err, "failed to perform the ChromeVox command: %s", cmd)
	}
	if err := <-ch2; err != nil {
		return errors.Wrap(err, "failed to verify speech from the Google TTS engine")
	}

	return nil
}

// PressKeysAndExpectSpeech presses keys and verifies that the utterances are
// spoken by the Google TTS engine.
func PressKeysAndExpectSpeech(ctx context.Context, tts *GoogleTtsConn, keys string, utterances []string) error {
	// Open a connection to the keyboard.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error with creating EventWriter from keyboard")
	}
	defer ew.Close()

	// Use a goroutine to verify that an action produces the correct speech.
	// We need to begin waiting for utterances before speech is given to avoid
	// any flakiness.
	// The first channel is used to communicate that the goroutine is ready.
	// The second channel is used to communicate errors, if any.
	ch1 := make(chan bool)
	ch2 := make(chan error)
	go tts.expectSpeech(ctx, ch1, ch2, utterances)
	if ready := <-ch1; ready != true {
		return <-ch2
	}
	if err := ew.Accel(ctx, keys); err != nil {
		return errors.Wrapf(err, "error when pressing the keys: %s", keys)
	}
	if err := <-ch2; err != nil {
		return errors.Wrap(err, "failed to verify speech from the Google TTS engine")
	}

	return nil
}
