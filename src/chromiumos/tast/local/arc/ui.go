// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/android/ui"
	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
)

// NewUIDevice creates a Device object by starting and connecting to UI Automator server.
// Close must be called to clean up resources when a test is over.
func (a *ARC) NewUIDevice(ctx context.Context) (*ui.Device, error) {
	return ui.NewDeviceWithRetry(ctx, a.device)
}

// DumpUIHierarchyOnError dumps arc UI hierarchy to 'arc_uidump.xml', when the test fails.
// Call this function after closing arc UI devices. Otherwise the uiautomator might exist with errors like
// status 137.
func (a *ARC) DumpUIHierarchyOnError(ctx context.Context, outDir string, hasError func() bool) error {
	if !hasError() {
		return nil
	}

	dumpFile := "/sdcard/window_dump.xml"

	if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to dump arc UI")
	}
	defer a.Command(ctx, "rm", dumpFile).Run(testexec.DumpLogOnError)

	dir := filepath.Join(outDir, "faillog")
	if err := os.MkdirAll(dir, 0777); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}

	file := filepath.Join(dir, "arc_uidump.xml")
	if err := a.PullFile(ctx, dumpFile, file); err != nil {
		return errors.Wrap(err, "failed to pull UI dump to outDir")
	}

	return nil
}

// OpenPlayStoreAccountSettings opens account settings in PlayStore where user
// can switch between available accounts.
func OpenPlayStoreAccountSettings(ctx context.Context, arcDevice *androidui.Device, tconn *chrome.TestConn) error {
	// Ensure the Play Store is closed so that we don't resume another session.
	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		testing.ContextLog(ctx, "Failed to close Play Store: ", err)
	}

	if err := launcher.LaunchApp(tconn, apps.PlayStore.Name)(ctx); err != nil {
		return errors.Wrap(err, "failed to launch Play Store")
	}

	noThanksButton := arcDevice.Object(androidui.ClassName("android.widget.Button"),
		androidui.TextMatches("(?i)No thanks"))
	if err := noThanksButton.WaitForExists(ctx, 10*time.Second); err != nil {
		testing.ContextLog(ctx, "No Thanks button doesn't exist: ", err)
	} else if err := noThanksButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click No Thanks button")
	}

	avatarIcon := arcDevice.Object(androidui.ClassName("android.widget.FrameLayout"),
		androidui.DescriptionContains("Account and settings"))
	if err := avatarIcon.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Avatar Icon")
	}

	expandAccountButton := arcDevice.Object(androidui.ClassName("android.view.ViewGroup"), androidui.Clickable(true))
	if err := expandAccountButton.WaitForExists(ctx, 10*time.Second); err != nil {
		testing.ContextLog(ctx, "Expand account button doesn't exist: ", err)
	} else if err := expandAccountButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click expand account button")
	}
	return nil
}

// SwitchPlayStoreAccount switches between the ARC account in PlayStore.
func SwitchPlayStoreAccount(ctx context.Context, arcDevice *androidui.Device, tconn *chrome.TestConn, accountEmail string) error {
	accountNameButton := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.Text(accountEmail))

	if err := uiauto.Retry(3, func(ctx context.Context) error {
		// The added new account sometimes need more time to be reflected in Play Store settings.
		if err := OpenPlayStoreAccountSettings(ctx, arcDevice, tconn); err != nil {
			return errors.Wrap(err, "failed to open Play Store account settings")
		}

		if err := accountNameButton.WaitForExists(ctx, 30*time.Second); err != nil {
			return errors.Wrap(err, "failed to find Account Name")
		}
		return nil
	})(ctx); err != nil {
		return errors.Wrap(err, "failed to open Play Store account settings and find account name")
	}

	if err := accountNameButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Account Name")
	}

	return nil
}

// ClickAddAccountInSettings clicks Add account > Google in ARC Settings. Settings window should be already open.
func ClickAddAccountInSettings(ctx context.Context, arcDevice *androidui.Device, tconn *chrome.TestConn) error {
	const (
		scrollClassName   = "android.widget.ScrollView"
		textViewClassName = "android.widget.TextView"
	)

	// Scroll until Accounts is visible.
	scrollLayout := arcDevice.Object(androidui.ClassName(scrollClassName),
		androidui.Scrollable(true))
	accounts := arcDevice.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches("(?i)Accounts"), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, 10*time.Second); err == nil {
		scrollLayout.ScrollTo(ctx, accounts)
	}
	if err := accounts.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on Accounts")
	}

	addAccount := arcDevice.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches("(?i)Add account"), androidui.Enabled(true))
	if err := addAccount.WaitForExists(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed finding Add account")
	}
	if err := addAccount.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Add account")
	}

	// Click on Google button which appears only on tablet flow.
	gaiaButton := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)Google"), androidui.Enabled(true), androidui.ResourceIDMatches("(android:id/title)"))
	if err := gaiaButton.WaitForExists(ctx, 10*time.Second); err != nil {
		testing.ContextLog(ctx, "Google button doesn't exist: ", err)
	} else if err := gaiaButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Google")
	}
	return nil
}
