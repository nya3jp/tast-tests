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
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type vaultType int64

type params struct {
	VaultType vaultType
}

const (
	noneVaultType vaultType = iota
	ecryptfsVaultType
	fscryptV1VaultType
	defaultVaultType
)

const (
	cleanupTime = 20 * time.Second
	testTime    = 60 * time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PersistentAuthSession,
		Desc: "Test new create/prepare API for persistent vault with auth session",
		Contacts: []string{
			"dlunev@chromium.org",
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline"},
		// Only tpm2 support soft TPM clear.
		SoftwareDeps: []string{"tpm2"},
		Params: []testing.Param{{
			Name: "default",
			Val:  &params{VaultType: defaultVaultType},
		}, {
			Name: "ecryptfs",
			Val:  &params{VaultType: ecryptfsVaultType},
		}, {
			Name:              "fscrypt_v1",
			ExtraSoftwareDeps: []string{"use_fscrypt_v2"},
			Val:               &params{VaultType: fscryptV1VaultType},
		}},
		Timeout: testTime,
	})
}

func PersistentAuthSession(ctx context.Context, s *testing.State) {
	const (
		userName     = "foo@bar.baz"
		userPassword = "secret"
		keyLabel     = "fake_label"
	)

	vtype := s.Param().(*params).VaultType
	isEcryptfs := vtype == ecryptfsVaultType

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}

	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	if vtype == fscryptV1VaultType {
		s.Log("Switch cryptohome to fscryptv1 ")
		if err := upstart.RestartJob(ctx, "cryptohomed", upstart.WithArg("CRYPTOHOMED_ARGS", "--negate_fscrypt_v2_for_test")); err != nil {
			s.Fatal("Can't disable fscryptv2: ", err)
		}
		defer upstart.RestartJob(ctx, "cryptohomed")
	}

	if err := cryptohome.CreateUserWithAuthSession(ctx, userName, userPassword, keyLabel, false); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)

	// Mount the vault for the first time.
	authSessionID, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, keyLabel, false, false)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if _, err := client.PreparePersistentVault(ctx, authSessionID, isEcryptfs); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	if _, err := client.PreparePersistentVault(ctx, authSessionID, isEcryptfs); err == nil {
		s.Fatal("Secondary prepare attempt for the same user should fail, but succeeded")
	}

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	// Unmount and mount again.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	if err := cryptohome.VerifyFileUnreadability(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("File is readable after unmount")
	}

	if vtype == fscryptV1VaultType {
		if err := upstart.RestartJob(ctx, "cryptohomed"); err != nil {
			s.Fatal("Can't reset cryptohomed")
		}
	}

	authSessionID, err = cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, keyLabel, false, false)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if _, err := client.PreparePersistentVault(ctx, authSessionID, isEcryptfs); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}

	// Verify that file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to verify file persistence: ", err)
	}
}
