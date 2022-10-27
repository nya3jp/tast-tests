// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/accountmanager"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddAccountFromOGB,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify that a secondary account can be added from One Google Bar",
		Contacts:     []string{"anastasiian@chromium.org", "team-dent@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "loggedInToChromeAndArc",
			Val:               browser.TypeAsh,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "loggedInToChromeAndArc",
			Val:               browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "loggedInToChromeAndArcWithLacros",
			Val:               browser.TypeLacros,
		}, {
			Name:              "vm_lacros",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "loggedInToChromeAndArcWithLacros",
			Val:               browser.TypeLacros,
		}},
		VarDeps: []string{"accountmanager.username1", "accountmanager.password1"},
		Timeout: 7 * time.Minute,
	})
}

func AddAccountFromOGB(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.username1")
	password := s.RequiredVar("accountmanager.password1")

	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr := s.FixtValue().(accountmanager.FixtureData).Chrome()

	// Setup the browser.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Running test cleanup")
		if err := accountmanager.TestCleanup(ctx, tconn, cr); err != nil {
			s.Fatal("Failed to do cleanup: ", err)
		}
	}(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "add_account_from_ogb")

	ui := uiauto.New(tconn).WithTimeout(time.Minute)
	a := s.FixtValue().(accountmanager.FixtureData).ARC
	defer a.DumpUIHierarchyOnError(ctx, s.OutDir(), s.HasError)

	arcDevice, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer arcDevice.Close(ctx)

	if err := accountmanager.OpenOneGoogleBar(ctx, tconn, br); err != nil {
		s.Fatal("Failed to open OGB: ", err)
	}

	if err := clickAddAccount(ctx, ui); err != nil {
		s.Fatal("Failed to find add account link: ", err)
	}

	// ARC toggle should NOT be checked.
	if err := accountmanager.CheckARCToggleStatus(ctx, tconn, s.Param().(browser.Type), false); err != nil {
		s.Fatal("Failed to check ARC toggle status: ", err)
	}

	s.Log("Adding a secondary Account")
	if err := accountmanager.AddAccount(ctx, tconn, username, password); err != nil {
		s.Fatal("Failed to add a secondary Account: ", err)
	}

	s.Log("Verifying that account is present in OS Settings")
	// Find "More actions, <email>" button to make sure that account was added.
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	if err := uiauto.Combine("Click Add Google Account button",
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		ui.WaitUntilExists(moreActionsButton),
	)(ctx); err != nil {
		s.Fatal("Failed to find More actions button: ", err)
	}

	// Check that account is present in OGB.
	s.Log("Verifying that account is present in OGB")
	secondaryAccountListItem := nodewith.NameContaining(username).Role(role.Link)
	if err := accountmanager.CheckOneGoogleBar(ctx, tconn, br, ui.WaitUntilExists(secondaryAccountListItem)); err != nil {
		s.Fatal("Failed to check that account is present in OGB: ", err)
	}

	// Account is expected to be not present in ARC only if browser type is Lacros. The feature is being applied only if Lacros is enabled.
	expectedPresentInArc := s.Param().(browser.Type) != browser.TypeLacros
	if err := accountmanager.CheckIsAccountPresentInARCAction(tconn, arcDevice, username, expectedPresentInArc)(ctx); err != nil {
		s.Fatalf("Failed to check if account is present in ARC, expected '%t', err: %v", expectedPresentInArc, err)
	}
}

// clickAddAccount clicks on 'add another account' button in OGB until account addition dialog is opened.
func clickAddAccount(ctx context.Context, ui *uiauto.Context) error {
	addAccount := nodewith.Name("Add another account").Role(role.Link)
	dialog := accountmanager.AddAccountDialog()
	if err := uiauto.Combine("Click add account",
		ui.WaitUntilExists(addAccount),
		ui.WithInterval(time.Second).LeftClickUntil(addAccount, ui.Exists(dialog)),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to find and click add account link")
	}
	return nil
}
