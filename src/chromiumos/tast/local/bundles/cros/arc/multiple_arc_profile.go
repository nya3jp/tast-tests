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
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 180*time.Second,
		Vars:    []string{"arc.username", "arc.password", "arc.parentUser", "arc.parentPassword"},
	})
}

func MultipleArcProfile(ctx context.Context, s *testing.State) {

	username := s.RequiredVar("arc.username")
	password := s.RequiredVar("arc.password")

	cr, err := chrome.New(ctx, chrome.GAIALogin(),
		chrome.Auth(username, password, "gaia-id"), chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

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

	if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
		s.Log("Failed to close Play Store: ", err)
	}

	// Navigate to Android Settings.
	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Apps").Role(role.Heading))
	if err != nil {
		s.Fatal("Failed to open settings page: ", err)
	}

	ui := uiauto.New(tconn)
	playStoreButton := nodewith.Name("Google Play Store").Role(role.Button)
	if err := uiauto.Combine("Open Android Settings",
		settings.FocusAndWait(playStoreButton),
		settings.LeftClick(playStoreButton),
		settings.LeftClick(nodewith.Name("Manage Android preferences").Role(role.Link)),
	)(ctx); err != nil {
		s.Fatal("Failed to Open Android Settings : ", err)
	}

	s.Log("Add ARC Account")
	if err := addArcAccount(ctx, d, ui, s); err != nil {
		s.Fatal("Failed to Add Account: ", err)
	}

	s.Log("Open PlayStore and Switch Account")
	if err := launcher.LaunchApp(tconn, apps.PlayStore.Name)(ctx); err != nil {
		s.Fatal("Failed to launch Play Store: ", err)
	}

	noThanksButton := d.Object(androidui.ClassName("android.widget.Button"), androidui.Text("NO THANKS"))
	if err := noThanksButton.WaitForExists(ctx, 10*time.Second); err != nil {
		s.Log("noThanksButton doesn't exists: ", err)
	} else if err := noThanksButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on noThanksButton: ", err)
	}

	avatarIcon := d.Object(androidui.ClassName("android.widget.FrameLayout"),
		androidui.DescriptionContains("Open account menu"))
	if err := avatarIcon.Click(ctx); err != nil {
		s.Fatal("Failed to click Avatar: ", err)
	}

	accountNameButton := d.Object(androidui.ClassName("android.widget.TextView"), androidui.Text("Shireen Snow"))
	if err := accountNameButton.WaitForExists(ctx, 30*time.Second); err != nil {
		s.Fatal("accountNameButton doesn't exists: ", err)
	} else if err := accountNameButton.Click(ctx); err != nil {
		s.Fatal("Failed to click on accountNameButton: ", err)
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

func addArcAccount(ctx context.Context, arcDevice *androidui.Device, ui *uiauto.Context, s *testing.State) error {
	const (
		scrollClassName   = "android.widget.ScrollView"
		textViewClassName = "android.widget.TextView"
	)

	secondUser := s.RequiredVar("arc.parentUser")
	secondPassword := s.RequiredVar("arc.parentPassword")

	// Scroll until Accounts is visible.
	scrollLayout := arcDevice.Object(androidui.ClassName(scrollClassName), androidui.Scrollable(true))
	accounts := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)Accounts"), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, 10*time.Second); err == nil {
		scrollLayout.ScrollTo(ctx, accounts)
	}

	if err := accounts.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click on System")
	}

	addAccount := arcDevice.Object(androidui.ClassName("android.widget.TextView"), androidui.TextMatches("(?i)Add account"), androidui.Enabled(true))

	if err := addAccount.WaitForExists(ctx, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed finding About Device Text View")
	}

	if err := addAccount.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click About Device")
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	// Enter User Name
	if err := uiauto.Combine("Click on User Name",
		ui.WaitUntilExists(nodewith.Name("Email or phone").Role(role.TextField)),
		ui.LeftClick(nodewith.Name("Email or phone").Role(role.TextField)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on user name")
	}

	if err := kb.Type(ctx, secondUser+"\n"); err != nil {
		return errors.Wrap(err, "failed to type user name")
	}

	// Enter Password
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
