// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
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
		LacrosStatus: testing.LacrosVariantUnknown,
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
		Timeout: 6 * time.Minute,
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
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	defer func(ctx context.Context) {
		s.Log("Running test cleanup")
		if err := accountmanager.TestCleanup(ctx, tconn, cr, s.Param().(browser.Type)); err != nil {
			s.Fatal("Failed to do cleanup: ", err)
		}
	}(cleanupCtx)

	ui := uiauto.New(tconn).WithTimeout(time.Minute)
	a := s.FixtValue().(accountmanager.FixtureData).ARC

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	secondaryAccountListItem := nodewith.NameContaining(username).Role(role.Link)

	if err := uiauto.Combine("Add a secondary account",
		accountmanager.OpenOneGoogleBarAction(tconn, br),
		clickAddAccountAction(ui),
		accountmanager.CheckArcToggleStatusAction(tconn, false),
		accountmanager.AddAccountAction(tconn, username, password),
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		ui.WaitUntilExists(moreActionsButton),
		accountmanager.CheckOneGoogleBarAction(tconn, br, ui.WaitUntilExists(secondaryAccountListItem)),
		// Check that account is not present in ARC.
		accountmanager.CheckAccountNotPresentInArcAction(tconn, d, username),
	)(ctx); err != nil {
		s.Fatal("Failed to add a secondary account: ", err)
	}
}

// clickAddAccountAction returns an action that clicks on 'add another account' button in OGB.
func clickAddAccountAction(ui *uiauto.Context) action.Action {
	return func(ctx context.Context) error {
		addAccount := nodewith.Name("Add another account").Role(role.Link)
		if err := uiauto.Combine("Click add account",
			ui.WaitUntilExists(addAccount),
			ui.LeftClick(addAccount),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to find and click add account link")
		}
		return nil
	}
}
