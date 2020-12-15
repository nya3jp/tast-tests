// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	arcui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeIntentPicker,
		Desc: "Installs an ARC app and opens it from Chrome intent picker",
		Contacts: []string{
			"benreich@chromium.org",
			"mxcai@chromium.org",
			"chromeos-apps-foundation-team@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Pre:  arc.Booted(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 10 * time.Minute,
	})
}

const (
	arcChromeIntentPickerUITimeout    = 15 * time.Second
	arcChromeIntentPickerPollInterval = 2 * time.Second
)

func ChromeIntentPicker(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	arcDevice := s.PreValue().(arc.PreData).ARC

	const (
		appName        = "Intent Picker Test App"
		intentActionID = "org.chromium.arc.testapp.chromeintentpicker:id/intent_action"
		expectedAction = "android.intent.action.VIEW"
	)

	// Setup Test API Connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Setup ARC, UI Automator and Installs APK.
	uiAutomator, err := setUpARCForChromeIntentPicker(ctx, arcDevice, s.OutDir())
	if err != nil {
		s.Fatal("Failed setting up ARC: ", err)
	}
	defer uiAutomator.Close(ctx)

	// Navigate to URL which ArcChromeIntentPickerTest app has associated an intent.
	conn, err := cr.NewConn(ctx, "https://www.google.com")
	if err != nil {
		s.Fatal("Failed to create renderer: ", err)
	}
	defer conn.Close()

	// Locate and left click on the Intent Picker button in Chrome omnibox.
	params := chromeui.FindParams{
		ClassName: "IntentPickerView",
		Role:      chromeui.RoleTypeButton,
	}
	intentPicker, err := chromeui.FindWithTimeout(ctx, tconn, params, arcChromeIntentPickerUITimeout)
	if err != nil {
		s.Fatal("Failed to find intent picker button: ", err)
	}
	defer intentPicker.Release(ctx)

	if err := intentPicker.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click intent picker button: ", err)
	}

	if err := waitAndClickAppOnStableIntentView(ctx, tconn, appName); err != nil {
		s.Fatal("Failed clicking on app: ", err)
	}

	// Wait for the android intent to show in the Android test app.
	intentActionField := uiAutomator.Object(arcui.ID(intentActionID), arcui.Text(expectedAction))
	if err := intentActionField.WaitForExists(ctx, arcChromeIntentPickerUITimeout); err != nil {
		s.Fatalf("Failed waiting for intent action %q to appear in ARC window: %v", expectedAction, err)
	}
}

func waitAndClickAppOnStableIntentView(ctx context.Context, tconn *chrome.TestConn, appName string) error {
	pollOpts := testing.PollOptions{Interval: arcChromeIntentPickerPollInterval, Timeout: arcChromeIntentPickerUITimeout}

	// Get the Intent Picker popover.
	params := chromeui.FindParams{
		ClassName: "IntentPickerBubbleView",
		Name:      "Open with",
		Role:      chromeui.RoleTypeWindow,
	}
	intentPickerPopover, err := chromeui.FindWithTimeout(ctx, tconn, params, arcChromeIntentPickerUITimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find intent picker popover")
	}
	defer intentPickerPopover.Release(ctx)

	// Setup a watcher to wait for the apps list in Intent Picker to stabilize.
	ew, err := chromeui.NewWatcher(ctx, intentPickerPopover, chromeui.EventTypeActiveDescendantChanged)
	if err != nil {
		return errors.Wrap(err, "failed getting a watcher for the intent picker popover")
	}
	defer ew.Release(ctx)

	// Check the Intent Picker popover for any Activedescendantchanged events occurring in a 2 second interval.
	// If any events are found continue polling until 10s is reached.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return ew.EnsureNoEvents(ctx, arcChromeIntentPickerPollInterval)
	}, &pollOpts); err != nil {
		return errors.Wrapf(err, "failed waiting %v for intent picker popover to stabilize", pollOpts.Timeout)
	}

	// Find the appName button in the Intent Picker popover and left click it.
	params = chromeui.FindParams{
		Role: chromeui.RoleTypeButton,
		Name: appName,
	}
	appLabel, err := chromeui.FindWithTimeout(ctx, tconn, params, arcChromeIntentPickerUITimeout)
	if err != nil {
		return errors.Wrapf(err, "failed to find app intent picker label %q", appName)
	}
	defer appLabel.Release(ctx)

	if err := appLabel.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to click app %q button", appName)
	}

	// Left click the Open button in the Intent Picker popover.
	params = chromeui.FindParams{
		Role: chromeui.RoleTypeButton,
		Name: "Open",
	}
	openButton, err := chromeui.FindWithTimeout(ctx, tconn, params, arcChromeIntentPickerUITimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find open intent picker button")
	}
	defer openButton.Release(ctx)

	if err := openButton.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click open button on intent picker")
	}

	return nil
}

// setUpARCForChromeIntentPicker starts an ARC device and starts UI automator.
func setUpARCForChromeIntentPicker(ctx context.Context, arcDevice *arc.ARC, outDir string) (*arcui.Device, error) {
	// Start up UI automator.
	uiAutomator, err := arcDevice.NewUIDevice(ctx)
	if err != nil {
		if err := arcDevice.Close(); err != nil {
			testing.ContextLog(ctx, "Failed closing UI automator: ", err)
		}
		return nil, errors.Wrap(err, "failed initializing UI automator")
	}

	if err := arcDevice.WaitIntentHelper(ctx); err != nil {
		return nil, errors.Wrap(err, "failed waiting for intent helper")
	}

	if err := arcDevice.Install(ctx, arc.APKPath("ArcChromeIntentPickerTest.apk")); err != nil {
		return nil, errors.Wrap(err, "failed installing the APK")
	}

	return uiAutomator, nil
}
