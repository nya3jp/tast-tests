// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	androidui "chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/accountmanager"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultipleArcProfile,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that Second Account can be added from ARC Settings ",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 6*time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault", "arc.parentUser", "arc.parentPassword"},
	})
}

func MultipleArcProfile(ctx context.Context, s *testing.State) {

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

	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to connect to ARC: ", err)
	}
	defer a.Close(ctx)
	defer func() {
		if s.HasError() {
			if err := a.Command(ctx, "uiautomator", "dump").Run(testexec.DumpLogOnError); err != nil {
				s.Error("Failed to dump UIAutomator: ", err)
			} else if err := a.PullFile(ctx, "/sdcard/window_dump.xml",
				filepath.Join(s.OutDir(), "uiautomator_dump.xml")); err != nil {
				s.Error("Failed to pull UIAutomator dump: ", err)
			}
		}
	}()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	if err := openARCSettings(ctx, tconn); err != nil {
		s.Fatal("Failed to Open ARC Settings: ", err)
	}

	s.Log("Add ARC Account")
	if err := addARCAccount(ctx, d, tconn, s); err != nil {
		s.Fatal("Failed to Add Account: ", err)
	}

	if err := arc.SwitchPlayStoreAccount(ctx, d, tconn, s.RequiredVar("arc.parentUser")); err != nil {
		s.Fatal("Failed to Switch Account: ", err)
	}

	var translateinstalled bool
	s.Log("Installing an app in new acount")
	if err := playstore.InstallApp(ctx, a, d, "com.google.android.apps.dynamite", 3); err != nil {
		s.Log("Failed to install chat app: ", err)
		if err := playstore.InstallApp(ctx, a, d, "com.google.android.apps.translate", 3); err != nil {
			s.Fatal("Failed to install translate app: ", err)
		}
		translateinstalled = true
	}

	// Check the newly downloaded app in Launcher.
	if translateinstalled {
		if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Translate)(ctx); err != nil {
			s.Fatal("Failed to launch Translate app: ", err)
		}
		return
	}
	// TODO(b/210702593): Replace with LaunchAndWaitForAppOpen once fixed.
	if err := launcher.LaunchApp(tconn, apps.Chat.Name)(ctx); err != nil {
		s.Fatal("Failed to launch Chat app: ", err)
	}
	ui := uiauto.New(tconn)
	chatButton := nodewith.Name(apps.Chat.Name).ClassName("ash/ShelfAppButton")
	if err := ui.WaitUntilExists(chatButton)(ctx); err != nil {
		s.Fatal("Failed to find Google Chat in Shelf: ", err)
	}

}

// openARCSettings opens the ARC Settings Page from Chrome Settings.
func openARCSettings(ctx context.Context, tconn *chrome.TestConn) error {
	settings, err := ossettings.LaunchAtPage(ctx, tconn,
		nodewith.Name("Apps").Role(role.Heading))
	if err != nil {
		return errors.Wrap(err, "failed to open settings page")
	}
	playStoreButton := nodewith.Name("Google Play Store").Role(role.Button)
	if err := uiauto.Combine("Open Android Settings",
		settings.FocusAndWait(playStoreButton),
		settings.LeftClick(playStoreButton),
		settings.LeftClick(nodewith.Name("Manage Android preferences").Role(role.Link)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to open ARC settings page")
	}
	return nil
}

// addARCAccount adds a second ARC account from ARC Settings->Accounts Screen.
func addARCAccount(ctx context.Context, arcDevice *androidui.Device, tconn *chrome.TestConn,
	s *testing.State) error {
	const (
		scrollClassName   = "android.widget.ScrollView"
		textViewClassName = "android.widget.TextView"
	)

	ui := uiauto.New(tconn)
	secondUser := s.RequiredVar("arc.parentUser")
	secondPassword := s.RequiredVar("arc.parentPassword")

	// Scroll until Accounts is visible.
	scrollLayout := arcDevice.Object(androidui.ClassName(scrollClassName),
		androidui.Scrollable(true))
	accounts := arcDevice.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches("(?i)Accounts"), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, 10*time.Second); err == nil {
		scrollLayout.ScrollTo(ctx, accounts)
	}

	if err := accounts.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on System")
	}

	addAccount := arcDevice.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches("(?i)Add account"), androidui.Enabled(true))

	if err := addAccount.WaitForExists(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed finding addAccount Text View")
	}

	if err := addAccount.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click addAccount")
	}

	if err := accountmanager.AddAccount(ctx, tconn, secondUser, secondPassword); err != nil {
		return errors.Wrap(err, "failed to add account")
	}

	if err := ui.WaitUntilExists(nodewith.Name("Manage Android preferences").Role(role.Link).Focused())(ctx); err != nil {
		return errors.Wrap(err, "failed to find Manage Android preferences link")
	}
	return nil
}
