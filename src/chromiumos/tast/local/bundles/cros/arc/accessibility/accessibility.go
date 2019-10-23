// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accessibility provides functions to assist with interacting with accessibility settings
// in ARC accessibility tests.
package accessibility

import (
	"context"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// This is a build of an application containing a single activity and basic UI elements.
	// The source code is in vendor/google_arc.
	packageName  = "org.chromium.arc.testapp.accessibilitytest"
	activityName = "org.chromium.arc.testapp.accessibilitytest.AccessibilityActivity"

	toggleButtonID    = "org.chromium.arc.testapp.accessibilitytest:id/toggleButton"
	checkBoxID        = "org.chromium.arc.testapp.accessibilitytest:id/checkBox"
	seekBarID         = "org.chromium.arc.testapp.accessibilitytest:id/seekBar"
	seekBarDiscreteID = "org.chromium.arc.testapp.accessibilitytest:id/seekBarDiscrete"
	webViewID         = "org.chromium.arc.testapp.accessibilitytest:id/webView"

	extURL = "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/cvox2/background/background.html"
)

// AutomationNode represents an accessibility struct, which contains properties from chrome.automation.Autotmation.
// This is defined at:
// https://developer.chrome.com/extensions/automation#type-AutomationNode
// Only the properties which are used in tast tests are defined here.
type AutomationNode struct {
	ClassName     string
	Tooltip       string
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
						Tooltip: node.tooltip,
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

// ChromeVoxExtConn returns a connection to the ChromeVox extension's background page.
// If the extension is not ready, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func ChromeVoxExtConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
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

// EnableSpokenFeedback will enable spoken feedback on Chrome.
// Also checks that setting is reflected in Android as well.
func EnableSpokenFeedback(ctx context.Context, cr *chrome.Chrome, a *arc.ARC) error {
	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "creating test API connection failed")
	}

	if err := conn.EvalPromise(ctx, `
		new Promise((resolve, reject) => {
			chrome.accessibilityFeatures.spokenFeedback.set({value: true});
			chrome.accessibilityFeatures.spokenFeedback.get({}, (details) => {
				if (details.value) {
					resolve();
				} else {
					reject();
				}
			});
		})`, nil); err != nil {
		return errors.Wrap(err, "failed to enable spoken feedback")
	}

	// Wait until spoken feedback is enabled in Android side.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := Enabled(ctx, a); err != nil {
			return errors.Wrap(err, "failed to check whether accessibility is enabled in Android")
		} else if !res {
			return errors.New("accessibility not enabled")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to ensure accessibility is enabled: ")
	}
	return nil
}

// NewChrome starts Chrome calling chrome.New() with accessibility enabled.
// The calling function will close the connection.
func NewChrome(ctx context.Context) (*chrome.Chrome, error) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-renderer-accessibility"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome: ")
	}
	return cr, nil
}

// NewARC starts ARC by calling arc.New().
// The calling function will close the connection.
func NewARC(ctx context.Context, outDir string) (*arc.ARC, error) {
	a, err := arc.New(ctx, outDir)
	if err != nil {
		return nil, errors.New("failed to start ARC")
	}
	return a, err
}

// InstallAndStartSampleApp starts the test application, and checks that UI components exist.
func InstallAndStartSampleApp(ctx context.Context, a *arc.ARC, apkPath string) error {
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
			return errors.Errorf("focused node is %q, want %q", focusedNode, node)
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
		if err := chromeVoxConn.Eval(ctx, "cvox.ChromeVox.tts.isSpeaking()", &isSpeaking); err != nil {
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

// WaitForChromeVoxReady polls until ChromeVox connection finishes loading.
func WaitForChromeVoxReady(ctx context.Context, chromeVoxConn *chrome.Conn) error {
	if err := chromeVoxConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return errors.Wrap(err, "timed out waiting for ChromeVox connection to be ready")
	}

	// Wait for ChromeVox to stop speaking before interacting with it further.
	if err := WaitForChromeVoxStopSpeaking(ctx, chromeVoxConn); err != nil {
		return err
	}

	testing.ContextLog(ctx, "ChromeVox is ready")
	return nil
}
