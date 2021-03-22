// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

// authSessionMountParam contains the test parameters which are different
// between the types of mounts.
type authSessionMountParam struct {
	// Specifies the user email with which to login
	testUser string
	// Specifies the password to login with, for kiosk users this is empty.
	testPass string
	// Specifies if the user is a kiosk user
	isKioskUser bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: AuthSessionMount,
		Desc: "Ensures creates, authenticate and mount with an AuthSession",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "regular_mount",
			Val: authSessionMountParam{
				testUser:    "cryptohome_auth_session_test@chromium.org",
				testPass:    "testPass",
				isKioskUser: false,
			},
		}, {
			Name: "kiosk_mount",
			Val: authSessionMountParam{
				testUser:    "cryptohome_auth_session_kiosk_test@chromium.org",
				testPass:    "", // Password is derived from username
				isKioskUser: true,
			},
		}},
	})
}

// AuthSessionMount ensures that an AuthSession can be used for mounting.
// Here are the steps that this takes:
// 1. Create AuthSession, which gives us back an authSessionID
// 2. Use that authSessionID to create a new user
// 3. Authenticate the newly created user
// 4. Perform mount using AuthSession
// 5. Unmount and remove the user
func AuthSessionMount(ctx context.Context, s *testing.State) {
	userParam := s.Param().(authSessionMountParam)
	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, userParam.testUser, userParam.isKioskUser)
	if err != nil {
		s.Fatal("Failed to start Auth session: ", err)
	}
	testing.ContextLogf(ctx, "Auth session ID: %s", authSessionID)

	if err := cryptohome.AddCredentialsWithAuthSession(ctx, userParam.testUser, userParam.testPass, authSessionID, userParam.isKioskUser); err != nil {
		s.Fatal("Failed to add credentials with AuthSession: ", err)
	}

	defer func(ctx context.Context, s *testing.State, testUser string) {
		// Removing the user now despite if we could authenticate or not.
		if err := cryptohome.RemoveVault(ctx, testUser); err != nil {
			s.Fatal("Failed to remove user -: ", err)
		}
		testing.ContextLog(ctx, "User removed")
	}(ctx, s, userParam.testUser)

	// Authenticate the same AuthSession using authSessionID.
	// If we cannot authenticate, do not proceed with mount and unmount.
	if err := cryptohome.AuthenticateAuthSession(ctx, userParam.testPass, authSessionID, userParam.isKioskUser); err != nil {
		s.Fatal("Failed to authenticate with AuthSession: ", err)
	}
	testing.ContextLog(ctx, "User authenticated successfully")

	// Mounting with AuthSession now.
	if err := cryptohome.MountWithAuthSession(ctx, authSessionID, userParam.isKioskUser); err != nil {
		s.Fatal("Failed to mount user -: ", err)
	}
	testing.ContextLog(ctx, "User mounted successfully")

	// Unmounting user vault.
	if err := cryptohome.UnmountVault(ctx, userParam.testUser); err != nil {
		s.Fatal("Failed to unmount vault user -: ", err)
	}
}
