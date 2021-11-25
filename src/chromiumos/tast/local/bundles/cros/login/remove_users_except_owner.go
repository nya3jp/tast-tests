// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"time"

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
		Desc:         "Checks if device owner can remove other users, but not self",
		Contacts:     []string{"jaflis@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: 3 * time.Minute,
	})
}

func RemoveUsersExceptOwner(ctx context.Context, s *testing.State) {
	const (
		defaultUser     = "testuser@gmail.com"
		customUser1     = "custom-user1@gmail.com"
		customUser2     = "custom-user2@gmail.com"
		defaultPassword = "testpass"
		customPassword  = "password"
	)

	setupUsers(ctx, s, customUser1, customUser2, customPassword)

	manageUsersInSettings(ctx, s, defaultUser, defaultPassword, customUser1)

	manageUsersOnSignInPage(ctx, s, defaultUser, customUser1, customUser2)
}

func createUser(ctx context.Context, s *testing.State, creds chrome.Creds, extraOpts ...chrome.Option) {
	opts := append([]chrome.Option{chrome.FakeLogin(creds)}, extraOpts...)
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	cr.Close(ctx)
}

func setupUsers(ctx context.Context, s *testing.State, customUser1, customUser2, commonPassword string) {
	// create device owner with default credentials
	createUser(ctx, s, chrome.Creds{User: chrome.DefaultUser, Pass: chrome.DefaultPass})

	// create two more user
	createUser(ctx, s, chrome.Creds{User: customUser1, Pass: commonPassword}, chrome.KeepState())
	createUser(ctx, s, chrome.Creds{User: customUser2, Pass: commonPassword}, chrome.KeepState())
}

func getUsernameFromEmail(email string) string {
	return email[:strings.IndexByte(email, '@')]
}

func manageUsersInSettings(ctx context.Context, s *testing.State, deviceOwner, ownersPassword, userToRemove string) {
	// login as device owner
	creds := chrome.Creds{User: deviceOwner, Pass: ownersPassword}
	opts := append([]chrome.Option{chrome.FakeLogin(creds)}, chrome.KeepState())

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
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
	if err := uiauto.Combine("toggle user management settings",
		ui.WaitUntilExists(nodewith.Name(optionName).Role(role.ToggleButton)),
		ui.LeftClick(nodewith.Name(optionName).Role(role.ToggleButton)),
	)(ctx); err != nil {
		s.Fatal("Failed to manage users: ", err)
	}

	// delete a secondary user
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
	knownEmails := getKnowEmailsFromLocalState(s)

	if knownEmails[userToRemove] {
		s.Fatal("Removed user is still in LoggedInUsers list")
	}
	if !knownEmails[deviceOwner] {
		s.Fatal("Device owner is not in LoggedInUsers list")
	}

	checkUsersCryptohome(ctx, s, userToRemove, false)
	checkUsersCryptohome(ctx, s, deviceOwner, true)
}

func manageUsersOnSignInPage(ctx context.Context, s *testing.State, deviceOwner, removedUser, userToRemove string) {
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

	// secondary user should be removable
	tryRemoveUser(ctx, s, ui, userToRemove, true)

	// there should be no pod for user that was removed earlier
	userPodFound, err := ui.IsNodeFound(ctx, nodewith.Name(removedUser).Role(role.Button))
	if err != nil {
		s.Fatal("Failed to lookup user pod: ", err)
	}
	if userPodFound {
		s.Fatal("Found remove button for a user that should not exist: ", err)
	}

	// device owner should not be removable
	tryRemoveUser(ctx, s, ui, deviceOwner, false)

	// check if device owner still exists, and the other user was removed
	knownEmails := getKnowEmailsFromLocalState(s)
	if knownEmails[userToRemove] {
		s.Fatal("Removed user is still in LoggedInUsers list")
	}
	if !knownEmails[deviceOwner] {
		s.Fatal("Device owner is not in LoggedInUsers list")
	}

	checkUsersCryptohome(ctx, s, deviceOwner, true)
	checkUsersCryptohome(ctx, s, userToRemove, false)
}

func getKnowEmailsFromLocalState(s *testing.State) map[string]bool {
	// LocalState is a json like structure, from which we will need only LoggedInUsers field.
	type LocalState struct {
		Emails []string `json:"LoggedInUsers"`
	}

	localStateFile, err := os.Open("/home/chronos/Local State")
	if err != nil {
		s.Fatal("Failed to open Local State file: ", err)
	}
	defer localStateFile.Close()

	var localState LocalState
	b, err := ioutil.ReadAll(localStateFile)
	if err != nil {
		s.Fatal("Failed to read Local State file contents: ", err)
	}
	if err := json.Unmarshal(b, &localState); err != nil {
		s.Fatal("Failed to unmarshal Local State: ", err)
	}
	knownEmails := make(map[string]bool)
	for _, email := range localState.Emails {
		knownEmails[email] = true
	}

	return knownEmails
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

func tryRemoveUser(ctx context.Context, s *testing.State, ui *uiauto.Context, user string, isRemovable bool) {
	if err := ui.WaitUntilExists(nodewith.Name(user).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pods to be available: ", err)
	}
	if err := ui.LeftClick(nodewith.Name(user).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to click on user pod: ", err)
	}

	if isRemovable {
		// Remove user pod by clicking remove button twice.
		if err := ui.LeftClick(nodewith.Name("Open remove dialog for " + user).Role(role.Button))(ctx); err != nil {
			s.Fatal("Failed to open remove dialog: ", err)
		}

		if err := ui.LeftClick(nodewith.Name("Remove account").Role(role.Button))(ctx); err != nil {
			s.Fatal("Failed to click remove account button first time: ", err)
		}
		if err := ui.LeftClick(nodewith.Name("Remove account").Role(role.Button))(ctx); err != nil {
			s.Fatal("Failed to click remove account button second time: ", err)
		}
		// Check that user pod was deleted.
		if err := ui.WaitUntilGone(nodewith.Name(user).Role(role.Button))(ctx); err != nil {
			s.Fatal("Removed user pod is still reachable: ", err)
		}

	} else {
		removeButtonFound, err := ui.IsNodeFound(ctx, nodewith.Name("Open remove dialog for "+user).Role(role.Button))
		if err != nil {
			s.Fatal("Failed to lookup remove button: ", err)
		}
		if removeButtonFound {
			s.Fatal("Found remove button for a user that is not removable: ", err)
		}
	}
}
