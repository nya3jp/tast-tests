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
	arcui "chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifySettingsSanity,
		Desc:         "Verifies ARC++ settings work as intended",
		Contacts:     []string{"vkrishan@google.com", "rohitbm@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 3 * time.Minute,
	})
}

// Time to wait for UI elements to appear in Play Store and Chrome
const timeoutUI = 30 * time.Second

func VerifySettingsSanity(ctx context.Context, s *testing.State) {

	// Chrome Login
	p := s.PreValue().(arc.PreData)
	a := p.ARC
	cr := p.Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	d, err := arcui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	// Navigate to Andoroid Settings
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch the Settings app: ", err)
	}

	appsParam := ui.FindParams{
		Role: ui.RoleTypeHeading,
		Name: "Apps",
	}

	apps, err := ui.FindWithTimeout(ctx, tconn, appsParam, 10*time.Second)
	if err != nil {
		s.Fatal("Waiting to find Apps Heading failed: ", err)
	}
	defer apps.Release(ctx)

	if err := apps.FocusAndWait(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to call focus() on the Advanced button: ", err)
	}

	if err := apps.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click the Apps Heading: ", err)
	}

	webview, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeWebView, ClassName: "WebView"}, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find webview: ", err)
	}
	defer webview.Release(ctx)

	enter, err := webview.DescendantWithTimeout(ctx, ui.FindParams{Name: "Manage Android preferences"}, 10*time.Second)
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

	if err := checkPlayStoreSettings(ctx, d); err != nil {
		s.Fatal("Failed Checking Playstore Settings: ", err)
	}
}

func checkPlayStoreSettings(ctx context.Context, arcDevice *arcui.Device) error {

	backButton := arcDevice.Object(arcui.ClassName("android.widget.ImageButton"), arcui.Enabled(true))

	// Verify 'System' settings in ARC++
	system := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)system"), arcui.Enabled(true))
	if err := system.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding 'System' Text View")
	}

	if err := system.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on System")
	}

	// Find And click 'About Device'
	aboutDevice := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)about device"), arcui.Enabled(true))
	if err := aboutDevice.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding 'About Device' Text View")
	}

	if err := aboutDevice.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click 'About Device'")
	}

	buildNumber := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)build number"), arcui.Enabled(true))
	if err := buildNumber.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding 'Build Number' TextView")
	}

	// Enable and verify 'Developer Options'
	for i := 0; i < 7; i++ {
		if err := buildNumber.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click 'Build Number' TextView")
		}
	}

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

	//Turn Backup ON
	backup := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)backup"), arcui.Enabled(true))
	if err := backup.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding 'Backup'")
	}

	if err := backup.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click 'Backup'")
	}

	backupSwitchOff := arcDevice.Object(arcui.ClassName("android.widget.Switch"), arcui.TextMatches("(?i)off"), arcui.Enabled(true))
	if err := backupSwitchOff.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Backup Switch Off")
	}

	if err := backupSwitchOff.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Backup switch")
	}

	backupSwitchOn := arcDevice.Object(arcui.ClassName("android.widget.Switch"), arcui.TextMatches("(?i)on"), arcui.Enabled(true))
	if err := backupSwitchOn.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to turn Backup switch On")
	}

	// Turn Backup OFF
	if err := backupSwitchOn.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click backup switch")
	}

	turnOffTitle := arcDevice.Object(arcui.ClassName("android.widget.Button"), arcui.TextMatches("(?i)turn off & delete"), arcui.Enabled(true))
	if err := turnOffTitle.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Turn Off backup button")
	}

	if err := turnOffTitle.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Turn Off backup button")
	}

	// Turn Location ON and OFF
	for i := 0; i < 2; i++ {
		if err := backButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Back Button")
		}
	}

	security := arcDevice.Object(arcui.ClassName("android.widget.TextView"), arcui.TextMatches("(?i)security & location"), arcui.Enabled(true))
	if err := security.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding 'Security & location' TextView")
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

	locationToggleOff := arcDevice.Object(arcui.ClassName("android.widget.Switch"), arcui.TextMatches("(?i)off"), arcui.Enabled(true))
	locationToggleOn := arcDevice.Object(arcui.ClassName("android.widget.Switch"), arcui.TextMatches("(?i)on"), arcui.Enabled(true))

	if err := locationToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding location toggle button")
	}

	if err := locationToggleOff.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Location toggle")
	}

	if err := locationToggleOn.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to switch Location ON")
	}

	if err := locationToggleOn.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Location toggle")
	}

	if err := locationToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to switch Location OFF")
	}

	return nil
}
