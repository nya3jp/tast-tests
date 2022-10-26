// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"os"
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
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoveUsersExceptOwner,
		Desc:         "Checks if device owner can remove other users, but not self (on the Settings page)",
		LacrosStatus: testing.LacrosVariantUnknown,
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
	// 30 seconds should be enough for all clean up operations.
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	if err := setupUsers(ctx, cleanUpCtx); err != nil {
		s.Fatal("Failed to setup users: ", err)
	}

	// Non-owner should not be able to remove users.
	func() {
		cr, err := userutil.Login(ctx, additionalUser2, commonPassword)
		if err != nil {
			s.Fatal("Failed to log in as non-owner user: ", err)
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

		isEnabled, err := settings.IsToggleOptionEnabled(ctx, cr, signinutil.RestrictSignInOption)
		if err != nil {
			s.Fatal("Could not check the status of the toggle: ", err)
		}
		if isEnabled {
			s.Fatal("The option should not be enabled for non-owners")
		}
	}()

	// Device owner should be able to delete other users, but not self.
	func() {
		cr, err := userutil.Login(ctx, deviceOwner, commonPassword)
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

		if err := ui.LeftClick(nodewith.Name(signinutil.RestrictSignInOption).Role(role.ToggleButton))(ctx); err != nil {
			s.Fatal("Failed to show the list of users: ", err)
		}

		// Verify that there are three User accounts shown in the Users list.
		// Verify that only the first user is designated as the "Owner".
		if err := uiauto.Combine("verify users list",
			ui.WaitUntilExists(nodewith.NameStartingWith(signinutil.GetUsernameFromEmail(deviceOwner)).NameContaining("owner").Role(role.StaticText)),
			ui.WaitUntilExists(nodewith.Name(signinutil.GetUsernameFromEmail(additionalUser1)).Role(role.StaticText)),
			ui.WaitUntilExists(nodewith.Name(signinutil.GetUsernameFromEmail(additionalUser2)).Role(role.StaticText)),
		)(ctx); err != nil {
			s.Fatal("Failed to verify users list: ", err)
		}

		// Cryptohomes of all users should be available.
		for _, user := range []string{deviceOwner, additionalUser1, additionalUser2} {
			if err := checkCryptohomeFileInfo(ctx, user); err != nil {
				s.Fatalf("Failed to check cryptohome info for user %q: %v", user, err)
			}
		}

		// Remove a non-owner user.
		removeButtonName := "Remove " + signinutil.GetUsernameFromEmail(additionalUser1)

		if err := uiauto.Combine("remove a non-owner user",
			ui.WaitUntilExists(nodewith.Name(removeButtonName).Role(role.Button)),
			ui.LeftClick(nodewith.Name(removeButtonName).Role(role.Button)),
			ui.WaitUntilGone(nodewith.Name(removeButtonName).Role(role.Button)),
		)(ctx); err != nil {
			s.Fatal("Deletion failed: ", err)
		}

		// It shouldn't be possible to remove the owner.
		if err := ui.Gone(nodewith.Name("Remove " + signinutil.GetUsernameFromEmail(deviceOwner)).Role(role.Button))(ctx); err != nil {
			s.Fatal("Didn't expect to find remove button for device owner: ", err)
		}

		// Check if the user has been removed properly, and that the device owher is still there.
		knownEmails, err := userutil.GetKnownEmailsFromLocalState()
		if err != nil {
			s.Fatal("Failed to get known emails from local state: ", err)
		}

		if knownEmails[additionalUser1] {
			s.Fatal("Removed user is still in LoggedInUsers list")
		}
		if !knownEmails[deviceOwner] {
			s.Fatal("Device owner is not in LoggedInUsers list")
		}

		// Cryptohome of a deleted user should not exist.
		if _, err := getCryptohomeFileInfo(ctx, additionalUser1); err == nil {
			s.Fatalf("Cryptohome directory for %s still exists", additionalUser1)
		} else if !os.IsNotExist(err) {
			s.Fatal("Unexpected error: ", err)
		}

		// Cryptohome of the device owner should still be available.
		if err := checkCryptohomeFileInfo(ctx, deviceOwner); err != nil {
			s.Fatal("Failed to check cryptohome info for device owner: ", err)
		}
	}()

	// Go back to the login screen and check user pods.
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

	// Pods of device owner and one of the other users should be available.
	if err := ui.WaitUntilExists(nodewith.Name(deviceOwner).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pod to be available: ", err)
	}
	if err := ui.WaitUntilExists(nodewith.Name(additionalUser2).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pod to be available: ", err)
	}

	// There should be no pod for the user that was removed earlier.
	userPodFound, err := ui.IsNodeFound(ctx, nodewith.Name(additionalUser1).Role(role.Button))
	if err != nil {
		s.Fatal("Failed to lookup user pod: ", err)
	}
	if userPodFound {
		s.Fatal("Found remove button for a user that should not exist: ", err)
	}
}

func setupUsers(ctx, cleanUpCtxs context.Context) error {
	// For the device owner we wait until their ownership has been established.
	if err := userutil.CreateDeviceOwner(ctx, deviceOwner, commonPassword); err != nil {
		return errors.Wrap(err, "failed to create device owner")
	}

	// For other users we don't need to wait for anything.
	if err := userutil.CreateUser(ctx, additionalUser1, commonPassword, chrome.KeepState()); err != nil {
		return errors.Wrap(err, "failed to create new user")
	}
	if err := userutil.CreateUser(ctx, additionalUser2, commonPassword, chrome.KeepState()); err != nil {
		return errors.Wrap(err, "failed to create new user")
	}
	return nil
}

func getCryptohomeFileInfo(ctx context.Context, user string) (os.FileInfo, error) {
	path, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get path to %s's cryptohome", user)
	}

	return os.Stat(path)
}

// checkCryptohomeFileInfo checks that cryptohome of the specified user is available.
func checkCryptohomeFileInfo(ctx context.Context, user string) error {
	if cryptohomeFileInfo, err := getCryptohomeFileInfo(ctx, user); err != nil {
		return errors.Wrapf(err, "cryptohome directory for %s could not be accessed", deviceOwner)
	} else if cryptohomeFileInfo == nil {
		return errors.Errorf("Cryptohome directory for %s does not exist", deviceOwner)
	}
	return nil
}
