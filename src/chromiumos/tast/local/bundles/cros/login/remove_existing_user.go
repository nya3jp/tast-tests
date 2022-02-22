// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/ctxutil"
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
		Func:         RemoveExistingUser,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Remove user pods from start screen",
		Contacts:     []string{"dkuzmin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: 5*chrome.LoginTimeout + 25*time.Second,
	})
}

const (
	localStatePath = "/home/chronos/Local State"
	knownUsersList = "KnownUsers"
)

func RemoveExistingUser(ctx context.Context, s *testing.State) {
	// LocalState is a json like structure, from which we will need only LoggedInUsers field.
	type LocalState struct {
		Emails []string `json:"LoggedInUsers"`
	}
	const (
		user1    = "user1@gmail.com"
		user2    = "user2@gmail.com"
		user3    = "user3@gmail.com"
		password = "password"
	)
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	if err := userutil.CreateUser(ctx, user1, password); err != nil {
		s.Fatal("Failed to create new user1: ", err)
	}
	if err := userutil.CreateUser(ctx, user2, password, chrome.KeepState()); err != nil {
		s.Fatal("Failed to create new user2: ", err)
	}
	if err := userutil.CreateUser(ctx, user3, password, chrome.KeepState()); err != nil {
		s.Fatal("Failed to create new user3: ", err)
	}

	removeUsersOnLoginScreen(ctx, cleanUpCtx, s, user1, user3)

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

	// Check that there is no user3 in LoggedInUsers list.
	knownEmails, err := userutil.GetKnownEmailsFromLocalState()
	if err != nil {
		s.Fatal("Failed to get known emails from local state: ", err)
	}
	if knownEmails[user3] {
		s.Fatal("Removed user is still in LoggedInUsers list")
	}

	// Check that cryptohome for user3 was deleted.
	path, err := cryptohome.UserPath(ctx, user3)
	if _, err := os.Stat(path); err == nil {
		s.Fatal("Cryptohome directory still exists under ", path)
	} else if !os.IsNotExist(err) {
		s.Fatal("Unexpected error: ", err)
	}

	// Connect to login extension.
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tLoginConn)
	ui := uiauto.New(tLoginConn)
	// Wait for user pods to be available.
	if err := ui.WaitUntilExists(nodewith.Name(user1).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pods to be available after reboot: ", err)
	}
	if err := ui.WaitUntilExists(nodewith.Name(user2).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pods to be available after reboot: ", err)
	}
	// Check that there is no user pod for user3.
	if err := ui.Gone(nodewith.Name(user3).Role(role.Button))(ctx); err != nil {
		s.Fatal("Removed user pod for " + user3 + " still exists")
	}
}

func removeUsersOnLoginScreen(ctx, cleanUpCtx context.Context, s *testing.State, deviceOwner, user string) {
	// chrome.NoLogin() and chrome.KeepState() are needed to show the login
	// screen with a user pod (instead of the OOBE login screen).
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
	// Connect to login extension.
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanUpCtx, s.OutDir(), s.HasError, tLoginConn)
	ui := uiauto.New(tLoginConn)
	// Wait for user pods to be available.
	if err := ui.WaitUntilExists(nodewith.Name(user).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pods to be available: ", err)
	}
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

	// try to delete device owner - it should not be possible
	if err := ui.WaitUntilExists(nodewith.Name(deviceOwner).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to wait for user pods to be available: ", err)
	}
	if err := ui.LeftClick(nodewith.Name(deviceOwner).Role(role.Button))(ctx); err != nil {
		s.Fatal("Failed to click on user pod: ", err)
	}

	removeButtonFound, err := ui.IsNodeFound(ctx, nodewith.Name("Open remove dialog for "+deviceOwner).Role(role.Button))
	if err != nil {
		s.Fatal("Failed to lookup remove button: ", err)
	}
	if removeButtonFound {
		s.Fatal("Found remove button for device owner, who should not be removable: ", err)
	}
}
