// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/login/userutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RemoveUserOnSigninScreen,
		Desc: "Checks if users can be removed on the sign in screen",
		Contacts: []string{
			"mbid@google.com",
			"cros-lurs@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		// This test
		// * creates three users,
		// * launches chrome twice,
		// * does some fast ui operations.
		// We also need some time for cleanup.
		Timeout:      5*chrome.LoginTimeout + time.Minute + 30*time.Second,
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

func RemoveUserOnSigninScreen(ctx context.Context, s *testing.State) {
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	signinVar := s.RequiredVar("ui.signinProfileTestExtensionManifestKey")

	// Setup three users with the same password.
	firstUser := "first-user@gmail.com"
	secondUser := "second-user@gmail.com"
	thirdUser := "third-user@gmail.com"
	password := "password"
	for _, user := range []string{firstUser, secondUser, thirdUser} {
		if err := userutil.CreateUser(ctx, user, password, chrome.KeepState()); err != nil {
			s.Fatal("Failed to create user: ", err)
		}
	}

	// Go to the login screen, remove second user and check that second user pod is gone.
	func() {
		// We need NoLogin() to get on the login screen, and we need
		// --skip-force-online-signin-for-testing so that we're not asked to signin in when focussing a
		// user pod.
		cr, err := chrome.New(
			ctx,
			chrome.NoLogin(),
			chrome.KeepState(),
			chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
			chrome.LoadSigninProfileExtension(signinVar),
		)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(cleanUpCtx)

		tconn, err := cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create Test API connection: ", err)
		}
		defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tconn)
		ui := uiauto.New(tconn)

		// Focus second user pod.
		if err := ui.LeftClick(nodewith.Name(secondUser).Role(role.Button))(ctx); err != nil {
			s.Fatal("Failed to focus user pod: ", err)
		}

		// Open the remove dialog.
		rdName := "Open remove dialog for " + secondUser
		if err := ui.LeftClick(nodewith.Name(rdName).Role(role.Button))(ctx); err != nil {
			s.Fatal("Failed to open remove dialog: ", err)
		}

		// Check that we can find reference to second user before we remove the second user.
		if err := ui.WaitUntilExists(nodewith.Name(secondUser).First())(ctx); err != nil {
			s.Fatal("Could not find second user pod after focusing it: ", err)
		}

		// Click on remove button for the first time.
		removeButton := nodewith.Name("Remove account").Role(role.Button)
		if err := ui.LeftClick(removeButton)(ctx); err != nil {
			s.Fatal("Failed to click on \"Remove account\" button: ", err)
		}

		// Click on remove button again to confirm.
		if err := ui.LeftClick(removeButton)(ctx); err != nil {
			s.Fatal("Failed to click on \"Remove account\" button for confirmation: ", err)
		}

		// Check that second user is gone.
		if err := ui.WaitUntilGone(nodewith.Name(secondUser))(ctx); err != nil {
			s.Fatal("Second user pod has not disappeared: ", err)
		}
	}()

	// Restart chrome and check that the user we just removed is still gone.
	func() {
		cr, err := chrome.New(
			ctx,
			chrome.NoLogin(),
			chrome.KeepState(),
			chrome.LoadSigninProfileExtension(signinVar),
		)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(cleanUpCtx)

		tconn, err := cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create Test API connection: ", err)
		}
		defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tconn)
		ui := uiauto.New(tconn)

		// Wait until we find reference to firstUser.
		if err := ui.WaitUntilExists(nodewith.Name(firstUser).First())(ctx); err != nil {
			s.Fatal("Failed to wait for user pod to be available after restart: ", err)
		}

		// Check that we cannot find reference to second user. We use IsNodeFound instead of Gone or
		// WaitUntilGone so that we wait a while before we conclude that there's no reference.
		secondUserFound, err := ui.IsNodeFound(ctx, nodewith.Name(secondUser))
		if err != nil {
			s.Fatal("Failed to lookup user pod: ", err)
		}
		if secondUserFound {
			s.Fatal("Removed user reappeared after restart")
		}
	}()
}
