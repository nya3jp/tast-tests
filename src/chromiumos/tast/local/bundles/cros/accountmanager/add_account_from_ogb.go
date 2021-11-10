// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/accountmanager"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/launcher"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AddAccountFromOgb,
		Desc:         "Verify that a secondary account can be added from One Google Bar ",
		Contacts:     []string{"anastasiian@chromium.org", "team-dent@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "loggedInToChromeAndArc",
			Val:               lacros.ChromeTypeChromeOS,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "loggedInToChromeAndArc",
			Val:               lacros.ChromeTypeChromeOS,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "loggedInToChromeAndArcWithLacros",
			Val:               lacros.ChromeTypeLacros,
		}, {
			Name:              "vm_lacros",
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "loggedInToChromeAndArcWithLacros",
			Val:               lacros.ChromeTypeLacros,
		}},
		VarDeps: []string{"accountmanager.username1", "accountmanager.password1"},
		Timeout: chrome.LoginTimeout + optin.OptinTimeout + arc.BootTimeout + 6*time.Minute,
	})
}

func AddAccountFromOgb(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.username1")
	password := s.RequiredVar("accountmanager.password1")

	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	var cr *chrome.Chrome
	var ci accountmanager.ChromeInterface
	lacrosChromeType := s.Param().(lacros.ChromeType)
	switch lacrosChromeType {
	case lacros.ChromeTypeChromeOS:
		cr = s.FixtValue().(accountmanager.FixtureData).Chrome
		ci = cr
	case lacros.ChromeTypeLacros:
		var l *launcher.LacrosChrome
		var err error
		cr, l, _, err = lacros.Setup(ctx, s.FixtValue().(accountmanager.FixtureData).LacrosFixt, lacros.ChromeTypeLacros)
		if err != nil {
			s.Fatal("Failed to initialize lacros: ", err)
		}
		defer lacros.CloseLacrosChrome(cleanupCtx, l)
		ci = l
	}

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn).WithTimeout(accountmanager.DefaultUITimeout)
	a := s.FixtValue().(accountmanager.FixtureData).ARC

	if err := accountmanager.OpenOneGoogleBar(ctx, tconn, ci); err != nil {
		s.Fatal("Failed to open OGB: ", err)
	}

	if err := clickAddAccount(ctx, ui); err != nil {
		s.Fatal("Failed to find add account link: ", err)
	}

	s.Log("Adding a secondary Account")
	if err := accountmanager.AddAccount(ctx, tconn, username, password); err != nil {
		s.Fatal("Failed to add a secondary Account: ", err)
	}

	s.Log("Verifying that account is present in OS Settings")
	// Open Account Manager page in OS Settings and find Add Google Account button.
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "accountManager", ui.Exists(nodewith.Name("Add Google Account").Role(role.Button))); err != nil {
		s.Fatal("Failed to launch Account Manager page: ", err)
	}
	// Find "More actions, <email>" button to make sure that account was added.
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	if err := ui.WaitUntilExists(moreActionsButton)(ctx); err != nil {
		s.Fatal("Failed to find More actions button: ", err)
	}

	// Check that account is present in OGB.
	s.Log("Verifying that account is present in OGB")
	secondaryAccountListItem := nodewith.NameContaining(username).Role(role.Link)
	if err := accountmanager.CheckOneGoogleBar(ctx, tconn, ci, ui.WaitUntilExists(secondaryAccountListItem)); err != nil {
		s.Fatal("Failed to check that account is present in OGB: ", err)
	}

	// Check that account is present in ARC.
	s.Log("Verifying that account is present in ARC")
	if present, err := accountmanager.IsAccountPresentInArc(ctx, tconn, a, username); err != nil || !present {
		s.Fatalf("Failed to check that account is present in ARC, present=%v, err=%v", present, err)
	}
}

// clickAddAccount clicks on 'add another account' button in OGB.
func clickAddAccount(ctx context.Context, ui *uiauto.Context) error {
	addAccount := nodewith.Name("Add another account").Role(role.Link)
	if err := uiauto.Combine("Click add account",
		ui.WaitUntilExists(addAccount),
		ui.LeftClick(addAccount),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to find and click add account link")
	}
	return nil
}
