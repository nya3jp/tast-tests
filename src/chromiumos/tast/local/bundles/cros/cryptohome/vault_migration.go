// Copyright 2022 The ChromiumOS Authors
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
		Func: VaultMigration,
		Desc: "Test vault encryption migration from ecryptfs to fscrypt",
		Contacts: []string{
			"dlunev@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "fscrypt_v1",
			ExtraSoftwareDeps: []string{"use_fscrypt_v1"},
		}, {
			Name:              "fscrypt_v2",
			ExtraSoftwareDeps: []string{"use_fscrypt_v2"},
		}},
		Timeout: 60 * time.Second,
	})
}

func VaultMigration(ctx context.Context, s *testing.State) {
	const (
		userName     = "foo@bar.baz"
		userPassword = "secret"
		keyLabel     = "fake_label"
	)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	s.Log("Prepare environment")

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	// Ensure cryptohomed is started and wait for it to be available.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	if err := cryptohome.CreateUserWithAuthSession(ctx, userName, userPassword, keyLabel, false); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)

	s.Log("Create ecryptfs vault with a file")
	authSessionID, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, keyLabel, false /*ephemeral*/, false /*kiosk*/)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if _, err := client.PreparePersistentVault(ctx, authSessionID, true /*ecryptfs*/); err != nil {
		s.Fatal("Failed to prepare ecryptfs vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	if err := cryptohome.VerifyFileUnreadability(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("File is readable after unmount")
	}

	authSessionID, err = cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, keyLabel, false /*ephemeral*/, false /*kiosk*/)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PrepareVaultForMigration(ctx, authSessionID); err != nil {
		s.Fatal("Failed to prepare vault for migration: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	if err := client.MigrateToDircrypto(ctx, userName); err != nil {
		s.Fatal("Failed to migrate vault to dircrypto: ", err)
	}

	s.Log("Mount as fscrypt")
	authSessionID, err = cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, keyLabel, false /*ephemeral*/, false /*kiosk*/)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if _, err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to prepare fscrypt vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Verify that file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to verify file persistence: ", err)
	}
}
