// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package accountmanager

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/accountmanager"
	"chromiumos/tast/local/chrome"
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
		VarDeps:      []string{"ui.gaiaPoolDefault", "accountmanager.username1", "accountmanager.password1"},
	})
}

func AddAccountFromOgb(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.username1")
	password := s.RequiredVar("accountmanager.password1")

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
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

	ui := uiauto.New(tconn).WithTimeout(accountmanager.DefaultUITimeout)
	if err := accountmanager.OpenOneGoogleBar(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to open OGB: ", err)
	}

	if err := clickAddAccount(ctx, ui); err != nil {
		s.Fatal("Failed to find add account link: ", err)
	}

	s.Log("Adding a secondary Account")
	if err := accountmanager.AddAccount(ctx, tconn, username, password); err != nil {
		s.Fatal("Failed to add a secondary Account: ", err)
	}

	// Open Account Manager page in OS Settings and find Add Google Account button.
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "accountManager", ui.Exists(nodewith.Name("Add Google Account").Role(role.Button))); err != nil {
		s.Fatal("Failed to launch Account Manager page: ", err)
	}

	// Find "More actions, <email>" button to make sure that account was added.
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	if err := ui.WaitUntilExists(moreActionsButton)(ctx); err != nil {
		s.Fatal("Failed to find More actions button: ", err)
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
