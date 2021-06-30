// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package a11y provides functions to assist with interacting with accessibility
// features and settings.
package a11y

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// List of extension IDs and URLs.
const (
	chromeVoxExtensionURL = "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/chromevox/background/background.html"
	ESpeakExtensionID     = "dakbfdmgjiabojdgbiljlhgjbokobjpg"
	GoogleTTSExtensionID  = "gjjabgpgjpampikjhjpfhneeoapjbjaf"
)

// Feature represents an accessibility feature in Chrome OS.
type Feature string

// List of accessibility features.
const (
	Autoclick       Feature = "autoclick"
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

// SetTTSRate sets the speaking rate of the tts, relative to the default rate (1.0).
func SetTTSRate(ctx context.Context, tconn *chrome.TestConn, rate float64) error {
	return tconn.Call(ctx, nil, "tast.promisify(chrome.settingsPrivate.setPref)", "settings.tts.speech_rate", rate)
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

		// Make sure ChromeVoxState is exported globally.
		if err := extConn.WaitForExpr(ctx, "ChromeVoxState.instance"); err != nil {
			return errors.Wrap(err, "ChromeVoxState is unavailable")
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
func (cv *ChromeVoxConn) focusedNode(ctx context.Context, tconn *chrome.TestConn) (*uiauto.NodeInfo, error) {
	var info uiauto.NodeInfo
	if err := cv.Eval(ctx, `(() => {
		const node = ChromeVoxState.instance.currentRange.start.node;
		// Return an object that matches the shape of NodeInfo.
		return {
				checked: node.checked,
				className: node.className,
				htmlAttributes: node.htmlAttributes,
				location: node.location,
				name: node.name,
				restriction: node.restriction,
				role: node.role,
				state: node.state,
				value: node.value,
			};
	})()`, &info); err != nil {
		return nil, errors.Wrap(err, "failed to retrieve the currently focused ChromeVox node")
	}
	return &info, nil
}

// WaitForFocusedNode polls until the properties of the focused node matches the finder.
// timeout specifies the timeout to use when polling.
func (cv *ChromeVoxConn) WaitForFocusedNode(ctx context.Context, tconn *chrome.TestConn, finder *nodewith.Finder, timeout time.Duration) error {
	ui := uiauto.New(tconn)

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		focusedNode, err := cv.focusedNode(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}

		if err := ui.Matches(ctx, focusedNode, finder); err != nil {
			return errors.Errorf("focused node is incorrect: got %v, want %s", focusedNode, finder.Pretty())
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "failed to get current focus")
	}
	return nil
}

// VoiceData stores information necessary to identify TTS voices.
type VoiceData struct {
	ExtID  string `json:"extensionId"`
	Locale string `json:"lang"`
	Name   string `json:"voiceName"`
}

// GetVoices returns the current TTS voices which are available.
func GetVoices(ctx context.Context, conn *chrome.TestConn) ([]VoiceData, error) {
	var voices []VoiceData
	if err := conn.Eval(ctx, "tast.promisify(chrome.tts.getVoices)()", &voices); err != nil {
		return nil, err
	}
	return voices, nil
}

// SetVoice sets the ChromeVox's voice, which is specified by using an extension
// ID and a locale.
func (cv *ChromeVoxConn) SetVoice(ctx context.Context, vd VoiceData) error {
	voices, err := GetVoices(ctx, &chrome.TestConn{Conn: cv.Conn})
	if err != nil {
		return errors.Wrap(err, "failed to getVoices")
	}
	for _, voice := range voices {
		if voice.ExtID == vd.ExtID && voice.Locale == vd.Locale {
			expr := fmt.Sprintf(`chrome.storage.local.set({'voiceName': '%s'});`, voice.Name)
			if err := cv.Eval(ctx, expr, nil); err != nil {
				return err
			}

			// Wait for ChromeVox's current voice to update.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				var actualVoicename string
				if err := cv.Eval(ctx, "getCurrentVoice()", &actualVoicename); err != nil {
					return err
				}

				if actualVoicename != voice.Name {
					return errors.New("ChromeVox's voice has not yet been updated yet")
				}

				return nil
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				return errors.Wrapf(err, "failed to wait for ChromeVox to update its current voice to: %s", voice.Name)
			}

			return nil
		}
	}

	return errors.Errorf("could not find voice with extension ID: %s and locale: %s", vd.ExtID, vd.Locale)
}

// TTSEngineData represents data for a TTS background page. ExtensionID specifies
// the ID of the background page. If |UseOnSpeakWithAudioStream| is true, we
// listen to onSpeakWithAudioStream when accumulating utterances. Otherwise, we
// listen to onSpeak.
type TTSEngineData struct {
	ExtID                     string
	UseOnSpeakWithAudioStream bool
}

// SpeechMonitor represents a connection to a TTS extension background
// page and is used to verify spoken utterances.
type SpeechMonitor struct {
	conn *chrome.Conn
}

// RelevantSpeechMonitor searches through all possible connections to TTS
// background pages and returns a SpeechMonitor for the one that matches engineData.
// If no TTS background pages can be found, then all
// test connections will be closed before returning. Otherwise the calling
// function will close the connection.
func RelevantSpeechMonitor(ctx context.Context, c *chrome.Chrome, tconn *chrome.TestConn, engineData TTSEngineData) (*SpeechMonitor, error) {
	if err := ensureTTSEngineLoaded(ctx, tconn, engineData); err != nil {
		return nil, errors.Wrap(err, "failed to wait for the TTS engine to load")
	}

	extID := engineData.ExtID
	bgURL := chrome.ExtensionBackgroundPageURL(extID)
	targets, err := c.FindTargets(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return nil, err
	}

	// Find and connect to the correct TTS extension background page. For each
	// potential background page:
	// 1. Connect to the background page and wait for it to load.
	// 2. Create a SpeechMonitor for the background page.
	// 3. Send a test utterance using chrome.tts.speak().
	// 4. Try to use the SpeechMonitor to consume the utterance. If it doesn't
	// consume, then we can assume that the current target is not the correct one.
	for _, t := range targets {
		var extConn *chrome.Conn
		// A helper function that will return an error if there was an error with
		// creating a SpeechMonitor. If this function returns an error, then we will
		// close extConn and continue to the next target.
		candidateMonitor, err := func() (*SpeechMonitor, error) {
			// Use a poll to connect to the target's background page.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				var err error
				extConn, err = c.NewConnForTarget(ctx, chrome.MatchTargetID(t.TargetID))
				return err
			}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
				return nil, errors.Wrap(err, "failed to create a connection to candidate TTS background page")
			}

			if err := extConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
				return nil, errors.Wrap(err, "timed out waiting for the TTS engine background page to load")
			}

			// Create SpeechMonitor.
			candidateMonitor, err := newSpeechMonitor(ctx, extConn, engineData)
			if err != nil {
				return nil, errors.Wrap(err, "could not create a speech monitor")
			}

			return candidateMonitor, nil
		}()

		if err != nil {
			testing.ContextLog(ctx, "skipping background page with ID ", t.TargetID, " for reason: ", err)
			if extConn != nil {
				extConn.Close()
			}
			continue
		}

		// A helper function that will return an error if there was a problem with
		// candidateMonitor. If this function returns an error, then we will close
		// candidateMonitor and continue to the next target.
		if err := func() error {
			// Send a test utterance from tts.
			expr := fmt.Sprintf("chrome.tts.speak('Testing', {extensionId: '%s'});", engineData.ExtID)
			if err := tconn.Eval(ctx, expr, nil); err != nil {
				return errors.Wrap(err, "failed to send a test utterance")
			}

			// Attempt to consume the test utterance.
			if err := candidateMonitor.Consume(ctx, []SpeechExpectation{StringExpectation{"Testing"}}); err != nil {
				return errors.Wrap(err, "failed to consume the test utterance")
			}

			return nil
		}(); err != nil {
			testing.ContextLog(ctx, "skipping background page with ID ", t.TargetID, " for reason: ", err)
			candidateMonitor.Close()
			continue
		}

		// If we get here, then we found the target we are looking for. Return the
		// SpeechMonitor for the target.
		return candidateMonitor, nil
	}

	return nil, errors.New("failed to connect to a TTS background page and create a speech monitor")
}

