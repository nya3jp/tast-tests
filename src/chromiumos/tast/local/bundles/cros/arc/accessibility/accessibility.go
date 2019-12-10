// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accessibility provides functions to assist with interacting with accessibility settings
// in ARC accessibility tests.
package accessibility

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// ApkName the apk which will be used in accessibility tests.
	// It is a build of an application containing a single activity and basic UI elements.
	// The source code is in vendor/google_arc.
	ApkName      = "ArcAccessibilityTest.apk"
	packageName  = "org.chromium.arc.testapp.accessibilitytest"
	activityName = "org.chromium.arc.testapp.accessibilitytest.AccessibilityActivity"

	toggleButtonID    = "org.chromium.arc.testapp.accessibilitytest:id/toggleButton"
	checkBoxID        = "org.chromium.arc.testapp.accessibilitytest:id/checkBox"
	seekBarID         = "org.chromium.arc.testapp.accessibilitytest:id/seekBar"
	seekBarDiscreteID = "org.chromium.arc.testapp.accessibilitytest:id/seekBarDiscrete"
	webViewID         = "org.chromium.arc.testapp.accessibilitytest:id/webView"

	extURL = "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/background/background.html"

	// CheckBox class for UI widget.
	CheckBox = "android.widget.CheckBox"
	// EditText class for UI widget.
	EditText = "android.widget.EditText"
	// SeekBar class for UI widget.
	SeekBar = "android.widget.SeekBar"
	// ToggleButton class for UI widget.
	ToggleButton = "android.widget.ToggleButton"
)

// AutomationNode represents an accessibility struct, which contains properties from chrome.automation.Automation.
// This is defined at:
// https://developer.chrome.com/extensions/automation#type-AutomationNode
// Only the properties which are used in tast tests are defined here.
type AutomationNode struct {
	ClassName     string
	Checked       string
	ValueForRange int
}

// FocusedNode returns the currently focused node of ChromeVox.
// chrome.automation.AutomationNode properties are defined using getters
// see: https://cs.chromium.org/chromium/src/extensions/renderer/resources/automation/automation_node.js?q=automationNode&sq=package:chromium&dr=CSs&l=1218
// meaning that resolve(node) cannot be applied here.
func FocusedNode(ctx context.Context, chromeVoxConn *chrome.Conn) (*AutomationNode, error) {
	var automationNode AutomationNode
	const script = `new Promise((resolve, reject) => {
				chrome.automation.getFocus((node) => {
					resolve({
						Checked: node.checked,
						ClassName: node.className,
						ValueForRange: node.valueForRange
					});
				});
			})`
	if err := chromeVoxConn.EvalPromise(ctx, script, &automationNode); err != nil {
		return nil, err
	}
	return &automationNode, nil
}

// Enabled checks if accessibility is enabled in Android.
func Enabled(ctx context.Context, a *arc.ARC) (bool, error) {
	res, err := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled").Output(testexec.DumpLogOnError)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(res)) == "1", nil
}

// chromeVoxExtConn returns a connection to the ChromeVox extension's background page.
// If the extension is not ready, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func chromeVoxExtConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	testing.ContextLog(ctx, "Waiting for ChromeVox background page at ", extURL)
	extConn, err := c.NewConnForTarget(ctx, chrome.MatchTargetURL(extURL))
	if err != nil {
		return nil, err
	}

	// Ensure that we don't attempt to use the extension before its APIs are
	// available: https://crbug.com/789313.
	if err := extConn.WaitForExpr(ctx, "ChromeVoxState.instance"); err != nil {
		extConn.Close()
		return nil, errors.Wrap(err, "ChromeVox unavailable")
	}

	testing.ContextLog(ctx, "Extension is ready")
	return extConn, nil
}

func toggleSpokenFeedback(ctx context.Context, cr *chrome.Chrome, enable bool) error {
	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "creating test API connection failed")
	}

	script := fmt.Sprintf(`
			chrome.accessibilityFeatures.spokenFeedback.set({value: %t})`, enable)
	if err := conn.Exec(ctx, script); err != nil {
		return errors.Wrap(err, "failed to toggle spoken feedback")
	}
	return nil

}

