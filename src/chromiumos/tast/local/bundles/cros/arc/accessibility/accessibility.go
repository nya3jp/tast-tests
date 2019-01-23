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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
		// This is a build of an application containing a single activity and basic UI elements.
		// The source code is in vendor/google_arc.
		apkName = "accessibility_sample.apk"
		packageName  = "org.chromium.arc.testapp.accessibility_sample"
		activityName = "org.chromium.arc.testapp.accessibility_sample.AccessibilityActivity"

		toggleButtonID    = "org.chromium.arc.testapp.accessibility_sample:id/toggleButton"
		checkBoxID        = "org.chromium.arc.testapp.accessibility_sample:id/checkBox"

		extURL = "chrome-extension://mndnfokpggljbaajbnioimlmbfngpief/cvox2/background/background.html"

)

// Enabled checks if accessibility is enabled in Android.
func Enabled(ctx context.Context, a *arc.ARC) (bool, error) {
	cmd := a.Command(ctx, "settings", "--user", "0", "get", "secure", "accessibility_enabled")
	res, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		return false, err
	}
	return strings.TrimSpace(string(res)) == "1", nil
}

// ChromeVoxExtConn returns a connection to the ChromeVox extension's background page.
// If the extension is not ready, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func ChromeVoxExtConn(ctx context.Context, c *chrome.Chrome) (*chrome.Conn, error) {
	testing.ContextLog(ctx, "Waiting for extension at ", extURL)
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
		return errors.Wrap(err, "Creating test API connection failed")
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
			return errors.Wrap(err, "Failed to enable spoken feedback")
	}

	// Wait until spoken feedback is enabled.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if res, err := Enabled(ctx, a); err != nil {
			return errors.Wrap(err, "Failed to check whether accessibility is enabled in Android")
		} else if !res {
			return errors.New("accessibility not enabled")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "Failed to ensure accessibility is enabled: ")
	}
	return nil
}

// StartChrome starts Chrome with accessibility enabled.
// If there is an error starting Chrome, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func StartChrome(ctx context.Context) (*chrome.Chrome, error) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs([]string{"--force-renderer-accessibility"}))
	if err != nil {
		cr.Close(ctx)
		return nil, errors.Wrap(err, "Failed to connect to Chrome: ")
	}
	return cr, nil
}

// StartARC starts ARC.
// If there is an error starting ARC, the connection will be closed before returning.
// Otherwise the calling function will close the connection.
func StartARC(ctx context.Context, s *testing.State) (*arc.ARC, error) {
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		a.Close()
		return nil, errors.New("Failed to start ARC")
	}
	return a, err
}

// StartAccessibilityApp starts the test application, and checks that UI components exist.
func StartAccessibilityApp(ctx context.Context, a *arc.ARC, s *testing.State) (error) {
	s.Log("Installing app")
	// Install accessibility_sample.apk
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		return errors.Wrap(err, "Failed installing app: ")
	}

	s.Log("Starting app")
	// Run accessibility_sample.apk.
	if err := a.Command(ctx, "am", "start", "-W", packageName+"/"+activityName).Run(); err != nil {
		return errors.Wrap(err, "Failed starting app")
	}
	// Setup UI Automator.
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		return errors.Wrap(err, "Failed initializing UI Automator")
	}
	defer d.Close()

	// Check UI components exist as expected.
	if err := d.Object(ui.ID(toggleButtonID)).WaitForExists(ctx); err != nil {
		return err
	}
	if err := d.Object(ui.ID(checkBoxID)).WaitForExists(ctx); err != nil {
		return err
	}
	return nil
}
