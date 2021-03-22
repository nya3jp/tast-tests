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
		Func: AuthSessionKioskMount,
		Desc: "Ensures creates, authenticate and does kiosk mount with an AuthSession",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// AuthSessionKioskMount ensures that an AuthSession can be used for mounting.
// Here are the steps that this takes:
// 1. Create AuthSession, which gives us back an authSessionID
// 2. Use that authSessionID to create a new kiosk user
// 3. Authenticate the newly created kiosk user
// 4. Perform kiosk mount using AuthSession
// 5. Unmount and remove the kiosk user
func AuthSessionKioskMount(ctx context.Context, s *testing.State) {
	const (
		testUser    = "cryptohome_auth_session_kiosk_user_test@chromium.org"
		testPass    = "" //This is just a standard parameter, that is not used in the function
		isKioskUser = true
	)
	// Start an Auth session with public mount flag and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, testUser, isKioskUser)
	if err != nil {
		s.Fatal("Failed to start Auth session: ", err)
	}
	testing.ContextLogf(ctx, "Auth session ID: %s", authSessionID)

	if err := cryptohome.AddCredentialsWithAuthSession(ctx, testUser, testPass, authSessionID, isKioskUser); err != nil {
		s.Fatal("Failed to add kiosk credentials with AuthSession: ", err)
	}

	defer func(ctx context.Context, s *testing.State, testUser string) {
		// Removing the kiosk user now despite if we could authenticate or not.
		if err := cryptohome.RemoveVault(ctx, testUser); err != nil {
			s.Fatal("Failed to remove kiosk user -: ", err)
		}
		testing.ContextLog(ctx, "Kiosk user removed")
	}(ctx, s, testUser)

	// Authenticate the same AuthSession using authSessionID.
	// If we cannot authenticate, do not proceed with kiosk mount and unmount.
	if err := cryptohome.AuthenticateAuthSession(ctx, testPass, authSessionID, isKioskUser); err != nil {
		s.Fatal("Failed to authenticate with AuthSession: ", err)
	}
	testing.ContextLog(ctx, "Kiosk user authenticated successfully")

	// Performing a kiosk mount with AuthSession now.
	if err := cryptohome.MountWithAuthSession(ctx, authSessionID, isKioskUser); err != nil {
		s.Fatal("Failed to mount kiosk user -: ", err)
	}
	testing.ContextLog(ctx, "Kiosk user mounted successfully")

	// Unmounting kiosk user vault.
	if err := cryptohome.UnmountVault(ctx, testUser); err != nil {
		s.Fatal("Failed to unmount vault for kiosk user -: ", err)
	}
}
