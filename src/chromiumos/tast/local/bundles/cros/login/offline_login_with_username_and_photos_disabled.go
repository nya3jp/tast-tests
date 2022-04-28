// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/login/signinutil"
	"chromiumos/tast/local/bundles/cros/login/userutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OfflineLoginWithUsernameAndPhotosDisabled,
		Desc:         "Checks that a user can login again if they have already signed in even though the network is offline and the device owner disabled the username and photos",
		LacrosStatus: testing.LacrosVariantUnneeded,
		Contacts:     []string{"bchikhaoui@google.com", "cros-oac@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: 3*chrome.LoginTimeout + 45*time.Second,
	})
}

func OfflineLoginWithUsernameAndPhotosDisabled(ctx context.Context, s *testing.State) {

	creds := []chrome.Creds{
		{User: "deviceOwner@gmail.com", Pass: "test pass 1"},
		{User: "additionalUser1@gmail.com", Pass: "test pass 2"},
	}

	cleanUpCtx := ctx
	// 10 seconds should be enough for all clean up operations.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	setupOwnerAndUsersAndPresetting(ctx, cleanUpCtx, s, creds)

	loginOffline := func(ctx context.Context) error {
		cr, err := chrome.New(ctx,
			chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
			chrome.NoLogin(),
			chrome.KeepState(),
			chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))

		if err != nil {
			return errors.Wrap(err, "chrome start failed")
		}
		defer cr.Close(ctx)

		oobeConn, err := cr.WaitForOOBEConnection(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create OOBE connection")
		}
		defer oobeConn.Close()

		tconn, err := cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "creating login test API connection failed")
		}
		defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tconn)

		const uiTimeout = 10 * time.Second

		ui := uiauto.New(tconn)

		clickSignInAsExistingUserLink(ctx, s, oobeConn)

		if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.OfflineLoginScreen.isReadyForTesting()"); err != nil {
			s.Fatal("Failed to wait for the offline login screen to be visible: ", err)
		}

		var emailFieldName string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.OfflineLoginScreen.getEmailFieldName()", &emailFieldName); err != nil {
			s.Fatal("Failed to retrieve the email field name: ", err)
		}

		var passwordFieldName string
		if err := oobeConn.Eval(ctx, "OobeAPI.screens.OfflineLoginScreen.getPasswordFieldName()", &passwordFieldName); err != nil {
			s.Fatal("Failed to retrieve the password field name: ", err)
		}

		kb, err := input.VirtualKeyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get virtual keyboard")
		}
		defer kb.Close()

		fillTextField(ctx, s, ui, kb, emailFieldName, creds[1].User)

		clickNextButton(ctx, s, ui, oobeConn)

		fillTextField(ctx, s, ui, kb, passwordFieldName, creds[1].Pass)

		clickNextButton(ctx, s, ui, oobeConn)

		if err := lockscreen.WaitForLoggedIn(ctx, tconn, chrome.LoginTimeout); err != nil {
			s.Fatal("Failed to login: ", err)
		}

		return nil
	}

	if err := network.ExecFuncOnChromeOffline(ctx, loginOffline); err != nil {
		s.Fatal("Failed to login offline: ", err)
	}
}

func clickSignInAsExistingUserLink(ctx context.Context, s *testing.State, oobeConn *chrome.Conn) {

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.ErrorScreen.isReadyForTesting()"); err != nil {
		s.Fatal("Failed to wait for the error screen to be visible : ", err)
	}

	if err := oobeConn.Eval(ctx, "OobeAPI.screens.ErrorScreen.clickSignInAsExistingUserLink()", nil); err != nil {
		s.Fatal("Failed to click on sign in as existing user link : ", err)
	}

}

func fillTextField(ctx context.Context, s *testing.State, ui *uiauto.Context, kb *input.KeyboardEventWriter, nodeName, nodeValue string) {

	textfield := nodewith.Name(nodeName).Role(role.TextField)

	if err := uiauto.Combine("Fill the text field",
		ui.WaitUntilExists(textfield),
		ui.LeftClick(textfield),
		ui.WaitUntilExists(textfield.Focused()),
	)(ctx); err != nil {
		s.Fatal("Failed to select the text field : ", err)
	}

	if err := kb.Type(ctx, nodeValue); err != nil {
		s.Fatal("Failed to type the text field : ", err)
	}

}

func clickNextButton(ctx context.Context, s *testing.State, ui *uiauto.Context, oobeConn *chrome.Conn) {

	var nextbuttonName string
	if err := oobeConn.Eval(ctx, "OobeAPI.screens.OfflineLoginScreen.getNextButtonName()", &nextbuttonName); err != nil {
		s.Fatal("Failed to retrieve the next button name: ", err)
	}

	if err := uiauto.Combine("Click on next button",
		ui.WaitUntilExists(nodewith.Name(nextbuttonName).Role(role.Button)),
		ui.LeftClick(nodewith.Name(nextbuttonName).Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to click on next button: ", err)
	}
}

func setupOwnerAndUsersAndPresetting(ctx, cleanUpCtxs context.Context, s *testing.State, creds []chrome.Creds) {

	chrome.New(ctx,
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))

	// Login with the Device Owner
	cr, err := userutil.Login(ctx, creds[0].User, creds[0].Pass)
	if err != nil {
		s.Fatal("Failed to log in as device owner: ", err)
	}

	// For the device owner we wait until their ownership has been established.
	userutil.WaitForOwnership(ctx, cr)

	// Setup the Setting Options
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanUpCtxs, s.OutDir(), s.HasError, tconn)

	settings, err := signinutil.OpenManageOtherPeople(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to open Manage other people: ", err)
	}
	defer cr.Close(cleanUpCtxs)
	if settings != nil {
		defer settings.Close(cleanUpCtxs)
	}
	ui := uiauto.New(tconn)

	if err := ui.LeftClick(nodewith.Name("Show usernames and photos on the sign-in screen").Role(role.ToggleButton))(ctx); err != nil {
		s.Fatal("Failed to click on the show usernames and photos toglle: ", err)
	}

	// Create another user
	if err := userutil.CreateUser(ctx, creds[1].User, creds[1].Pass, chrome.KeepState()); err != nil {
		s.Fatal("Failed to create new user: ", err)
	}

}
