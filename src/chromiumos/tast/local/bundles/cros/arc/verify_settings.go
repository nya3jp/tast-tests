// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifySettings,
		Desc:         "Verifies ARC++ settings work as intended",
		Contacts:     []string{"vkrishan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"arc.username", "arc.password"},
	})
}

func VerifySettings(ctx context.Context, s *testing.State) {

	username := s.RequiredVar("arc.username")
	password := s.RequiredVar("arc.password")

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

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

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

	condition := func(ctx context.Context) (bool, error) {
		return chromeui.Exists(ctx, tconn, appsParam)
	}

	opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 2 * time.Second}
	if err := apps.LeftClickUntil(ctx, condition, &opts); err != nil {
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

func checkAndroidSettings(ctx context.Context, arcDevice *androidui.Device) error {
	const (
		scrollClassName = "android.widget.ScrollView"
	)

	// Time to wait for UI elements to appear in Play Store and Chrome.
	const timeoutUI = 30 * time.Second

	// Scroll until logout is visible.
	scrollLayout := arcDevice.Object(androidui.ClassName(scrollClassName), androidui.Scrollable(true))
	system := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)system"), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, timeoutUI); err == nil {
		scrollLayout.ScrollTo(ctx, system)
	}

	// Verify System settings in ARC++.
	if err := system.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding System Text View")
	}

	if err := system.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on System")
	}

	aboutDevice := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)about device"), androidui.Enabled(true))

	if err := aboutDevice.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding About Device Text View")
	}

	if err := aboutDevice.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click About Device")
	}

	buildNumber := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)build number"), androidui.Enabled(true))
	if err := buildNumber.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Build Number TextView")
	}

	backButton := arcDevice.Object(androidui.ClassName("android.widget.ImageButton"), androidui.Enabled(true))

	if err := backButton.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Back Button")
	}

	if err := backButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Back Button")
	}

	developerOptions := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)developer options"), androidui.Enabled(true))
	if err := developerOptions.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Developer Options")
	}

	testing.ContextLog(ctx, "Toggle Backup Settings")
	// TODO(b/159956557): Confirm that Backup status is changing when the button is clicked.
	backup := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)backup"), androidui.Enabled(true))
	if err := backup.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Backup")
	}

	if err := backup.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Backup")
	}

	const backupID = "com.google.android.gms:id/switchWidget"
	backupStatus, err := arcDevice.Object(androidui.ID(backupID)).GetText(ctx)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Default Backup status: "+backupStatus)
	backupToggleOff := arcDevice.Object(androidui.ClassName("android.widget.Switch"), androidui.TextMatches("(?i)off"), androidui.Enabled(true))
	backupToggleOn := arcDevice.Object(androidui.ClassName("android.widget.Switch"), androidui.TextMatches("(?i)on"), androidui.Enabled(true))

	if backupStatus == "ON" {
		// Turn Backup OFF.
		if err := backupToggleOn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click backup toggle")
		}

		turnOffBackup := arcDevice.Object(androidui.ClassName("android.widget.Button"), androidui.TextMatches("(?i)turn off & delete"), androidui.Enabled(true))
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
	security := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)security & location"), androidui.Enabled(true))
	testing.ContextLog(ctx, "Toggle Location Settings")
	if err := security.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Security & location TextView")
	}

	if err := security.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Security & Location")
	}

	location := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)location"), androidui.Enabled(true))
	if err := location.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Location TextView")
	}

	if err := location.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Location")
	}

	const locationID = "com.android.settings:id/switch_widget"
	locationStatus, err := arcDevice.Object(androidui.ID(locationID)).GetText(ctx)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Default location status: "+locationStatus)
	locationToggleOff := arcDevice.Object(androidui.ClassName("android.widget.Switch"), androidui.TextMatches("(?i)off"), androidui.Enabled(true))
	locationToggleOn := arcDevice.Object(androidui.ClassName("android.widget.Switch"), androidui.TextMatches("(?i)on"), androidui.Enabled(true))

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
