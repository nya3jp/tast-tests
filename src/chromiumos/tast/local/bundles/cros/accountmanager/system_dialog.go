// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package accountmanager provides functions to manage accounts in-session.
package accountmanager

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/accountmanager"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemDialog,
		Desc:         "In-session account addition using system 'Add account' dialog",
		Contacts:     []string{"anastasiian@chromium.org", "team-dent@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          accountmanager.ChromePreWithFeaturesEnabled(),
		VarDeps:      []string{"accountmanager.username1", "accountmanager.password1"},
	})
}

func SystemDialog(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.username1")
	password := s.RequiredVar("accountmanager.password1")

	cr := s.PreValue().(*chrome.Chrome)
	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	defer func(ctx context.Context) {
		s.Log("Running test cleanup")
		if err := accountmanager.TestCleanup(ctx, tconn, cr, browser.TypeAsh); err != nil {
			s.Fatal("Failed to do cleanup: ", err)
		}
	}(cleanupCtx)

	ui := uiauto.New(tconn).WithTimeout(accountmanager.DefaultUITimeout)

	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)

	if err := uiauto.Combine("Add a secondary Account",
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		ui.LeftClick(addAccountButton),
		accountmanager.AddAccountAction(tconn, username, password),
		// Make sure that the settings page is focused again.
		ui.WaitUntilExists(addAccountButton),
		// Find "More actions, <email>" button to make sure that account was added.
		ui.WaitUntilExists(moreActionsButton),
	)(ctx); err != nil {
		s.Fatal("Failed to add a secondary Account: ", err)
	}
}
