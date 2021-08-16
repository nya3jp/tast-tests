// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

// NOTE: This test is largely similar to hwsec.RecreateUserVaultTPM2 (a local test), if change is made to one, it is likely that the other have to be changed as well.
// The referred test is specifically for TPMv2.0, while this test is for TPMv1.2.
// Both versions of TPM is incompatible with each other and they way we handle reboot for the 2 versions are different and thus the need for 2 versions of the same test.

func init() {
	testing.AddTest(&testing.Test{
		Func: RecreateUserVaultTPM1,
		Desc: "Verifies that for TPMv1.2 devices, cryptohome recreates user's vault directory when the TPM is re-owned",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"reboot", "tpm1"},
		Attr:         []string{"group:hwsec_destructive_func"},
		Timeout:      5 * time.Minute,
	})
}

// RecreateUserVaultTPM1 is the TPMv1.2 version of hwsec.RecreateUserVault test,
// which was ported from the autotest test platform_CryptohomeTPMReOwn and
// renamed to reflects what's being tested.
func RecreateUserVaultTPM1(ctx context.Context, s *testing.State) {
	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())

	helper, err := hwsecremote.NewHelper(cmdRunner, s.DUT())
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	utility := helper.CryptohomeClient()

	// Resets the TPM states before running the tests.
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
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

	s.Log("Phase 2: reboot and mounts user vault")

	// Reboot
	if err := helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
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
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
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