// enableSpokenFeedback tests enabling spoken feedback on Chrome.
// Spoken Feedback will also be disabled to ensure it does not affect other tests.
// Also checks that setting is reflected in Android as well.
// A connection to the ChromeVox extension background page is returned, and this will be
// close by the calling function.
func enableSpokenFeedback(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) (*chrome.Conn, error) {
	// Wait until spoken feedback is enabled in Android side.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := Enabled(ctx, a); err != nil {
			return errors.Wrap(err, "failed to check whether accessibility is enabled in Android")
		} else if !res {
			return errors.New("accessibility not enabled")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to ensure accessibility is enabled: ")
	}

	chromeVoxConn, err := chromeVoxExtConn(ctx, cr)
	if err != nil {
		return nil, errors.Wrap(err, "creating connection to ChromeVox extension failed: ")
	}

	// Poll until ChromeVox connection finishes loading.
	if err := chromeVoxConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "timed out waiting for ChromeVox connection to be ready")
	}
	// Wait for ChromeVox to stop speaking before interacting with it further.
	if err := WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "ChromeVox is ready")
	return chromeVoxConn, nil
}

// installAndStartSampleApp starts the test application, and checks that UI components exist.
func installAndStartSampleApp(ctx context.Context, a *arc.ARC, apkPath string) error {
	testing.ContextLog(ctx, "Installing app")
	// Install ArcAccessibilityTest.apk
	if err := a.Install(ctx, apkPath); err != nil {
		return errors.Wrap(err, "failed installing app: ")
	}

	testing.ContextLog(ctx, "Starting app")
	// Run ArcAccessibilityTest.apk.
	if err := a.Command(ctx, "am", "start", "-W", packageName+"/"+activityName).Run(); err != nil {
		return errors.Wrap(err, "failed starting app")
	}

	// Setup UI Automator.
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		return errors.Wrap(err, "failed initializing UI Automator")
	}
	defer d.Close()

	// Check UI components exist as expected.
	const timeout = 30 * time.Second
	for _, id := range []string{toggleButtonID, checkBoxID, seekBarID, seekBarDiscreteID, webViewID} {
		if err := d.Object(ui.ID(id)).WaitForExists(ctx, timeout); err != nil {
			return err
		}
	}
	return nil
}

// WaitForFocusedNode polls until the properties of the focused node matches node.
// Returns an error after 30 seconds.
func WaitForFocusedNode(ctx context.Context, chromeVoxConn *chrome.Conn, node *AutomationNode) error {
	// Wait for focusClassName to receive focus.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		focusedNode, err := FocusedNode(ctx, chromeVoxConn)
		if err != nil {
			return err
		} else if !cmp.Equal(focusedNode, node, cmpopts.EquateEmpty()) {
			return errors.Errorf("focused node is incorrect: got %q, want %q", focusedNode, node)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to get current focus")
	}
	return nil
}

// WaitForChromeVoxStopSpeaking polls until ChromeVox TTS has stoped speaking.
func WaitForChromeVoxStopSpeaking(ctx context.Context, chromeVoxConn *chrome.Conn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var isSpeaking bool
		if err := chromeVoxConn.Eval(ctx, "ChromeVox.tts.isSpeaking()", &isSpeaking); err != nil {
			return err
		}
		if isSpeaking {
			return errors.New("ChromeVox is speaking")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "timed out waiting for ChromeVox to finish speaking")
	}
	return nil
}

// RunTest installs the ArcAccessibilityTestApplication, launches it, and waits
// for ChromeVox to be ready.
func RunTest(ctx context.Context, s *testing.State, f func(a *arc.ARC, conn *chrome.Conn, ew *input.KeyboardEventWriter)) {
	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer audio.Unmute(ctx)

	d := s.PreValue().(arc.PreData)
	a := d.ARC
	cr := d.Chrome

	if err := installAndStartSampleApp(ctx, a, s.DataPath(ApkName)); err != nil {
		s.Fatal("Setting up ARC environment with accessibility failed: ", err)
	}

	if err := toggleSpokenFeedback(ctx, cr, true); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}

	chromeVoxConn, err := enableSpokenFeedback(ctx, cr, a)
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer chromeVoxConn.Close()
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	defer func() {
		if err := toggleSpokenFeedback(ctx, cr, false); err != nil {
			s.Fatal("Failed to disable spoken feedback: ", err)
		}
	}()
	f(a, chromeVoxConn, ew)
}
