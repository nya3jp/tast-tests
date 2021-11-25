// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"os"
	"strings"
	"time"

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
		Timeout: 3 * time.Minute,
	})
}

const (
	deviceOwner     = "device-owner@gmail.com"
	additionalUser1 = "additional-user1@gmail.com"
	additionalUser2 = "additional-user2@gmail.com"
	commonPassword  = "password"
)

func RemoveUsersExceptOwner(ctx context.Context, s *testing.State) {
	setupUsers(ctx, s, additionalUser1, additionalUser2, commonPassword)

	tryDeleteUsersInSettings(ctx, s, additionalUser2, commonPassword, "")
	tryDeleteUsersInSettings(ctx, s, deviceOwner, commonPassword, additionalUser1)

	checkUserPodsOnSignInPage(ctx, s, additionalUser1, additionalUser2)
}

func setupUsers(ctx context.Context, s *testing.State, customUser1, customUser2, commonPassword string) {
	// create device owner with default credentials
	userutil.CreateUser(ctx, s, deviceOwner, commonPassword)

	// create two more user
	userutil.CreateUser(ctx, s, customUser1, commonPassword, chrome.KeepState())
	userutil.CreateUser(ctx, s, customUser2, commonPassword, chrome.KeepState())
}

func getUsernameFromEmail(email string) string {
	return email[:strings.IndexByte(email, '@')]
}

func tryDeleteUsersInSettings(ctx context.Context, s *testing.State, loginUser, password, userToRemove string) {
	cr := userutil.Login(ctx, s, loginUser, password)
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// display the list of users
	ui := uiauto.New(tconn)

	subsettingsName := "Manage other people"

	// open settings, Manage Other People
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "osPrivacy", ui.WaitUntilExists(nodewith.Name(subsettingsName)))
	if err != nil {
		s.Fatal("Failed to connect to the settings page: ", err)
	}
	defer settings.Close(ctx)

	if err := ui.LeftClick(nodewith.Name(subsettingsName))(ctx); err != nil {
		s.Fatal("Failed to open Manage other people subsettings: ", err)
	}

	optionName := "Restrict sign-in to the following users:"

	if err := ui.WaitUntilExists(nodewith.Name(optionName).Role(role.ToggleButton))(ctx); err != nil {
		s.Fatal("Failed to wait for the toggle to show the list of users: ", err)
	}

	if loginUser != deviceOwner {
		isEnabled, err := settings.IsToggleOptionEnabled(ctx, cr, optionName)
		if err != nil {
			s.Fatal("Could not check the status of the toggle: ", err)
		}
		if isEnabled {
			s.Fatal("The option should not be enabled for non-owners")
		}
	} else {
		if err := ui.LeftClick(nodewith.Name(optionName).Role(role.ToggleButton))(ctx); err != nil {
			s.Fatal("Failed to show the list of users: ", err)
		}

		// remove a non-owner user
		removeButtonName := "Remove " + getUsernameFromEmail(userToRemove)

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
		knownEmails := userutil.GetKnowEmailsFromLocalState(s)

		if knownEmails[userToRemove] {
			s.Fatal("Removed user is still in LoggedInUsers list")
		}
		if !knownEmails[deviceOwner] {
			s.Fatal("Device owner is not in LoggedInUsers list")
		}

		checkUsersCryptohome(ctx, s, userToRemove, false)
		checkUsersCryptohome(ctx, s, deviceOwner, true)
	}
}

func checkUserPodsOnSignInPage(ctx context.Context, s *testing.State, removedUser, availableUser string) {
	// go back to the login screen
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)

	// two pods should be available
	if err := ui.WaitUntilExists(nodewith.Name(deviceOwner).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pod to be available: ", err)
	}
	if err := ui.WaitUntilExists(nodewith.Name(availableUser).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pod to be available: ", err)
	}

	// there should be no pod for user that was removed earlier
	userPodFound, err := ui.IsNodeFound(ctx, nodewith.Name(removedUser).Role(role.Button))
	if err != nil {
		s.Fatal("Failed to lookup user pod: ", err)
	}
	if userPodFound {
		s.Fatal("Found remove button for a user that should not exist: ", err)
	}

}

func checkUsersCryptohome(ctx context.Context, s *testing.State, user string, shouldExist bool) {
	path, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		s.Fatalf("Cannot get path to %s's cryptohome: %v", user, err)
	}

	cryptohomeFileInfo, err := os.Stat(path)
	if shouldExist {
		if err != nil {
			s.Fatalf("Cryptohome directory for %s could not be accessed: %v", user, err)
		} else if cryptohomeFileInfo == nil {
			s.Fatalf("Cryptohome directory for %s does not exist under %s", user, path)
		}
	} else {
		if err == nil {
			s.Fatalf("Cryptohome directory for %s still exists under %s", user, path)
		} else if !os.IsNotExist(err) {
			s.Fatal("Unexpected error: ", err)
		}
	}
}
