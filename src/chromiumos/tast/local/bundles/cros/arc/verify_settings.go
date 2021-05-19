// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const timeoutUI = 30 * time.Second

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifySettings,
		Desc:         "Verifies ARC++ settings work as intended",
		Contacts:     []string{"vkrishan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		Vars:    []string{"ui.gaiaPoolDefault"},
	})
}

func VerifySettings(ctx context.Context, s *testing.State) {

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Optin to PlayStore and Close
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	ui := uiauto.New(tconn)
	playStoreButton := nodewith.Name("Google Play Store").Role(role.Button)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "apps", ui.Exists(playStoreButton)); err != nil {
		s.Fatal("Failed to launch apps settings page: ", err)
	}

	if err := uiauto.Combine("Open Android Settings",
		ui.FocusAndWait(playStoreButton),
		ui.LeftClick(playStoreButton),
		ui.LeftClick(nodewith.Name("Manage Android preferences").Role(role.Link)),
	)(ctx); err != nil {
		s.Fatal("Failed to Open Android Settings : ", err)
	}

	if err := checkAndroidSettings(ctx, d); err != nil {
		s.Fatal("Failed checking Android Settings: ", err)
	}
}

func checkAndroidSettings(ctx context.Context, arcDevice *androidui.Device) error {
	const (
		scrollClassName = "android.widget.ScrollView"
		locationID      = "com.android.settings:id/switch_widget"
	)

	// Scroll until logout is visible.
	scrollLayout := arcDevice.Object(androidui.ClassName(scrollClassName), androidui.Scrollable(true))
	system := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)system"), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, timeoutUI); err == nil {
		scrollLayout.ScrollTo(ctx, system)
	}

	t, ok := arc.Type()
	if !ok {
		return errors.New("Unable to determine arc type")
	}
	// If ARC-P, check for About Device in System.
	if t == arc.Container {
		// Verify System settings in ARC++.
		if err := system.WaitForExists(ctx, timeoutUI); err != nil {
			return errors.Wrap(err, "failed finding System Text View")
		}

		if err := system.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click on System")
		}
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

	// If ARCVM, navigate back into System.
	if t == arc.VM {
		if err := system.WaitForExists(ctx, timeoutUI); err != nil {
			return errors.Wrap(err, "failed finding System Text View")
		}

		if err := system.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click on System")
		}
	}

	developerOptions := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)developer options"), androidui.Enabled(true))
	if err := developerOptions.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Developer Options")
	}

	backup := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)backup"), androidui.Enabled(true))
	if err := backup.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Backup")
	}

	if err := backup.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Backup")
	}

	var err error
	if t == arc.VM {
		err = testBackupButtonVM(ctx, arcDevice)
	} else {
		err = testBackupButton(ctx, arcDevice)
	}

	if err != nil {
		return errors.Wrap(err, "failed to turn backup off and on")
	}

	for i := 0; i < 2; i++ {
		if err := backButton.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Back Button")
		}
	}

	// If ARC-P, navigate to Security & Location.
	if t == arc.Container {
		testing.ContextLog(ctx, "Toggle Location Settings")
		security := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)security & location"), androidui.Enabled(true))
		if err := security.WaitForExists(ctx, timeoutUI); err != nil {
			return errors.Wrap(err, "failed finding Security & location TextView")
		}

		if err := security.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Security & Location")
		}
	}

	location := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)location"), androidui.Enabled(true))
	if err := location.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed finding Location TextView")
	}

	if err := location.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Location")
	}

	// locationStatus will check for toggle On/Off
	locationStatus, err := arcDevice.Object(androidui.ID(locationID)).IsChecked(ctx)
	if err != nil {
		return err
	}
	locationToggle := arcDevice.Object(androidui.ID(locationID))

	if locationStatus == true {
		// Turn Location Off.
		if err := locationToggle.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click Location toggle")
		}
	}

	// Turn Location On.
	if err := locationToggle.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Location toggle")
	}

	if locationStatus == false {
		return errors.New("Unable to Turn Location ON")
	}

	return nil
}

// testBackupButtonVM verifes if backup button can be turned off and on in ARC-R
func testBackupButtonVM(ctx context.Context, arcDevice *androidui.Device) error {

	const backupID = "com.google.android.gms:id/switchWidget"
	backupToggle := arcDevice.Object(androidui.ID(backupID))

	// backupStatus will check for toggle on/off.
	backupStatus, err := arcDevice.Object(androidui.ID(backupID)).IsChecked(ctx)
	if err != nil {
		return err
	}

	if backupStatus == true {
		// Turn Backup OFF.
		if err := backupToggle.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click backup toggle")
		}

		turnOffBackup := arcDevice.Object(androidui.ClassName("android.widget.Button"), androidui.TextMatches("(?i)turn off & delete"), androidui.Enabled(true))
		if err := turnOffBackup.WaitForExists(ctx, timeoutUI); err != nil {
			return errors.Wrap(err, "failed to find turn off & delete button")
		}

		if err := turnOffBackup.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click turn off & delete button")
		}
	}
	// Turn Backup ON.
	if err := backupToggle.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click backup toggle")
	}

	backupStatus, err = arcDevice.Object(androidui.ID(backupID)).IsChecked(ctx)
	if err != nil {
		return err
	}
	if backupStatus == false {
		return errors.New("Unable to turn Backup ON")
	}
	return nil
}

// testBackupButton verifes if backup button can be turned off and on in ARC-P
func testBackupButton(ctx context.Context, arcDevice *androidui.Device) error {

	const backupID = "com.google.android.gms:id/switchWidget"

	backupStatus, err := arcDevice.Object(androidui.ID(backupID)).GetText(ctx)
	if err != nil {
		return err
	}

	backupToggleOff := arcDevice.Object(androidui.ClassName("android.widget.Switch"), androidui.TextMatches("(?i)off"), androidui.Enabled(true))
	backupToggleOn := arcDevice.Object(androidui.ClassName("android.widget.Switch"), androidui.TextMatches("(?i)on"), androidui.Enabled(true))

	if backupStatus == "ON" {
		// Turn Backup OFF.
		if err := backupToggleOn.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click backup toggle")
		}

		turnOffBackup := arcDevice.Object(androidui.ClassName("android.widget.Button"), androidui.TextMatches("(?i)turn off & delete"), androidui.Enabled(true))
		if err := turnOffBackup.WaitForExists(ctx, timeoutUI); err != nil {
			return errors.Wrap(err, "failed to find turn off & delete button")
		}

		if err := turnOffBackup.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click turn off & delete button")
		}
	}
	// Turn Backup ON.
	if err := backupToggleOff.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to find backup off toggle")
	}

	if err := backupToggleOff.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click backup toggle")
	}

	if err := backupToggleOn.WaitForExists(ctx, timeoutUI); err != nil {
		return errors.Wrap(err, "failed to turn backup toggle on")
	}
	return nil
}
