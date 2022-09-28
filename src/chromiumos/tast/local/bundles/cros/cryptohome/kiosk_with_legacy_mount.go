// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KioskWithLegacyMount,
		Desc: "Ensures that cryptohome correctly mounts kiosk sessions through various Auth APIs",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// KioskWithLegacyMount tests the following case:
// User Created with Legacy mountEx call (this will involve KEY_TYPE_PASSWORD):
//
//	Ensure that the user can login with mountEx call
//	Ensure that the user can login with Credential APIs
//	Ensure that the user can login with AuthFactor APIs
//	Ensure that the user can login with AuthFactor APIs with USS Enabled
func KioskWithLegacyMount(ctx context.Context, s *testing.State) {
	const (
		cleanupTime = 20 * time.Second
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	// Unmount all user vaults before we start.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}

	// Ensure the vault had been removed.
	if err := cryptohome.RemoveVault(ctx, cryptohome.KioskUser); err != nil {
		s.Log("Failed to remove vault before test starts: ", err)
	}

	// Create and mount the kiosk for the first time.
	if err := cryptohome.MountKiosk(ctx); err != nil {
		s.Fatal("Failed to mount kiosk: ", err)
	}
	defer cryptohome.RemoveVault(cleanupCtx, cryptohome.KioskUser)
	defer cryptohome.UnmountAll(cleanupCtx)

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	if err := cryptohome.UnmountVault(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to unmount vault: ", err)
	}

	// Verify that file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, cryptohome.KioskUser); err == nil {
		s.Fatal("Failed to unmount successfully as file is still readable")
	}

	//	Ensure that the user can login with mountEx call
	if err := cryptohome.MountKiosk(ctx); err != nil {
		s.Fatal("Failed to mount kiosk: ", err)
	}

	// Verify that file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to verify file persistence: ", err)
	}

	if err := cryptohome.UnmountVault(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to unmount vault: ", err)
	}

	// Authenticate a new auth session via the new added pin auth factor and mount the user.
	_, authSessionID, err := client.StartAuthSession(ctx, cryptohome.KioskUser, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	defer client.InvalidateAuthSession(cleanupCtx, authSessionID)

	if err = client.AuthenticateAuthSession(ctx, "", "public_mount", authSessionID, true /*isKioskUser*/); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}

	if err := cryptohome.MountAndVerify(ctx, cryptohome.KioskUser, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to mount and verify persistence: ", err)
	}

	if err := client.InvalidateAuthSession(ctx, authSessionID); err != nil {
		s.Fatal("Failed to invalidate AuthSession: ", err)
	}

	// Unmount user vault.
	if err := cryptohome.UnmountVault(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to unmount vault: ", err)
	}

	// Ensure that Kiosk login works when USS flag is disabled, but should
	// still work with AuthFactor API.
	_, authSessionID, err = client.StartAuthSession(ctx, cryptohome.KioskUser, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	defer client.InvalidateAuthSession(cleanupCtx, authSessionID)
	if err = client.AuthenticateKioskAuthFactor(ctx, authSessionID); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}

	if err := cryptohome.MountAndVerify(ctx, cryptohome.KioskUser, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to mount and verify persistence: ", err)
	}

	if err := client.InvalidateAuthSession(ctx, authSessionID); err != nil {
		s.Fatal("Failed to invalidate AuthSession: ", err)
	}

	// Unmount user vault.
	if err := cryptohome.UnmountVault(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to unmount vault: ", err)
	}

	// Enable the UserSecretStash experiment for the remainder of the test by
	// creating a flag file that's checked by cryptohomed.
	cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
	if err != nil {
		s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
	}
	defer cleanupUSSExperiment(ctx)

	// Ensure that Kiosk login works when USS flag is enabled.
	_, authSessionID, err = client.StartAuthSession(ctx, cryptohome.KioskUser, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	if err = client.AuthenticateKioskAuthFactor(ctx, authSessionID); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}
	defer client.InvalidateAuthSession(cleanupCtx, authSessionID)

	if err := cryptohome.MountAndVerify(ctx, cryptohome.KioskUser, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to mount and verify persistence: ", err)
	}

	if err := client.InvalidateAuthSession(ctx, authSessionID); err != nil {
		s.Fatal("Failed to invalidate AuthSession: ", err)
	}

	// Unmount user vault.
	if err := cryptohome.UnmountVault(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to unmount vault: ", err)
	}
}
