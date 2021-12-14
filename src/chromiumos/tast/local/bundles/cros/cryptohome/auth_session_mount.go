// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// authSessionMountParam contains the test parameters which are different
// between the types of mounts.
type authSessionMountParam struct {
	// Specifies the user email with which to login.
	testUser string
	// Specifies the password to login with, for kiosk users this is empty.
	testPass string
	// Specifies if the user is a kiosk user.
	isKioskUser bool
	// Should AuthSession create user.
	createUser bool
	// Requested duration length to extend AuthSession, beyond default of
	// five minutes, expressed in seconds.
	extensionDuration int
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
				testUser:          "cryptohome_test@chromium.org",
				testPass:          "testPass",
				isKioskUser:       false,
				createUser:        true,
				extensionDuration: 60, // 1 minute extension
			},
		}, {
			Name: "kiosk_mount",
			Val: authSessionMountParam{
				testUser:          cryptohome.KioskUser,
				testPass:          "", // Password is derived from username
				isKioskUser:       true,
				createUser:        true,
				extensionDuration: 60, // 1 minute extension
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
// 5. Extend the AuthSession beyond default lifetime
// 6. Invalidate the AuthSession in memory
// 7. Unmount and remove the user
func AuthSessionMount(ctx context.Context, s *testing.State) {
	userParam := s.Param().(authSessionMountParam)

	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	// Ensure cryptohomed is started and wait for it to be available.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Unmount all user vaults before we start.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}

	// Run AuthSession Mount Flow for creating user.
	if err := cryptohome.AuthSessionMountFlow(ctx, userParam.isKioskUser, userParam.testUser, userParam.testPass, userParam.createUser, userParam.extensionDuration); err != nil {
		s.Fatal("Failed to Mount with AuthSession -: ", err)
	}

	// Tests to ensure the kiosk users work with the old API.
	if userParam.isKioskUser {
		if err := cryptohome.MountKiosk(ctx); err != nil {
			s.Fatal("Failed to mount kiosk: ", err)
		}
		// Unmount Vault.
		defer cryptohome.UnmountVault(ctx, cryptohome.KioskUser)
	}
}
