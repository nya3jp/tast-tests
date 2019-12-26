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
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// ApkName is a name the apk which is used in ARC++ accessibility tests.
	ApkName      = "ArcAccessibilityTest.apk"
	packageName  = "org.chromium.arc.testapp.accessibilitytest"
	activityName = ".AccessibilityActivity"

	extURL = "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/background/background.html"

	// CheckBox class name.
	CheckBox = "android.widget.CheckBox"
	// EditText class name.
	EditText = "android.widget.EditText"
	// SeekBar class name.
	SeekBar = "android.widget.SeekBar"
	// TextView class name.
	TextView = "android.widget.TextView"
	// ToggleButton class name.
	ToggleButton = "android.widget.ToggleButton"
)

// AutomationNode represents an accessibility struct, which contains properties from chrome.automation.Automation.
// This is defined at:
// https://developer.chrome.com/extensions/automation#type-AutomationNode
// Only the properties which are used in tast tests are defined here.
type AutomationNode struct {
	ClassName     string
	Checked       string
	Tooltip       string
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
						Tooltip: node.tooltip,
						ValueForRange: node.valueForRange
					});
				});
			})`
	if err := chromeVoxConn.EvalPromise(ctx, script, &automationNode); err != nil {
		return nil, err
	}
	return &automationNode, nil
}

// IsEnabledAndroid checks if accessibility is enabled in Android.
func IsEnabledAndroid(ctx context.Context, a *arc.ARC) (bool, error) {
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

// ToggleSpokenFeedback toggles spoken feedback using the provided connection to the extension.
func ToggleSpokenFeedback(ctx context.Context, conn *chrome.Conn, enable bool) error {
	script := fmt.Sprintf(`chrome.accessibilityFeatures.spokenFeedback.set({value: %t})`, enable)
	if err := conn.Exec(ctx, script); err != nil {
		return errors.Wrap(err, "failed to toggle spoken feedback")
	}
	return nil

}

// waitForSpokenFeedbackReady enables spoken feedback.
// A connection to the ChromeVox extension background page is returned, and this will be
// closed by the calling function.
func waitForSpokenFeedbackReady(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) (*chrome.Conn, error) {
	// Wait until spoken feedback is enabled in Android side.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := IsEnabledAndroid(ctx, a); err != nil {
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

	testing.ContextLog(ctx, "ChromeVox is ready")
	return chromeVoxConn, nil
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

// RunTest starts ChromeVox, installs the ArcAccessibilityTestApplication, launches it,
// waits for all of them to be ready, and run the given function.
func RunTest(ctx context.Context, s *testing.State, f func(*arc.ARC, *chrome.Conn, *input.KeyboardEventWriter)) {
	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute device: ", err)
	}
	defer audio.Unmute(ctx)

	d := s.PreValue().(arc.PreData)
	a := d.ARC
	cr := d.Chrome

	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	if err := ToggleSpokenFeedback(ctx, conn, true); err != nil {
		s.Fatal("Failed to enable spoken feedback: ", err)
	}
	defer func() {
		if err := ToggleSpokenFeedback(ctx, conn, false); err != nil {
			s.Fatal("Failed to disable spoken feedback: ", err)
		}
	}()

	chromeVoxConn, err := waitForSpokenFeedbackReady(ctx, cr, a)
	if err != nil {
		s.Fatal(err) // NOLINT: arc/ui returns loggable errors
	}
	defer chromeVoxConn.Close()

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error with creating EventWriter from keyboard: ", err)
	}
	defer ew.Close()

	if err := a.Install(ctx, s.DataPath(ApkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, packageName, activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	if err := act.WaitForResumed(ctx, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for activity to resume: ", err)
	}

	if err := WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		s.Fatal("Failed to wait for finishing ChromeVox speaking: ", err)
	}

	f(a, chromeVoxConn, ew)
}
