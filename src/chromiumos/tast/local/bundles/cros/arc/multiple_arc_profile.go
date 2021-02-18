// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultipleArcProfile,
		Desc:         "Verify PlayStore can be turned off in Settings ",
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
		Vars:    []string{"arc.parentUser", "arc.parentPassword"},
	})
}

func MultipleArcProfile(ctx context.Context, s *testing.State) {

	parentUser := s.RequiredVar("arc.parentUser")
	parentPass := s.RequiredVar("arc.parentPassword")

	var cr *chrome.Chrome
	var err error

	cr, err = chrome.New(ctx, chrome.GAIALogin(),
		chrome.Auth(parentUser, parentPass, "gaia-id"), chrome.ARCSupported())
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
	if err := uiauto.Run(ctx,
		settings.FocusAndWait(playStoreButton),
		settings.LeftClick(playStoreButton),
		ui.LeftClick(nodewith.Name("Manage Android preferences").Role(role.Link)),
	); err != nil {
		s.Fatal("Failed to open ARC Settings: ", err)
	}

	s.Log("Add ARC Account")
	if err := addAccount(ctx, d); err != nil {
		s.Fatal("Failed to Add Account: ", err)
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	if err := uiauto.Run(ctx,
		ui.WaitUntilExists(nodewith.Name("Email or phone").Role(role.TextField)),
		ui.LeftClick(nodewith.Name("Email or phone").Role(role.TextField)),
	); err != nil {
		s.Fatal("Failed to click user name: ", err)
	}

	// Type something.
	if err := kb.Type(ctx, "nrenugatest@gmail.com"); err != nil {
		s.Fatal("Failed to write gmail: ", err)
	}

	if err := uiauto.Run(ctx,
		ui.LeftClick(nodewith.Name("Next").Role(role.Button)),
	); err != nil {
		s.Fatal("Failed to click user name: ", err)
	}

}

func addAccount(ctx context.Context, arcDevice *androidui.Device) error {
	const (
		scrollClassName = "android.widget.ScrollView"
	)

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

	return nil
}
