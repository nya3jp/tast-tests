// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AuthSessionMount,
		Desc: "Ensures creates, authenticate and mount with an AuthSession",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
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
	const (
		testUser = "cryptohome_auth_session_test@chromium.org"
		testPass = "testme"
	)
	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, testUser)
	if err != nil {
		s.Error("Failed to start Auth session: ", err)
	}
	testing.ContextLogf(ctx, "Auth session ID: %s", authSessionID)

	err = cryptohome.AddCredentialsWithAuthSession(ctx, testUser, testPass, authSessionID)
	if err != nil {
		s.Error("Failed to add credentials with AuthSession: ", err)
		return
	}
	// Authenticate the same AuthSession using authSessionID.
	// If we cannot authenticate, do not proceed with mount and unmount.
	err = cryptohome.AuthenticateAuthSession(ctx, testPass, authSessionID)
	if err != nil {
		s.Error("Failed to authenticate with AuthSession: ", err)
	} else {
		testing.ContextLog(ctx, "User authenticated successfully")

		// Mounting with AuthSession now.
		err = cryptohome.MountWithAuthSession(ctx, authSessionID)
		if err != nil {
			s.Error("Failed to mount user -: ", err)
		} else {
			testing.ContextLog(ctx, "User mounted successfully")
			// Unmounting user vault.
			err = cryptohome.UnmountVault(ctx, testUser)
			if err != nil {
				s.Error("Failed to unmount vault user -: ", err)
			}
		}
	}

	// Removing the user now despite if we could authenticate or not.
	err = cryptohome.RemoveVault(ctx, testUser)
	if err != nil {
		s.Error("Failed to remove user -: ", err)
	}
	testing.ContextLog(ctx, "User removed")
}
