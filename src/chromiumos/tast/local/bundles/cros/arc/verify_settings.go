// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	arcui "chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifySettings,
		Desc:         "Verifies ARC++ settings work as intended",
		Contacts:     []string{"vkrishan@google.com", "rohitbm@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 3 * time.Minute,
		Vars:    []string{"arc.VerifySettings.username", "arc.VerifySettings.password"},
	})
}

func VerifySettings(ctx context.Context, s *testing.State) {

	username := s.RequiredVar("arc.VerifySettings.username")
	password := s.RequiredVar("arc.VerifySettings.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(), chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Optin to PlayStore.
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store: ", err)
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for Play Store: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	d, err := arcui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Log("Failed to close Play Store: ", err)
	}

	// Navigate to Android Settings.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch the Settings app: ", err)
	}

	appsParam := chromeui.FindParams{
		Role: chromeui.RoleTypeHeading,
		Name: "Apps",
	}

	apps, err := chromeui.FindWithTimeout(ctx, tconn, appsParam, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find Apps Heading: ", err)
	}
	defer apps.Release(ctx)

	if err := apps.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the Apps Heading: ", err)
	}

	// Find the "Google Play Store" heading and associated button.
	PlayStoreParam := chromeui.FindParams{
		Role: chromeui.RoleTypeButton,
		Name: "Google Play Store",
	}

	playstore, err := chromeui.FindWithTimeout(ctx, tconn, PlayStoreParam, 30*time.Second)
	if err != nil {
		s.Fatal("Failed to find GooglePlayStore Heading: ", err)
	}
	defer playstore.Release(ctx)

	if err := playstore.FocusAndWait(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to call focus() on GooglePlayStore: ", err)
	}

	if err := playstore.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the GooglePlayStore Heading: ", err)
	}

	webview, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{Role: chromeui.RoleTypeWebView, ClassName: "WebView"}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find webview: ", err)
	}
	defer webview.Release(ctx)

	s.Log("Navigating to Android Preferences")
	enter, err := webview.DescendantWithTimeout(ctx, chromeui.FindParams{Name: "Manage Android preferences"}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find Manage Android Preferences: ", err)
	}
	defer enter.Release(ctx)

	if err := enter.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click Manage Android Preferences: ", err)
	}

	if err := checkAndroidSettings(ctx, d); err != nil {
		s.Fatal("Failed checking Android Settings: ", err)
	}
}

func checkAndroidSettings(ctx context.Context, arcDevice *arcui.Device) error {

	// Time to wait for UI elements to appear in Play Store and Chrome.
	const timeoutUI = 30 * time.Second

	// Verify System settings in ARC++.
	system := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)system"), arcui.Enabled(true))
	if err := system.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding System Text View")
	}

	if err := system.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on System")
	}

	aboutDevice := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)about device"), arcui.Enabled(true))
	if err := aboutDevice.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding About Device Text View")
	}

	if err := aboutDevice.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click About Device")
	}

	buildNumber := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)build number"), arcui.Enabled(true))
	if err := buildNumber.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Build Number TextView")
	}

	testing.ContextLog(ctx, "Enable Developer Options")
	for i := 0; i < 7; i++ {
		if err := buildNumber.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Build Number TextView")
		}
	}

	backButton := arcDevice.Object(arcui.ClassName("android.widget.ImageButton"), arcui.Enabled(true))

	if err := backButton.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Back Button")
	}

	if err := backButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Back Button")
	}

	developerOptions := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)developer options"), arcui.Enabled(true))
	if err := developerOptions.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Developer Options")
	}

	testing.ContextLog(ctx, "Toggle Backup Settings")
	// TODO(b/159956557): Confirm that Backup status is changing when the button is clicked.
	backup := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)backup"), arcui.Enabled(true))
	if err := backup.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Backup")
	}

	if err := backup.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Backup")
	}

	const backupID = "com.google.android.gms:id/switchWidget"
	backupStatus, err := arcDevice.Object(arcui.ID(backupID)).GetText(ctx)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Default Backup status: "+backupStatus)
	backupToggleOff := arcDevice.Object(arcui.ClassName("android.widget.Switch"), arcui.TextMatches("(?i)off"), arcui.Enabled(true))
	backupToggleOn := arcDevice.Object(arcui.ClassName("android.widget.Switch"), arcui.TextMatches("(?i)on"), arcui.Enabled(true))

	if backupStatus == "ON" {
		// Turn Backup OFF.
		if err := backupToggleOn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click backup toggle")
		}

		turnOffBackup := arcDevice.Object(arcui.ClassName("android.widget.Button"), arcui.TextMatches("(?i)turn off & delete"), arcui.Enabled(true))
		if err := turnOffBackup.WaitForExists(ctx, timeoutUI); err != nil {
			return errors.Wrap(err, "failed finding Turn Off backup button")
		}

		if err := turnOffBackup.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Turn Off backup button")
		}

		if err := backupToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
			return errors.Wrap(err, "failed to turn off Backup")
		}
	}
	// Turn Backup ON.
	if err := backupToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Backup Off Toggle")
	}

	if err := backupToggleOff.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Backup toggle")
	}

	if err := backupToggleOn.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to turn Backup toggle On")
	}

	for i := 0; i < 2; i++ {
		if err := backButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Back Button")
		}
	}

	testing.ContextLog(ctx, "Toggle Location Settings")
	security := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)security & location"), arcui.Enabled(true))
	if err := security.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Security & location TextView")
	}

	if err := security.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Security & Location")
	}

	location := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)location"), arcui.Enabled(true))
	if err := location.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Location TextView")
	}

	if err := location.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Location")
	}

	const locationID = "com.android.settings:id/switch_widget"
	locationStatus, err := arcDevice.Object(arcui.ID(locationID)).GetText(ctx)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Default location status: "+locationStatus)
	locationToggleOff := arcDevice.Object(arcui.ClassName("android.widget.Switch"), arcui.TextMatches("(?i)off"), arcui.Enabled(true))
	locationToggleOn := arcDevice.Object(arcui.ClassName("android.widget.Switch"), arcui.TextMatches("(?i)on"), arcui.Enabled(true))

	if locationStatus == "ON" {
		// Turn Location OFF.
		if err := locationToggleOn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Location toggle")
		}

		if err := locationToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
			return errors.Wrap(err, "failed to switch Location OFF")
		}
	}
	// Turn Location ON.
	if err := locationToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Location Off toggle")
	}

	if err := locationToggleOff.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Location toggle")
	}

	if err := locationToggleOn.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to switch Location ON")
	}

	return nil
}