// newSpeechMonitor connects to a TTS extension, specified by conn and engineData, and
// starts accumulating utterances. Call Consume to compare expected and actual
// utterances.
func newSpeechMonitor(ctx context.Context, conn *chrome.Conn, engineData TTSEngineData) (*SpeechMonitor, error) {
	if err := startAccumulatingUtterances(ctx, conn, engineData); err != nil {
		return nil, errors.Wrap(err, "failed to inject JavaScript to accumulate utterances")
	}
	return &SpeechMonitor{conn}, nil
}

// Close closes the connection to the TTS extension's background page.
func (sm *SpeechMonitor) Close() {
	sm.conn.Close()
}

// ensureTTSEngineLoaded is a helper function for RelevantSpeechMonitor. It
// ensures that the desired TTS engine is awake and loaded
// before trying to connect to it. Otherwise, we will get errors when trying to
// create a new SpeechMonitor for an engine that hasn't loaded.
func ensureTTSEngineLoaded(ctx context.Context, tconn *chrome.TestConn, engineData TTSEngineData) error {
	// Call chrome.tts.getVoices() and check for a loaded voice with the specified
	// engine ID.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var voices []VoiceData
		if err := tconn.Eval(ctx, "tast.promisify(chrome.tts.getVoices)()", &voices); err != nil {
			return err
		}

		for _, voice := range voices {
			if voice.ExtID == engineData.ExtID {
				return nil
			}
		}

		return errors.New("TTS engine hasn't loaded yet")
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for the TTS engine to load")
	}

	return nil
}

