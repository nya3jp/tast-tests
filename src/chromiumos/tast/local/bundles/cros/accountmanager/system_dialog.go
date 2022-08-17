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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

type userParam struct {
	username string
	password string
	isSaml   bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SystemDialog,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "In-session account addition using system 'Add account' dialog",
		Contacts: []string{
			"anastasiian@chromium.org", // Test author.
			"team-dent@google.com",     // Account Manager owners.
			"cros-3pidp@google.com",    // Domain owners for SAML test.
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		VarDeps: []string{
			"accountmanager.username1",
			"accountmanager.password1",
			"accountmanager.managedusername",
			"accountmanager.managedpassword",
			"accountmanager.samlusername",
			"accountmanager.samlpassword",
		},
		Params: []testing.Param{{
			Val: userParam{
				isSaml:   false,
				username: "accountmanager.username1",
				password: "accountmanager.password1",
			},
		}, {
			Name: "managedchrome",
			Val: userParam{
				isSaml:   false,
				username: "accountmanager.managedusername",
				password: "accountmanager.managedpassword",
			},
		}, {
			Name: "saml",
			Val: userParam{
				isSaml:   true,
				username: "accountmanager.samlusername",
				password: "accountmanager.samlpassword",
			},
		}},
	})
}

func SystemDialog(ctx context.Context, s *testing.State) {
	param := s.Param().(userParam)
	username := s.RequiredVar(param.username)
	password := s.RequiredVar(param.password)

	cr := s.PreValue().(*chrome.Chrome)
	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	defer func(ctx context.Context) {
		s.Log("Running test cleanup")
		if err := accountmanager.TestCleanup(ctx, tconn, cr); err != nil {
			s.Fatal("Failed to do cleanup: ", err)
		}
	}(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "system_dialog")

	ui := uiauto.New(tconn).WithTimeout(accountmanager.DefaultUITimeout)

	// Open Account Manager page in OS Settings and click Add Google Account button.
	addAccountButton := nodewith.Name("Add Google Account").Role(role.Button)
	if err := uiauto.Combine("Click Add Google Account button",
		accountmanager.OpenAccountManagerSettingsAction(tconn, cr),
		ui.LeftClickUntil(addAccountButton, ui.Exists(accountmanager.AddAccountDialog())),
	)(ctx); err != nil {
		s.Fatal("Failed to click Add Google Account button: ", err)
	}

	s.Log("Adding a secondary Account")
	if param.isSaml {
		if err := accountmanager.AddAccountSAML(ctx, tconn, username, password); err != nil {
			s.Fatal("Failed to add a secondary SAML Account: ", err)
		}
	} else {
		if err := accountmanager.AddAccount(ctx, tconn, username, password); err != nil {
			s.Fatal("Failed to add a secondary Account: ", err)
		}
	}

	// Make sure that the settings page is focused again.
	if err := ui.WaitUntilExists(addAccountButton)(ctx); err != nil {
		s.Fatal("Failed to find Add Google Account button: ", err)
	}
	// Find "More actions, <email>" button to make sure that account was added.
	moreActionsButton := nodewith.Name("More actions, " + username).Role(role.Button)
	if err := ui.WithTimeout(accountmanager.LongUITimeout).WaitUntilExists(moreActionsButton)(ctx); err != nil {
		s.Fatal("Failed to find More actions button: ", err)
	}
}
