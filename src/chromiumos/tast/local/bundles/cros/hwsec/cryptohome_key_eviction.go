// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeKeyEviction,
		Desc: "Ensures that the cryptohome properly manages key eviction from the tpm",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@google.com",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// CryptohomeKeyEviction ensures that the cryptohome properly manages key eviction from the tpm.
// This test verifies this behaviour by creating 30 keys using chaps, and then remounting a user's cryptohome.
// Mount requires use of the user's cryptohome key, and thus the mount only succeeds if the cryptohome key was properly evicted and reloaded into the TPM.
func CryptohomeKeyEviction(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	cryptohome := helper.CryptohomeClient()

	chaps, err := pkcs11.NewChaps(ctx, cmdRunner, cryptohome)
	if err != nil {
		s.Fatal("Failed to create chaps client: ", err)
	}

	const (
		user     = util.FirstUsername
		password = util.FirstPassword
	)

	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure TPM is ready: ", err)
	}

	defer func() {
		// Ensure we remove the user account after the test.
		if _, err := cryptohome.Unmount(ctx, user); err != nil {
			s.Fatal("Failed to unmount: ", err)
		}
		if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
	}()

	// Ensure clean cryptohome.
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}
	if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	// Mount the user vault.
	if err := cryptohome.MountVault(ctx, user, password, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault: ", err)
	}

	// Wait and get the user slot.
	if err := cryptohome.WaitForUserToken(ctx, user); err != nil {
		s.Fatal("Failed to wait for user token: ", err)
	}
	slot, err := cryptohome.GetTokenForUser(ctx, user)
	if err != nil {
		s.Fatal("Failed to get user slot: ", err)
	}

	// First we inject 30 tokens into chaps. This forces the cryptohome key to get evicted.
	for i := 0; i < 30; i++ {
		if err := chaps.ReplayWifiBySlot(ctx, slot, "--inject"); err != nil {
			s.Fatal("Failed to inject a key into a PKCS #11 token and tests that it can sign: ", err)
		}
	}

	// Then we get a user to remount cryptohome. This process uses the cryptohome key,
	// and if the user was able to login, the cryptohome key was correctly reloaded.
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}
	if err := cryptohome.MountVault(ctx, user, password, util.PasswordLabel, false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault: ", err)
	}
}
