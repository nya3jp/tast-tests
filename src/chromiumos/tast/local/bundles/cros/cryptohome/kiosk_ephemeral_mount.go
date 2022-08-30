// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KioskEphemeralMount,
		Desc: "Ensures that cryptohome correctly mounts ephemeral kiosk sessions through various Auth APIs",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Data: []string{"testcert.p12"},
	})
}

// KioskEphemeralMount tests the following case for ephemeral mounts:
//  Ensure that the user can login with mountEx call
//  Ensure that the user can login with Credential APIs
//  Ensure that the user can login with AuthFactor APIs
//  Ensure that the user can login with AuthFactor APIs with USS Enabled
func KioskEphemeralMount(ctx context.Context, s *testing.State) {
	const (
		ownerUser   = "owner@owner.owner"
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

	daemonController := helper.DaemonController()
	// Ensure cryptohomed is started and wait for it to be available.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Unmount all user vaults before we start.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}

	// Set up the first user as the owner. It is required to mount ephemeral.
	if err := hwseclocal.SetUpVaultAndUserAsOwner(ctx, s.DataPath("testcert.p12"), ownerUser, "whatever", "whatever", helper.CryptohomeClient()); err != nil {
		client.UnmountAll(ctx)
		client.RemoveVault(ctx, ownerUser)
		s.Fatal("Failed to setup vault and user as owner: ", err)
	}
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	defer client.RemoveVault(cleanupCtx, ownerUser)

	// Ensure the vault had been removed.
	if err := cryptohome.RemoveVault(ctx, cryptohome.KioskUser); err != nil {
		s.Log("Failed to remove vault before test starts: ", err)
	}

	vaultConfig := hwsec.NewVaultConfig()
	vaultConfig.Ephemeral = true
	vaultConfig.KioskUser = true

	if err := client.MountVault(ctx, "public_mount", hwsec.NewPassAuthConfig(cryptohome.KioskUser, "" /*loginPassword*/), true, vaultConfig); err != nil {
		s.Fatal("Fail to mount ephemeral kiosk: ", err)
	}
	defer cryptohome.UnmountAll(cleanupCtx)

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Fatal("Failed ot unmount ephemeral user")
	}

	if err := client.MountVault(ctx, "public_mount", hwsec.NewPassAuthConfig(cryptohome.KioskUser, "" /*loginPassword*/), true, vaultConfig); err != nil {
		s.Fatal("Fail to mount ephemeral kiosk: ", err)
	}

	// Write a test file to verify persistence.
	if err := cryptohome.VerifyFileUnreadability(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("File verified when it should not have: ", err)
	}

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Fatal("Failed ot unmount ephemeral user")
	}

	// Authenticate a new auth session via the new added pin auth factor and mount the user.
	authSessionID, err := client.StartAuthSession(ctx, cryptohome.KioskUser, true /*ephemeral*/)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	defer client.InvalidateAuthSession(cleanupCtx, authSessionID)

	if err := client.PrepareEphemeralVault(ctx, authSessionID); err != nil {
		s.Fatal("Failed to prepare ephemeral vault: ", err)
	}

	if err := client.AddCredentialsWithAuthSession(ctx, cryptohome.KioskUser, "" /* no password required*/, "" /*no label required*/, authSessionID, true /*isKioskUser*/); err != nil {
		s.Fatal("Failed to add kiosk credentials: ", err)
	}

	// Write a test file to verify persistence.
	if err := cryptohome.VerifyFileUnreadability(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("File verified when it should not have: ", err)
	}

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Fatal("Failed ot unmount ephemeral user after logging in with credential APIs")
	}

	// Ensure that Kiosk login works when USS flag is disabled, but should
	// still work with AuthFactor API.
	authSessionID, err = client.StartAuthSession(ctx, cryptohome.KioskUser, true /*ephemeral*/)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	defer client.InvalidateAuthSession(cleanupCtx, authSessionID)

	if err := client.PrepareEphemeralVault(ctx, authSessionID); err != nil {
		s.Fatal("Failed to prepare ephemeral vault: ", err)
	}

	if err := client.AddKioskAuthFactor(ctx, authSessionID); err != nil {
		s.Fatal("Failed to add kiosk credentials: ", err)
	}

	// Write a test file to verify persistence.
	if err := cryptohome.VerifyFileUnreadability(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("File verified when it should not have: ", err)
	}

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Fatal("Failed ot unmount ephemeral user after logging in with AuthFactor APIs")
	}

	// Enable the UserSecretStash experiment for the remainder of the test by
	// creating a flag file that's checked by cryptohomed.
	cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
	if err != nil {
		s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
	}
	defer cleanupUSSExperiment()

	// Ensure that Kiosk login works when USS flag is enabled.
	authSessionID, err = client.StartAuthSession(ctx, cryptohome.KioskUser, true /*ephemeral*/)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	defer client.InvalidateAuthSession(cleanupCtx, authSessionID)

	if err := client.PrepareEphemeralVault(ctx, authSessionID); err != nil {
		s.Fatal("Failed to prepare ephemeral vault: ", err)
	}

	if err := client.AddKioskAuthFactor(ctx, authSessionID); err != nil {
		s.Fatal("Failed to add kiosk credentials: ", err)
	}

	// Write a test file to verify persistence.
	if err := cryptohome.VerifyFileUnreadability(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("File verified when it should not have: ", err)
	}

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}
}
