// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	adbui "chromiumos/tast/local/adb/ui"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifySettings,
		Desc:         "Verifies ARC++ settings work as intended",
		Contacts:     []string{"vkrishan@google.com", "rohitbm@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 3 * time.Minute,
	})
}

func VerifySettings(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	a := p.ARC
	cr := p.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

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

	if err := enter.FocusAndWait(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to call focus() on Manage Android Preferences: ", err)
	}

	if err := enter.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click Manage Android Preferences: ", err)
	}

	if err := checkAndroidSettings(ctx, d); err != nil {
		s.Fatal("Failed checking Android Settings: ", err)
	}
}

func checkAndroidSettings(ctx context.Context, arcDevice *adbui.Device) error {

	// Time to wait for UI elements to appear in Play Store and Chrome.
	const timeoutUI = 30 * time.Second

	// Verify System settings in ARC++.
	system := arcDevice.Object(adbui.ClassName("android.widget.TextView"), adbui.TextMatches("(?i)system"), adbui.Enabled(true))
	if err := system.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding System Text View")
	}

	if err := system.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on System")
	}

	testing.ContextLog(ctx, "Navigate to About Device")
	aboutDevice := arcDevice.Object(adbui.ClassName("android.widget.TextView"), adbui.TextMatches("(?i)about device"), adbui.Enabled(true))
	if err := aboutDevice.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding About Device Text View")
	}

	if err := aboutDevice.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click About Device")
	}

	buildNumber := arcDevice.Object(adbui.ClassName("android.widget.TextView"), adbui.TextMatches("(?i)build number"), adbui.Enabled(true))
	if err := buildNumber.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Build Number TextView")
	}

	testing.ContextLog(ctx, "Enable Developer Options")
	for i := 0; i < 7; i++ {
		if err := buildNumber.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Build Number TextView")
		}
	}

	backButton := arcDevice.Object(adbui.ClassName("android.widget.ImageButton"), adbui.Enabled(true))

	if err := backButton.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Back Button")
	}

	if err := backButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Back Button")
	}

	developerOptions := arcDevice.Object(adbui.ClassName("android.widget.TextView"), adbui.TextMatches("(?i)developer options"), adbui.Enabled(true))
	if err := developerOptions.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Developer Options")
	}

	testing.ContextLog(ctx, "Turn Backup On")
	// TODO(b/159956557): Confirm that Backup status is changing when the button is clicked.
	backup := arcDevice.Object(adbui.ClassName("android.widget.TextView"), adbui.TextMatches("(?i)backup"), adbui.Enabled(true))
	if err := backup.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Backup")
	}

	if err := backup.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Backup")
	}

	backupToggleOff := arcDevice.Object(adbui.ClassName("android.widget.Switch"), adbui.TextMatches("(?i)off"), adbui.Enabled(true))
	backupToggleOn := arcDevice.Object(adbui.ClassName("android.widget.Switch"), adbui.TextMatches("(?i)on"), adbui.Enabled(true))

	if err := backupToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Backup Off Toggle")
	}

	if err := backupToggleOff.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Backup toggle")
	}

	if err := backupToggleOn.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to turn Backup toggle On")
	}

	testing.ContextLog(ctx, "Turn Backup Off")
	if err := backupToggleOn.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click backup toggle")
	}

	turnOffBackup := arcDevice.Object(adbui.ClassName("android.widget.Button"), adbui.TextMatches("(?i)turn off & delete"), adbui.Enabled(true))
	if err := turnOffBackup.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Turn Off backup button")
	}

	if err := turnOffBackup.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Turn Off backup button")
	}

	if err := backupToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to turn off Backup")
	}

	for i := 0; i < 2; i++ {
		if err := backButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Back Button")
		}
	}

	testing.ContextLog(ctx, "Turn Location On")
	security := arcDevice.Object(adbui.ClassName("android.widget.TextView"), adbui.TextMatches("(?i)security & location"), adbui.Enabled(true))
	if err := security.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Security & location TextView")
	}

	if err := security.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Security & Location")
	}

	location := arcDevice.Object(adbui.ClassName("android.widget.TextView"), adbui.TextMatches("(?i)location"), adbui.Enabled(true))
	if err := location.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Location TextView")
	}

	if err := location.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Location")
	}

	locationToggleOff := arcDevice.Object(adbui.ClassName("android.widget.Switch"), adbui.TextMatches("(?i)off"), adbui.Enabled(true))
	locationToggleOn := arcDevice.Object(adbui.ClassName("android.widget.Switch"), adbui.TextMatches("(?i)on"), adbui.Enabled(true))

	if err := locationToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding location toggle button")
	}

	if err := locationToggleOff.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Location toggle")
	}

	if err := locationToggleOn.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to switch Location ON")
	}

	testing.ContextLog(ctx, "Turn Location Off")
	if err := locationToggleOn.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Location toggle")
	}

	if err := locationToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to switch Location OFF")
	}

	return nil
}
