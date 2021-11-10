// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accountmanager provides functions to manage accounts in-session.
package accountmanager

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/accountmanager"
	androidui "chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddAccountOsSettings,
		Desc:         "Verify that a secondary account can be added from OS Settings ",
		Contacts:     []string{"anastasiian@chromium.org", "team-dent@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		VarDeps: []string{"ui.gaiaPoolDefault", "accountmanager.username1", "accountmanager.password1"},
		Timeout: chrome.LoginTimeout + arc.BootTimeout + 6*time.Minute,
	})
}

func AddAccountOsSettings(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.username1")
	password := s.RequiredVar("accountmanager.password1")

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Opt-in to Play Store.
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

	ui := uiauto.New(tconn).WithTimeout(30 * time.Second)

	// Open Account Manager page in OS Settings and find Add Google Account button.
	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "accountManager", ui.Exists(addAccountButton)); err != nil {
		s.Fatal("Failed to launch Account Manager page: ", err)
	}

	// Click the button to open account addition dialog.
	if err := ui.LeftClick(addAccountButton)(ctx); err != nil {
		s.Fatal("Failed to click Add Google Account button: ", err)
	}

	s.Log("Adding a secondary Account")
	if err := accountmanager.AddAccount(ctx, tconn, username, password); err != nil {
		s.Fatal("Failed to add a secondary Account: ", err)
	}

	// Make sure that the settings page is focused again.
	if err := ui.WaitUntilExists(addAccountButton)(ctx); err != nil {
		s.Fatal("Failed to find Add Google Account button: ", err)
	}
	// Find "More actions, <email>" button to make sure that account was added.
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	if err := ui.WaitUntilExists(moreActionsButton)(ctx); err != nil {
		s.Fatal("Failed to find More actions button: ", err)
	}

	// Check that account is present in OGB.
	secondaryAccountListItem := nodewith.NameContaining(username).Role(role.Link)
	if err := accountmanager.CheckOneGoogleBar(ctx, cr, tconn, ui.WaitUntilExists(secondaryAccountListItem)); err != nil {
		s.Fatal("Failed to check that account is present in OGB: ", err)
	}

	// Check that account is present in ARC.
	if present, err := isAccountPresentInArc(ctx, tconn, a, username); err != nil || !present {
		s.Fatalf("Failed to check that account is present in ARC, present=%v, err=%v", present, err)
	}

	// Open OS Settings again.
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "accountManager", ui.Exists(addAccountButton)); err != nil {
		s.Fatal("Failed to launch Account Manager page: ", err)
	}
	// Find and click "More actions, <email>" button.
	if err := uiauto.Combine("Click More actions",
		ui.WaitUntilExists(moreActionsButton),
		ui.LeftClick(moreActionsButton),
	)(ctx); err != nil {
		s.Fatal("Failed to click More actions button: ", err)
	}

	removeAccountButton := nodewith.Name("Remove this account").Role(role.MenuItem)
	if err := uiauto.Combine("Remove account",
		ui.WaitUntilExists(removeAccountButton),
		ui.LeftClick(removeAccountButton),
		ui.WaitUntilGone(moreActionsButton),
	)(ctx); err != nil {
		s.Fatal("Failed to remove account: ", err)
	}

	// Check that account is not present in OGB anymore.
	if err := accountmanager.CheckOneGoogleBar(ctx, cr, tconn, ui.WaitUntilGone(secondaryAccountListItem)); err != nil {
		s.Fatal("Failed to remove account from OGB: ", err)
	}

	// Check that account is not present in ARC.
	if present, err := isAccountPresentInArc(ctx, tconn, a, username); err != nil || present {
		s.Fatalf("Failed to check that account is NOT present in ARC, present=%v, err=%v", present, err)
	}
}

// isAccountPresentInArc returns `true` if account is present in ARC Settings > Accounts.
func isAccountPresentInArc(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, accountName string) (bool, error) {
	const (
		scrollClassName   = "android.widget.ScrollView"
		textViewClassName = "android.widget.TextView"
	)

	// Initialize UI automator for ARC.
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed initializing UI Automator")
	}
	defer d.Close(ctx)

	if err := openARCSettings(ctx, tconn); err != nil {
		return false, errors.Wrap(err, "failed to Open ARC Settings")
	}

	defer func(ctx context.Context) error {
		// Cleanup: close the ARC Settings window.
		activeWindow, err := ash.GetActiveWindow(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get the active window")
		}
		if err := activeWindow.CloseWindow(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to close the active window "+activeWindow.Name)
		}
		return nil
	}(ctx)

	// Scroll until Accounts is visible.
	scrollLayout := d.Object(androidui.ClassName(scrollClassName),
		androidui.Scrollable(true))
	accounts := d.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches("(?i)Accounts"), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, accountmanager.DefaultUITimeout); err == nil {
		scrollLayout.ScrollTo(ctx, accounts)
	}
	if err := accounts.Click(ctx); err != nil {
		return false, errors.Wrap(err, "failed to click Accounts in ARC settings")
	}

	account := d.Object(androidui.ClassName("android.widget.TextView"),
		androidui.TextMatches(accountName), androidui.Enabled(true))
	if err := scrollLayout.WaitForExists(ctx, accountmanager.DefaultUITimeout); err == nil {
		scrollLayout.ScrollTo(ctx, account)
	}

	if err := account.Exists(ctx); err != nil {
		return false, nil
	}
	return true, nil
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
