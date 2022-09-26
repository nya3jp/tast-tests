// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/login/userutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SignInWithLotsOfUsers,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test user pods are all visible in the login screen and each user can log in accordingly",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"jason.hsiao@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Params: []testing.Param{
			{
				Name:    "10_users",
				Val:     10,
				Timeout: 5 * time.Minute,
			},
			{
				Name:    "20_users",
				Val:     20,
				Timeout: 10 * time.Minute,
			},
		},
	})
}

// SignInWithLotsOfUsers tests user pods are all visible in the login screen and each user can log in accordingly.
func SignInWithLotsOfUsers(ctx context.Context, s *testing.State) {
	deviceOwner := chrome.Creds{User: "test_owner@gmail.com", Pass: "test0000"}
	if err := userutil.CreateDeviceOwner(ctx, deviceOwner.User, deviceOwner.Pass); err != nil {
		s.Fatal("Failed to create device owner: ", err)
	}

	userCount := s.Param().(int)
	testCreds := []chrome.Creds{deviceOwner}
	for i := 0; i < userCount; i++ {
		userCreds := chrome.Creds{User: fmt.Sprintf("test_user%d@gmail.com", i), Pass: "test0000"}
		s.Log("Creating new user pod: ", userCreds.User)
		if err := userutil.CreateUser(ctx, userCreds.User, userCreds.Pass, chrome.KeepState()); err != nil {
			s.Fatal("Failed to create new user: ", err)
		}
		testCreds = append(testCreds, userCreds)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, creds := range testCreds {
		func() {
			cr, err := chrome.New(
				ctx,
				chrome.NoLogin(),
				chrome.KeepState(),
				chrome.SkipForceOnlineSignInForTesting(),
				chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
			)
			if err != nil {
				s.Fatal("Failed to start Chrome: ", err)
			}
			defer cr.Close(cleanupCtx)

			tconn, err := cr.SigninProfileTestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to connect to test API: ", err)
			}
			defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

			if err := signinAndVerify(ctx, tconn, kb, creds); err != nil {
				s.Fatal("Failed to verify sign-in function: ", err)
			}
		}()
	}
}

// signinAndVerify ensures the user is visible on the sign-in screen and verify the user can sign in with password.
func signinAndVerify(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, creds chrome.Creds) error {
	ui := uiauto.New(tconn)
	loginWindow := nodewith.Name("Login Screen").Role(role.Window)
	userButton := nodewith.Name(creds.User).Role(role.Button).Ancestor(loginWindow)
	if err := uiauto.NamedCombine(fmt.Sprintf("ensure user %q is visible", creds.User),
		ui.WaitUntilExists(userButton),
		ui.MakeVisible(userButton),
	)(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Start signing in to user ", creds.User)
	if err := ui.DoDefault(userButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to focus on the user")
	}
	if err := lockscreen.EnterPassword(ctx, tconn, creds.User, creds.Pass, kb); err != nil {
		return errors.Wrap(err, "failed to enter password")
	}
	if err := lockscreen.WaitForLoggedIn(ctx, tconn, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for logged in")
	}

	return nil
}
