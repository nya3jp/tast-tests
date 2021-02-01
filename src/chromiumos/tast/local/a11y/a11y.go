// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility
// features and settings.
package a11y

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	chromeVoxExtensionURL = "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/chromevox/background/background.html"
	googleTTSExtensionID  = "gjjabgpgjpampikjhjpfhneeoapjbjaf"
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

// SpeechMonitor represents a connection to the Google TTS extension background
// page and is used to verify spoken utterances.
type SpeechMonitor struct {
	conn       *chrome.Conn
	startIndex int
}

// NewSpeechMonitor connects to the Google TTS extension and starts accumulating
// utterances. Call Consume to compare expected and actual utterances.
// If the extension is not ready, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func NewSpeechMonitor(ctx context.Context, c *chrome.Chrome) (sm *SpeechMonitor, retErr error) {
	bgURL := chrome.ExtensionBackgroundPageURL(googleTTSExtensionID)
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
		var err error
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

	if err := extConn.Eval(ctx, `
		if (!window.testUtterances) {
	    window.testUtterances = [];
	    chrome.ttsEngine.onSpeak.addListener(utterance => testUtterances.push(utterance));
	  }
`, nil); err != nil {
		return nil, errors.Wrap(err, "failed to inject code to accumulate utterances")
	}

	return &SpeechMonitor{extConn, 0}, nil
}

// Close closes the connection to the Google TTS extension's background page.
func (sm *SpeechMonitor) Close() {
	sm.conn.Close()
}

// consume ensures that the expected utterances were spoken by the
// TTS engine. It also consumes all utterances accumulated in TTS extension's
// background page.
// For each utterance we:
// 1. Retrieve a list of utterances spoken by the TTS engine. This is our list
// of actual utterances.
// 2. Check for a match in the list of actual utterances.
// 3. After an utterance is matched, we filter out all utterances that came
// before it by updating the startIndex property of SpeechMonitor. This ensures
// that we never compare against stale utterances.
func (sm *SpeechMonitor) consume(ctx context.Context, expected []string) error {
	for _, exp := range expected {
		// Use a poll to allow time for each utterance to be spoken.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// 1. Retrieve the list of utterances.
			var actual []string
			if err := sm.conn.Eval(ctx, "testUtterances", &actual); err != nil {
				return errors.Wrap(err, "failed to retrieve actual utterances")
			}

			// 2. Check for a match. Begin iteration at startIndex to filter out stale
			// utterances.
			found := -1
			for i := sm.startIndex; i < len(actual); i++ {
				if exp == actual[i] {
					found = i
					break
				}
			}

			if found == -1 {
				return errors.Errorf("Utterance hasn't been spoken yet: %s", exp)
			}

			// 3. Update startIndex to filter out all previous utterances.
			sm.startIndex = found + 1
			return nil

		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Errorf("Utterance expected but not spoken: %s", exp)
		}
	}

	return nil
}

// PressKeysAndConsumeUtterances presses keys and ensures that the utterances
// were spoken by the TTS engine.
func PressKeysAndConsumeUtterances(ctx context.Context, sm *SpeechMonitor, keySequence, utterances []string) error {
	// Open a connection to the keyboard.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "error with creating EventWriter from keyboard")
	}
	defer ew.Close()

	for _, keys := range keySequence {
		if err := ew.Accel(ctx, keys); err != nil {
			return errors.Wrapf(err, "error when pressing the keys: %s", keys)
		}
	}

	if err := sm.consume(ctx, utterances); err != nil {
		return errors.Wrap(err, "error when consuming utterances")
	}

	return nil
}