// SpeechExpectation defines an interface for a speech expectation.
type SpeechExpectation interface {
	matches(utterance string) error
}

// RegexExpectation represents data for a speech expectation, where |expectation|
// is a regular expression.
type RegexExpectation struct {
	expectation string
}

// StringExpectation represents data for a speech expectation, where |expectation|
// is a string.
type StringExpectation struct {
	expectation string
}

func (re RegexExpectation) matches(utterance string) error {
	matched, err := regexp.MatchString(re.expectation, utterance)
	if !matched || err != nil {
		return errors.Errorf("expected pattern: %s does not match utterance: %s", re.expectation, utterance)
	}

	return nil
}

func (se StringExpectation) matches(utterance string) error {
	if se.expectation != utterance {
		return errors.Errorf("expected utterance: %s does not match utterance: %s", se.expectation, utterance)
	}

	return nil
}

// NewRegexExpectation is a convenience method for creating a RegexExpectation
// object.
func NewRegexExpectation(expectation string) RegexExpectation {
	return RegexExpectation{expectation}
}

// NewStringExpectation is a convenience method for creating a StringExpectation
// object.
func NewStringExpectation(expectation string) StringExpectation {
	return StringExpectation{expectation}
}

// Consume ensures that the expectations were spoken by the TTS engine. It also
// consumes all utterances accumulated in TTS extension's background page.
// For each expectation we:
// 1. Shift the next spoken utterance off of testUtterances. This ensures that
// we never compare against stale utterances; a spoken utterance is either
// matched or discarded.
// 2. Check if the utterance matches the expectation.
func (sm *SpeechMonitor) Consume(ctx context.Context, expectations []SpeechExpectation) error {
	var actual []string
	for _, exp := range expectations {
		// Use a poll to allow time for each utterance to be spoken.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var utterance string
			if err := sm.conn.Eval(ctx, "testUtterances.shift()", &utterance); err != nil {
				return errors.Wrap(err, "couldn't assign utterance to value of testUtterances.shift() (testUtterances is likely empty)")
			}

			actual = append(actual, utterance)
			if err := exp.matches(utterance); err != nil {
				return errors.Wrap(err, "expected utterance/pattern hasn't been matched yet")
			}

			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Errorf("expected: %q, but got: %q", expectations, actual)
		}
	}

	return nil
}

// PressKeysAndConsumeExpectations presses keys and ensures that the expectations
// were spoken by the TTS engine.
func PressKeysAndConsumeExpectations(ctx context.Context, sm *SpeechMonitor, keySequence []string, expectations []SpeechExpectation) error {
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

	if err := sm.Consume(ctx, expectations); err != nil {
		return errors.Wrap(err, "error when consuming expectations")
	}

	return nil
}

// startAccumulatingUtterances injects JavaScript into conn's background
// page to accumulate spoken utterances. This function only supports the Google
// TTS and eSpeak TTS engines. If engineData.UseOnSpeakWithAudioStream is true,
// then we will listen to onSpeakWithAudioStream when accumulating utterances.
// Otherwise, we will listen to onSpeak.
func startAccumulatingUtterances(ctx context.Context, conn *chrome.Conn, engineData TTSEngineData) error {
	extID := engineData.ExtID
	if extID != GoogleTTSExtensionID && extID != ESpeakExtensionID {
		return errors.Errorf("could not inject JavaScript into the background page of extension with id, %s, since it doesn't match the Google TTS or eSpeak extension ID", extID)
	}
	if engineData.UseOnSpeakWithAudioStream {
		return conn.Eval(ctx, `
			if (!window.testUtterances) {
		    window.testUtterances = [];
		    chrome.ttsEngine.onSpeakWithAudioStream.addListener(utterance => testUtterances.push(utterance));
		  }
	`, nil)
	}

	return conn.Eval(ctx, `
	if (!window.testUtterances) {
    window.testUtterances = [];
    chrome.ttsEngine.onSpeak.addListener(utterance => testUtterances.push(utterance));
  }
`, nil)
}

// NewTabWithHTML creates a new tab with the specified HTML, waits for it to
// load, and returns a connection to the page.
func NewTabWithHTML(ctx context.Context, cr *chrome.Chrome, html string) (*chrome.Conn, error) {
	url := fmt.Sprintf("data:text/html, %s", html)
	c, err := cr.NewConn(ctx, url, cdputil.WithNewWindow())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open new tab with url: %s", url)
	}

	if err := c.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		c.Close()
		return nil, errors.Wrap(err, "timed out waiting for page to load")
	}

	return c, nil
}
