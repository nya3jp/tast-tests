// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"path/filepath"
	"time"

	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ExistingVaultKeysetsWhenUssEnabled,
		Desc: "Test AuthFactor API basic password flow when USS experiment is enabled but there is an existing VaultKeysets",
		Contacts: []string{
			"betuls@google.com",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

const userName = "foo@bar.baz"

func checkKeyFileExists(ctx context.Context, keysetFile string) error {
	const shadowDir = "/home/.shadow"
	hash, err := cryptohome.UserHash(ctx, userName)
	if err != nil {
		return errors.Wrap(err, "failed to get user hash")
	}
	if _, err = os.Stat(filepath.Join(shadowDir, hash, keysetFile)); err != nil {
		return errors.Wrap(err, "failed to stat keyset file")
	}
	return nil
}

// ExistingVaultKeysetsWhenUssEnabled tests the AuthFactor API flows when
// UserSecretStash is enabled for an existing user with legacy VaultKesyets such that:
// - AuthenticateAuthFactor uses VaultKeyset to authenticate an existing
// - AddAuthFactor adds the new factor as a VaultKeyset
// - UpdateAuthFactor successfully updates an existing factor stored via VaultKeyset
// - RemoveAuthFActor removes an existing factor stored via VaultKeyset
func ExistingVaultKeysetsWhenUssEnabled(ctx context.Context, s *testing.State) {
	const (
		userPassword     = "secret"
		userPin          = "1234"
		userPinNew       = "4321"
		passwordLabel    = "gaia"
		pinLabel         = "pin"
		firstKeysetFile  = "master.0" // nocheck
		secondKeysetFile = "master.1" // nocheck
		ussFile          = "user_secret_stash/uss"
	)

	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	// Wait for cryptohomed to become available if needed.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Clean up obsolete state, in case there's any.
	if err := client.UnmountAll(ctx); err != nil {
		s.Error("Failed to unmount vaults for preparation: ", err)
	}
	if _, err := client.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// 1. Create a new user when UserSecretStash is not enabled and add a password factor.
	//
	// Start an Auth session and get an authSessionID.
	_, authSessionID, err := client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start AuthSession: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanup, authSessionID)
	// Create user vault.
	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create user: ", err)
	}
	defer client.RemoveVault(ctxForCleanup, userName)
	// Mount user home directories and daemon-store directories.
	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to mount user profile after creation: ", err)
	}
	defer client.Unmount(ctxForCleanup, userName)
	// Add password AuthFactor.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to add password AuthFactor: ", err)
	}
	// Check that the password VaultKeyset file is created.
	if err := checkKeyFileExists(ctx, firstKeysetFile); err != nil {
		s.Fatal("Failed to check password VaultKeyset file: ", err)
	}
	// Invalidate AuthSession and unmount user directories.
	if err := client.CleanupSession(ctx, authSessionID); err != nil {
		s.Fatal("Failed to cleanup user AuthSession: ", err)
	}

	// 2. Enable the UserSecretStash experiment.
	cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
	if err != nil {
		s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
	}
	defer cleanupUSSExperiment(ctxForCleanup)

	// 3. Add a second credential to the existing user after UserSecretStash is enabled.
	// The new VaultKeyset should be added and UserSecretStash shouldn't be created.
	//
	// Start an Auth session and get an authSessionID.
	_, authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start Auth session: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanup, authSessionID)
	// Authenticate with password AuthFactor.
	if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to authenticate with password AuthFactor: ", err)
	}
	// Mount user home directories and daemon-store directories.
	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to mount user profile: ", err)
	}
	// Add PIN AuthFactor and authenticate with it.
	if err := client.AddPinAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
		s.Fatal("Failed to add PIN AuthFactor: ", err)
	}
	if _, err := client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
		s.Fatal("Failed to authenticate with pin AuthFactor: ", err)
	}
	// Check that the PIN VaultKeyset file is created.
	if err := checkKeyFileExists(ctx, secondKeysetFile); err != nil {
		s.Fatal("Failed to check PIN VaultKeyset file: ", err)
	}
	// Check that UserSecretStash file is not created.
	if err := checkKeyFileExists(ctx, ussFile); err == nil {
		s.Fatal("UserSecretStash file shouldn't have been created but found in disk: ", err)
	}
	// Invalidate AuthSession and unmount user directories.
	if err := client.CleanupSession(ctx, authSessionID); err != nil {
		s.Fatal("Failed to cleanup user AuthSession: ", err)
	}

	// 4. Update the second AuthFactor and then remove the second AuthFactor VaultKeyset backed credential when
	// UserSecretStash is enabled. Both update and remove operations should operate with the VaultKeyset, and
	// UserSecretStash shouldn't be created.
	//
	// Start an Auth session and get an authSessionID.
	_, authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start Auth session: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanup, authSessionID)
	// Authenticate with password AuthFactor.
	if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to authenticate with password AuthFactor: ", err)
	}
	// Mount user home directories and daemon-store directories.
	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to mount user profile after authenticating with password: ", err)
	}
	// Update PIN AuthFactor.
	if err := client.UpdatePinAuthFactor(ctx, authSessionID, pinLabel, userPinNew); err != nil {
		s.Fatal("Failed to add password AuthFactor: ", err)
	}
	// Check that the PIN VaultKeyset file still exists.
	if err := checkKeyFileExists(ctx, secondKeysetFile); err != nil {
		s.Fatal("Failed to check PIN VaultKeyset file: ", err)
	}
	// Check that UserSecretStash file is not created.
	if err := checkKeyFileExists(ctx, ussFile); err == nil {
		s.Fatal("UserSecretStash file shouldn't have been created but found in disk: ", err)
	}
	// Invalidate AuthSession to re-authenticate with the updated PIN
	if err := client.InvalidateAuthSession(ctx, authSessionID); err != nil {
		s.Fatal("Failed to invalidate AuthSession: ", err)
	}
	// Start a new session and test authenticate with the updated PIN & not authenticate with the old PIN.
	_, authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start Auth session: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanup, authSessionID)
	// Authenticate with old PIN should fail.
	if _, err := client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, userPin); err == nil {
		s.Fatal("Authenticating with the old PIN succeeded but should have failed: ", err)
	}
	// Authenticate with updated PIN.
	if _, err := client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, userPinNew); err != nil {
		s.Fatal("Authenticating with the updated PIN failed: ", err)
	}
	// Authenticate with password AuthFactor to remove the PIN factor.
	if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to authenticate with password AuthFactor: ", err)
	}
	// Remove AuthFactor, the PIN backed by vault keyset.
	if err := client.RemoveAuthFactor(ctx, authSessionID, pinLabel); err != nil {
		s.Fatal("Authenticating with the updated PIN failed: ", err)
	}
	// Check that the PIN VaultKeyset file is deleted.
	if err := checkKeyFileExists(ctx, secondKeysetFile); err == nil {
		s.Fatal("PIN VaultKeyset file exists, should have been deleted: ", err)
	}
	// Authenticate with the removed factor should now fail.
	if _, err := client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, userPinNew); err == nil {
		s.Fatal("Authenticating with PIN should have failed after removal, but succeeded: ", err)
	}
}
