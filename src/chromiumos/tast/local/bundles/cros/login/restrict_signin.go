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
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RestrictSignin,
		Desc:         "Checks if device owner can restrict signin",
		LacrosStatus: testing.LacrosVariantUnneeded,
		Contacts: []string{
			"anastasiian@chromium.org",
			"cros-lurs@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      chrome.LoginTimeout + time.Minute,
	})
}

func RestrictSignin(ctx context.Context, s *testing.State) {
	const (
		deviceOwner    = "device-owner@gmail.com"
		devicePassword = "password"
	)

	cleanUpCtx := ctx
	// 30 seconds should be enough for all clean up operations.
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	// For the device owner we wait until their ownership has been established.
	if err := userutil.CreateDeviceOwner(ctx, deviceOwner, devicePassword); err != nil {
		s.Fatal("Failed to create device owner: ", err)
	}

	// Check that 'Add person' is enabled by default.
	func() {
		cr, err := chrome.New(
			ctx,
			chrome.NoLogin(),
			chrome.KeepState(),
			chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(cleanUpCtx)

		tconn, err := cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			s.Fatal("Creating login test API connection failed: ", err)
		}
		defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tconn)

		// Make sure the button is enabled.
		addPersonEnabled, err := getAddPersonFocusableState(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to find Add person button info: ", err)
		}
		if !addPersonEnabled {
			s.Fatal("Failed to make sure Add person button is enabled")
		}
	}()

	// Select 'Restrict sign-in' in Settings.
	func() {
		cr, err := userutil.Login(ctx, deviceOwner, devicePassword)
		if err != nil {
			s.Fatal("Failed to log in as device owner: ", err)
		}
		if err := userutil.WaitForOwnership(ctx, cr); err != nil {
			s.Fatal("User did not become device owner: ", err)
		}

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Creating login test API connection failed: ", err)
		}
		defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tconn)

		settings, err := signinutil.OpenManageOtherPeople(ctx, cr, tconn)
		if err != nil {
			s.Fatal("Failed to open Manage other people: ", err)
		}
		defer cr.Close(cleanUpCtx)
		if settings != nil {
			defer settings.Close(cleanUpCtx)
		}

		ui := uiauto.New(tconn)

		if err := uiauto.Combine("restrict sign-in to existing users only",
			ui.LeftClick(nodewith.Name(signinutil.RestrictSignInOption).Role(role.ToggleButton)),
			ui.WaitUntilExists(nodewith.NameStartingWith(signinutil.GetUsernameFromEmail(deviceOwner)).NameContaining("owner").Role(role.StaticText)),
		)(ctx); err != nil {
			s.Fatal("Failed to restrict sign-in: ", err)
		}
	}()

	// Check that 'Add person' is disabled.
	func() {
		cr, err := chrome.New(
			ctx,
			chrome.NoLogin(),
			chrome.KeepState(),
			chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(cleanUpCtx)

		tconn, err := cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			s.Fatal("Creating login test API connection failed: ", err)
		}
		defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tconn)

		addPersonEnabled, err := getAddPersonFocusableState(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to find Add person button info: ", err)
		}
		if addPersonEnabled {
			s.Fatal("Failed to make sure Add person button is disabled")
		}
	}()
}

// getAddPersonFocusableState finds "Add person" button and returns whether it's focusable/enabled.
func getAddPersonFocusableState(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	ui := uiauto.New(tconn)

	// Find 'Add person' button. Make sure the button is enabled.
	addPersonButton := nodewith.Name("Add Person").Role(role.Button).HasClass("LoginShelfButton")
	addPersonInfo, err := ui.Info(ctx, addPersonButton)
	if err != nil {
		return false, errors.Wrap(err, "failed to find Add person button info")
	}
	return addPersonInfo.State[state.Focusable], nil
}
