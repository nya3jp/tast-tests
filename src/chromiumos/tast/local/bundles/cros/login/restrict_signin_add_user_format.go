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
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RestrictSigninAddUserFormat,
		Desc:         "Check that 'Manage other people' validates user email format",
		LacrosStatus: testing.LacrosVariantUnneeded,
		Contacts: []string{
			"anastasiian@chromium.org",
			"cros-lurs@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey", "ui.gaiaPoolDefault"},
		Timeout:      chrome.LoginTimeout + 3*time.Minute,
	})
}

var addUserDialog = nodewith.Role(role.Dialog)

func RestrictSigninAddUserFormat(ctx context.Context, s *testing.State) {
	const (
		deviceOwner    = "fake-device-owner@gmail.com"
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
		s.Fatal("Failed to open 'Manage other people' section in OS Settings: ", err)
	}
	defer cr.Close(cleanUpCtx)
	if settings != nil {
		defer settings.Close(cleanUpCtx)
	}

	// Set up keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("enable restricted sign-in",
		ui.LeftClick(nodewith.Name(signinutil.RestrictSignInOption).Role(role.ToggleButton)),
		ui.WaitUntilExists(nodewith.NameStartingWith(signinutil.GetUsernameFromEmail(deviceOwner)).NameContaining("owner").Role(role.StaticText)),
	)(ctx); err != nil {
		s.Fatal("Failed to enable restricted sign-in: ", err)
	}

	for _, tc := range []struct {
		email string
		// Expected email to be added. If unspecified, use "email" field.
		addedEmail string
		isValid    bool
	}{
		// Valid email addresses:
		{email: "test@gmail.com", isValid: true},
		{email: "1234567890@gmail.com", isValid: true},
		{email: "test987", addedEmail: "test987@gmail.com", isValid: true},
		{email: "firstname.lastname@gmail.com", addedEmail: "firstnamelastname@gmail.com", isValid: true},
		{email: "test.email.with+symbol@example.com", isValid: true},
		{email: "test@subdomain.example.com", isValid: true},
		{email: "email@example-one.com", isValid: true},
		{email: "_______@example.com", isValid: true},
		{email: "*@gmail.com", isValid: true},
		// Invalid email addresses:
		{email: "", isValid: false},
		{email: "test@gmail", isValid: false},
		{email: "test..123@gmail", isValid: false},
		{email: "@gmail.com", isValid: false},
	} {
		s.Run(ctx, tc.email, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+tc.email)

			if tc.addedEmail == "" {
				tc.addedEmail = tc.email
			}

			result, err := maybeAddUser(ctx, tconn, kb, tc.email, tc.addedEmail)
			if err != nil {
				s.Fatal("Failed to add user: ", err)
			}

			if result != tc.isValid {
				s.Errorf("Failed to confirm if email format is valid: want %t; got %t", tc.isValid, result)
			}
		})
	}
}

// maybeAddUser opens the Add user dialog, enters provided email and tries to
// add it to 'restricted users' list.
// If 'Add' button in the dialog is not active - returns false. This means that
// the provided email was invalid.
// Otherwise - successfully closes the dialog and checks that the user with
// provided addedEmail was added to the 'restricted users' list.
func maybeAddUser(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter,
	email, addedEmail string) (bool, error) {

	addUserButton := nodewith.Name("Add user").Role(role.Link)
	addUserEmailField := nodewith.Role(role.TextField).Name("Email address")
	addButton := nodewith.Name("Add").Focusable()

	ui := uiauto.New(tconn)

	if err := uiauto.Combine("start adding user",
		ui.WaitUntilExists(addUserButton),
		ui.LeftClick(addUserButton),
		ui.WaitUntilExists(addUserDialog),
		ui.WaitUntilExists(addUserEmailField),
		ui.LeftClick(addUserEmailField),
		kb.TypeAction(email),
		ui.WaitUntilExists(addButton),
	)(ctx); err != nil {
		return false, err
	}

	addButtonInfo, err := ui.Info(ctx, addButton)
	if err != nil {
		return false, errors.Wrap(err, "failed to find info for addButton")
	}

	if addButtonInfo.Restriction == restriction.Disabled {
		if err := cancelDialog(ctx, ui); err != nil {
			return false, err
		}
		return false, nil
	}

	// Try to submit with 'Add' button.
	if err := ui.WithTimeout(5*time.Second).LeftClickUntil(addButton, ui.WaitUntilGone(addUserDialog))(ctx); err != nil {
		// 'Add' button doesn't work -> cancel the dialog and return false.
		if err1 := cancelDialog(ctx, ui); err1 != nil {
			return false, err1
		}
		return false, err
	}

	if err := ui.WaitUntilExists(nodewith.Name(addedEmail).Role(role.StaticText))(ctx); err != nil {
		return false, errors.Wrap(err, "failed to make sure the user was added")
	}

	return true, nil
}

func cancelDialog(ctx context.Context, ui *uiauto.Context) error {
	cancelButton := nodewith.Name("Cancel").Focusable()

	if err := uiauto.Combine("cancel the dialog",
		ui.WaitUntilExists(cancelButton),
		ui.LeftClick(cancelButton),
		ui.WaitUntilGone(addUserDialog),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to cancel the dialog")
	}

	return nil
}
