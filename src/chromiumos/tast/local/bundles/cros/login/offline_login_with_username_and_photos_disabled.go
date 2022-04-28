// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/coords"
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
	// 30 seconds should be enough for all clean up operations.
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	setupOwnerAndUsers(ctx, cleanUpCtx, s, creds)

	// Login as owner and disable username and photo
	setupPresetting(ctx, cleanUpCtx, s, creds)

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

		tconn, err := cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			s.Fatal("Creating login test API connection failed: ", err)
		}
		defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tconn)

		const uiTimeout = 10 * time.Second

		ui := uiauto.New(tconn)

		clickSignInAsExistingUserLink(ctx, s, ui)

		kb, err := input.VirtualKeyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get virtual keyboard")
		}
		defer kb.Close()

		fillTextField(ctx, s, ui, kb, "Enter your email", creds[1].User)

		clickNextButton(ctx, s, ui)

		fillTextField(ctx, s, ui, kb, "Enter your password", creds[1].Pass)

		clickNextButton(ctx, s, ui)

		if err := lockscreen.WaitForLoggedIn(ctx, tconn, chrome.LoginTimeout); err != nil {
			s.Fatal("Failed to login: ", err)
		}

		return nil
	}

	if err := network.ExecFuncOnChromeOffline(ctx, loginOffline); err != nil {
		s.Fatal("Failed to login offline: ", err)
	}
}

func clickSignInAsExistingUserLink(ctx context.Context, s *testing.State, ui *uiauto.Context) {
	linkField := nodewith.Name("sign in as an existing user").Role(role.Link)

	if err := ui.WaitUntilExists(linkField)(ctx); err != nil {
		s.Fatal("Failed to wait for the link field : ", err)
	}

	loc, err := ui.Location(ctx, linkField)

	if err != nil {
		s.Fatal("Failed to Locate the link field : ", err)
	}

	clickPoint := coords.Point{
		X: loc.TopRight().X - 1,
		Y: loc.TopRight().Y + 1,
	}

	ui.MouseClickAtLocation(0, clickPoint)(ctx)
}

func fillTextField(ctx context.Context, s *testing.State, ui *uiauto.Context, kb *input.KeyboardEventWriter, nodeName, nodeValue string) {

	textfield := nodewith.Name(nodeName).Role(role.TextField)

	if err := uiauto.Combine("Fill the text field",
		ui.WaitUntilExists(textfield),
		ui.LeftClick(textfield),
		ui.WaitUntilExists(textfield.Focused()),
	)(ctx); err != nil {
		s.Fatal("Failed to select the text field: ", err)
	}

	if err := kb.Type(ctx, nodeValue); err != nil {
		s.Fatal("Failed to type the text field : ", err)
	}

}

func clickNextButton(ctx context.Context, s *testing.State, ui *uiauto.Context) {
	const buttonName = "Next"

	if err := uiauto.Combine("Click on next button",
		ui.WaitUntilExists(nodewith.Name(buttonName).Role(role.Button)),
		ui.LeftClick(nodewith.Name(buttonName).Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to click on next button: ", err)
	}
}

func setupPresetting(ctx, cleanUpCtxs context.Context, s *testing.State, creds []chrome.Creds) {
	// Login with device owner and disable username and photos.
	cr, err := userutil.Login(ctx, creds[0].User, creds[0].Pass)
	if err != nil {
		s.Fatal("Failed to log in as device owner: ", err)
	}

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
}

func setupOwnerAndUsers(ctx, cleanUpCtxs context.Context, s *testing.State, creds []chrome.Creds) {
	// create Device Owner
	if err := userutil.CreateDeviceOwner(ctx, creds[0].User, creds[0].Pass); err != nil {
		s.Fatal("Failed to create device owner: ", err)
	}

	// For other users we don't need to wait for anything.
	if err := userutil.CreateUser(ctx, creds[1].User, creds[1].Pass, chrome.KeepState()); err != nil {
		s.Fatal("Failed to create new user: ", err)
	}

}
