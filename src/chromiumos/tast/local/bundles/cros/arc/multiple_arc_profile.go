// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	androidui "chromiumos/tast/local/android/ui"
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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultipleArcProfile,
		Desc:         "Verify that Second Account can be added from ARC Settings ",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 6*time.Minute,
		Vars:    []string{"arc.username", "arc.password", "arc.parentUser", "arc.parentPassword"},
	})
}

func MultipleArcProfile(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("arc.username")
	password := s.RequiredVar("arc.password")

	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{
			User: username,
			Pass: password,
		}),
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

	s.Log("Add ARC Account")
	if err := optinPlayStore(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to Optin to PlayStore: ", err)
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

	if err := addARCAccount(ctx, d, tconn, s); err != nil {
		s.Fatal("Failed to Add Account: ", err)
	}

	if err := switchPlayStoreAccount(ctx, d, tconn, s); err != nil {
		s.Fatal("Failed to Switch Account: ", err)
	}

	s.Log("Installing an app in new acount")
	if err := playstore.InstallApp(ctx, a, d, "com.google.android.apps.dynamite", 3); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	// Check the newly downloaded app in Launcher.
	if err := launcher.LaunchAndWaitForAppOpen(tconn, apps.Chat)(ctx); err != nil {
		s.Fatal("Failed to launch: ", err)
	}
}

// switchPlayStoreAccount switches between the ARC account in PlayStore.
func switchPlayStoreAccount(ctx context.Context, arcDevice *androidui.Device,
	tconn *chrome.TestConn, s *testing.State) error {
	if err := launcher.LaunchApp(tconn, apps.PlayStore.Name)(ctx); err != nil {
		return errors.Wrap(err, "failed to launch Play Store")
	}

	noThanksButton := arcDevice.Object(androidui.ClassName("android.widget.Button"),
		androidui.TextMatches("(?i)No thanks"))
	if err := noThanksButton.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Log("No Thanks button doesn't exists: ", err)
	} else if err := noThanksButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click No Thanks button")
	}

	avatarIcon := arcDevice.Object(androidui.ClassName("android.widget.FrameLayout"),
		androidui.DescriptionContains("Open account menu"))
	if err := avatarIcon.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Avatar Icon")
	}

	accountNameButton := arcDevice.Object(androidui.ClassName("android.widget.TextView"),
		androidui.Text("Shireen Snow"))
	if err := accountNameButton.WaitForExists(ctx, 30*time.Second); err != nil {
		return errors.Wrap(err, "failed to find Account Name")
	}
	if err := accountNameButton.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click Account Name")
	}
	return nil
}

// optinPlayStore performs the PlayStore sign in, waits until it launches and closes it.
func optinPlayStore(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	// Optin to PlayStore.
	if err := optin.Perform(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to optin to Play Store")
	}
	if err := optin.WaitForPlayStoreShown(ctx, tconn, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait for Play Store")
	}

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to close Play Store")
	}
	return nil
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
		return errors.Wrap(err, "failed finding About Device Text View")
	}

	if err := addAccount.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click About Device")
	}

	// Enter User Name.
	if err := uiauto.Combine("Click on User Name",
		ui.WaitUntilExists(nodewith.Name("Email or phone").Role(role.TextField)),
		ui.LeftClick(nodewith.Name("Email or phone").Role(role.TextField)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on user name")
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	if err := kb.Type(ctx, secondUser+"\n"); err != nil {
		return errors.Wrap(err, "failed to type user name")
	}

	// Enter Password.
	if err := uiauto.Combine("Click on Password",
		ui.WaitUntilExists(nodewith.Name("Enter your password").Role(role.TextField)),
		ui.LeftClick(nodewith.Name("Enter your password").Role(role.TextField)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on password")
	}

	if err := kb.Type(ctx, secondPassword); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	if err := uiauto.Combine("Agree and Finish Adding Account",
		ui.LeftClick(nodewith.Name("Next").Role(role.Button)),
		ui.LeftClick(nodewith.Name("I agree").Role(role.Button)),
		ui.WaitUntilExists(nodewith.Name("Manage Android preferences").Role(role.Link)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to add account")
	}
	return nil
}
