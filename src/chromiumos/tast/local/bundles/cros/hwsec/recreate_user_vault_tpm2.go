// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// NOTE: This test is largely similar to hwsec.RecreateUserVaultTPM1 (a remote test), if change is made to one,
// it is likely that the other have to be changed as well.
// The referred test is specifically for TPMv1.2, while this test is for TPMv2.0.
// Both versions of TPM are incompatible with each other and the way we handle reboot for the 2 versions are
// different and thus the need for 2 versions of the same test.

func init() {
	testing.AddTest(&testing.Test{
		Func: RecreateUserVaultTPM2,
		Desc: "Verifies that for TPMv2.0 devices, cryptohome recreates user's vault directory when the TPM is re-owned",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"garryxiao@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline"},
		Timeout:      3 * time.Minute,
	})
}

// RecreateUserVaultTPM2 is ported from the autotest test platform_CryptohomeTPMReOwn and renamed to
// reflects what's being tested. It avoids reboots in the original test by using the soft-clearing
// TPM utils and restarting TPM-related daemons.
func RecreateUserVaultTPM2(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	utility := helper.CryptohomeClient()

	// Resets the TPM, system, and user states before running the tests.
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err = cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}
	if err := helper.EnsureTPMIsReadyAndBackupSecrets(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}
	if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}

	s.Log("Phase 1: mounts vault for the test user")

	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}
	if err := utility.CheckTPMWrappedUserKeyset(ctx, util.FirstUsername); err != nil {
		s.Fatal("Check user keyset failed: ", err)
	}
	if err := hwsec.WriteUserTestContent(ctx, utility, cmdRunner, util.FirstUsername, util.TestFileName1, util.TestFileContent); err != nil {
		s.Fatal("Failed to write user test content: ", err)
	}
	if _, err := utility.Unmount(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}

	s.Log("Phase 2: restarts TPM daemons and mounts user vault")

	// Restarts all TPM daemons to simulate a reboot in the original autotest test.
	if err := helper.DaemonController().RestartTPMDaemons(ctx); err != nil {
		s.Fatal("Failed to restart TPM-related daemons to simulate reboot: ", err)
	}
	if err = cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}
	if err := utility.CheckTPMWrappedUserKeyset(ctx, util.FirstUsername); err != nil {
		s.Fatal("Check user keyset failed: ", err)
	}

	// User vault should already exist and shouldn't be recreated.
	if content, err := hwsec.ReadUserTestContent(ctx, utility, cmdRunner, util.FirstUsername, util.TestFileName1); err != nil {
		s.Fatal("Failed to read user test content: ", err)
	} else if !bytes.Equal(content, []byte(util.TestFileContent)) {
		s.Fatalf("Unexpected test file content: got %q, want %q", string(content), util.TestFileContent)
	}

	if _, err := utility.Unmount(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to remove user vault: ", err)
	}

	s.Log("Phase 3: clears TPM and mounts user vault again")

	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err = cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}
	if err := helper.EnsureTPMIsReadyAndBackupSecrets(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}
	if err := utility.CheckTPMWrappedUserKeyset(ctx, util.FirstUsername); err != nil {
		s.Fatal("Check user keyset failed: ", err)
	}

	// User vault should be recreated after TPM is cleared.
	if exists, err := hwsec.DoesUserTestFileExist(ctx, utility, cmdRunner, util.FirstUsername, util.TestFileName1); err != nil {
		s.Fatal("Failed to check user test file: ", err)
	} else if exists {
		s.Fatal("Cryptohome didn't recreate user vault; original test file still exists")
	}
}
