// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
)

// NewUIDevice creates a Device object by starting and connecting to UI Automator server.
// Close must be called to clean up resources when a test is over.
func (a *ARC) NewUIDevice(ctx context.Context) (*ui.Device, error) {
	return ui.NewDevice(ctx, a.device)
}

// DumpUIHierarchyOnError dumps arc UI hierarchy to 'arc_uidump.xml', when the test fails.
// Call this function after closing arc UI devices. Otherwise the uiautomator might exist with errors like
// status 137.
func (a *ARC) DumpUIHierarchyOnError(ctx context.Context, outDir string, hasError func() bool) error {
	if !hasError() {
		return nil
	}

	if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to dump arc UI")
	}

	dir := filepath.Join(outDir, "faillog")
	if err := os.MkdirAll(dir, 0777); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}

	file := filepath.Join(dir, "arc_uidump.xml")
	if err := a.PullFile(ctx, "/sdcard/window_dump.xml", file); err != nil {
		return errors.Wrap(err, "failed to pull UI dump to outDir")
	}

	return nil
}

// OpenPlayStoreAccountSettings opens account settings in PlayStore where user
// can switch between available accounts.
func OpenPlayStoreAccountSettings(ctx context.Context, arcDevice *androidui.Device, tconn *chrome.TestConn) error {
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
	if err := OpenPlayStoreAccountSettings(ctx, arcDevice, tconn); err != nil {
		return errors.Wrap(err, "failed to open Play Store account settings")
	}

	accountNameButton := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.Text(accountEmail))
	if err := accountNameButton.WaitForExists(ctx, 30*time.Second); err != nil {
		return errors.Wrap(err, "failed to find Account Name")
	}
	if err := accountNameButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Account Name")
	}
	return nil
}
