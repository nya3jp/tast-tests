// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/login/userutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoveUsersExceptOwner,
		Desc:         "Checks if device owner can remove other users, but not self (on the Settings page)",
		Contacts:     []string{"jaflis@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		// the test performs 5 log-ins and some additional operations. we also reserve some time for clean-up
		Timeout: 5*chrome.LoginTimeout + 45*time.Second,
	})
}

const (
	deviceOwner     = "device-owner@gmail.com"
	additionalUser1 = "additional-user1@gmail.com"
	additionalUser2 = "additional-user2@gmail.com"
	commonPassword  = "password"
)

func RemoveUsersExceptOwner(ctx context.Context, s *testing.State) {
	cleanUpCtx := ctx
	// 30 seconds should be enough for all clean up operations
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	setupUsers(ctx, cleanUpCtx, s)

	const restrictSignInOption = "Restrict sign-in to the following users:"

	// non-owner should not be able to remove users
	func() {
		cr := userutil.Login(ctx, s, additionalUser2, commonPassword)
		settings, _ := openSettingsInSession(ctx, cleanUpCtx, s, cr, restrictSignInOption)
		defer cr.Close(cleanUpCtx)
		defer settings.Close(cleanUpCtx)

		isEnabled, err := settings.IsToggleOptionEnabled(ctx, cr, restrictSignInOption)
		if err != nil {
			s.Fatal("Could not check the status of the toggle: ", err)
		}
		if isEnabled {
			s.Fatal("The option should not be enabled for non-owners")
		}
	}()

	// device owner should be able to delete other users, but not self
	func() {
		cr := userutil.Login(ctx, s, deviceOwner, commonPassword)

		userutil.WaitForOwnership(ctx, s, cr)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Creating login test API connection failed: ", err)
		}
		defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tconn)

		settings, ui := openSettingsInSession(ctx, cleanUpCtx, s, cr, restrictSignInOption)
		defer cr.Close(cleanUpCtx)
		defer settings.Close(cleanUpCtx)

		if err := ui.LeftClick(nodewith.Name(restrictSignInOption).Role(role.ToggleButton))(ctx); err != nil {
			s.Fatal("Failed to show the list of users: ", err)
		}

		// remove a non-owner user
		removeButtonName := "Remove " + getUsernameFromEmail(additionalUser1)

		if err := uiauto.Combine("remove a non-owner user",
			ui.WaitUntilExists(nodewith.Name(removeButtonName).Role(role.Button)),
			ui.LeftClick(nodewith.Name(removeButtonName).Role(role.Button)),
			ui.WaitUntilGone(nodewith.Name(removeButtonName).Role(role.Button)),
		)(ctx); err != nil {
			s.Fatal("Deletion failed: ", err)
		}

		// it shouldn't be possible to remove the owner
		if err := ui.Gone(nodewith.Name("Remove " + getUsernameFromEmail(deviceOwner)).Role(role.Button))(ctx); err != nil {
			s.Fatal("Didn't expect to find remove button for device owner: ", err)
		}

		// check if the user has been removed properly, and that the device owher is still there
		knownEmails := userutil.GetKnownEmailsFromLocalState(s)

		if knownEmails[additionalUser1] {
			s.Fatal("Removed user is still in LoggedInUsers list")
		}
		if !knownEmails[deviceOwner] {
			s.Fatal("Device owner is not in LoggedInUsers list")
		}

		// cryptohome of a deleted user should not exist
		cryptohomeFileInfo, err := getCryptohomeFileInfo(ctx, s, additionalUser1)
		if err == nil {
			s.Fatalf("Cryptohome directory for %s still exists", additionalUser1)
		} else if !os.IsNotExist(err) {
			s.Fatal("Unexpected error: ", err)
		}

		// cryptohome of the device owher should be available
		cryptohomeFileInfo, err = getCryptohomeFileInfo(ctx, s, deviceOwner)
		if err != nil {
			s.Fatalf("Cryptohome directory for %s could not be accessed: %v", deviceOwner, err)
		} else if cryptohomeFileInfo == nil {
			s.Fatalf("Cryptohome directory for %s does not exist", deviceOwner)
		}
	}()

	// go back to the login screen and check user pods
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

	ui := uiauto.New(tconn)

	// pods of device owner and one of the other users should be available
	if err := ui.WaitUntilExists(nodewith.Name(deviceOwner).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pod to be available: ", err)
	}
	if err := ui.WaitUntilExists(nodewith.Name(additionalUser2).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pod to be available: ", err)
	}

	// there should be no pod for the user that was removed earlier
	userPodFound, err := ui.IsNodeFound(ctx, nodewith.Name(additionalUser1).Role(role.Button))
	if err != nil {
		s.Fatal("Failed to lookup user pod: ", err)
	}
	if userPodFound {
		s.Fatal("Found remove button for a user that should not exist: ", err)
	}
}

func setupUsers(ctx, cleanUpCtxs context.Context, s *testing.State) {
	// for the device owner we wait until their ownership has been established
	userutil.CreateDeviceOwner(ctx, cleanUpCtxs, s, deviceOwner, commonPassword)

	// for other users we don't need to wait for anything
	userutil.CreateUser(ctx, cleanUpCtxs, s, additionalUser1, commonPassword, chrome.KeepState())
	userutil.CreateUser(ctx, cleanUpCtxs, s, additionalUser2, commonPassword, chrome.KeepState())
}

func getUsernameFromEmail(email string) string {
	return email[:strings.IndexByte(email, '@')]
}

func openSettingsInSession(ctx, cleanUpCtx context.Context, s *testing.State, cr *chrome.Chrome, restrictSignInOption string) (*ossettings.OSSettings, *uiauto.Context) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tconn)

	// display the list of users
	ui := uiauto.New(tconn)

	const subsettingsName = "Manage other people"

	// open settings, Manage Other People
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osPrivacy", ui.WaitUntilExists(nodewith.Name(subsettingsName)))
	if err != nil {
		s.Fatal("Failed to connect to the settings page: ", err)
	}

	if err := ui.LeftClick(nodewith.Name(subsettingsName))(ctx); err != nil {
		s.Fatal("Failed to open Manage other people subsettings: ", err)
	}

	if err := ui.WaitUntilExists(nodewith.Name(restrictSignInOption).Role(role.ToggleButton))(ctx); err != nil {
		s.Fatal("Failed to wait for the toggle to show the list of users: ", err)
	}

	return settings, ui
}

func getCryptohomeFileInfo(ctx context.Context, s *testing.State, user string) (os.FileInfo, error) {
	path, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		s.Fatalf("Cannot get path to %s's cryptohome: %v", user, err)
	}

	return os.Stat(path)
}
