// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// NOTE: This test is largely similar to hwsec.DictionaryAttackLockoutResetTPM1 (a remote test), if change is
// made to one, it is likely that the other have to be changed as well.
// The referred test is specifically for TPMv1.2, while this test is for TPMv2.0.
// Both versions of TPM is incompatible with each other and the available NVRAM index differs across the 2 versions.
// Therefore, we need 2 versions of the test.
// This version creates new NVRAM space because none of the TPMv2.0 index is guaranteed to exist and still
// generates a dictionary attack event when read/write with incorrect auth value. Creating a new NVRAM space
// is not feasible on TPMv1.2 because soft-clear is not available there yet and we don't want another reboot.

func init() {
	testing.AddTest(&testing.Test{
		Func: DictionaryAttackLockoutResetTPM2,
		Desc: "Verifies that on TPMv2.0 devices, dictionary attack counter functions correctly and can be reset",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// DictionaryAttackLockoutResetTPM2 checks that get dictionary attack info and reset dictionary attack lockout works as expected.
func DictionaryAttackLockoutResetTPM2(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	tpmManager := helper.TPMManagerClient()

	// In this test, we want to check if DA counter increases, and then reset it to see if everything is correct.
	// Reset/Clear TPM => Check DA Counter => Create NVRAM Index => Write NVRAM Index => Check DA Counter => Reset DA Lockout => Check DA Counter
	// Write NVRAM Index is used to trigger an increase in DA counter.

	// Reset TPM and take ownership.
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}
	if err := helper.EnsureTPMIsReadyAndBackupSecrets(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	// Check that the counter is 0 right after resetting.
	err = hwseclocal.CheckDAIsZero(ctx, tpmManager)
	if err != nil {
		s.Fatal("Failed to check DA counter is zero: ", err)
	}

	const testNVRAMIndex = "0xADF00D"
	const testPassword = "1234"
	const testIncorrectPassword = "4321"
	const testFilePath = "/tmp/dictionary_attack_test_1byte.bin"

	// Create the NVRAM space so that we can attempt to increase the counter by unauthorized write.
	if _, err := tpmManager.DefineSpace(ctx, 1, false, testNVRAMIndex, []string{hwsec.NVRAMAttributeWriteAuth}, testPassword); err != nil {
		s.Fatal("Failed to create NVRAM space: ", err)
	}
	// Clean up by removing the NVRAM space.
	defer func() {
		if _, err := tpmManager.DestroySpace(ctx, testNVRAMIndex); err != nil {
			s.Error("Failed to destroy NVRAM space: ", err)
		}
	}()

	// Create a 1 byte file for writing
	if _, err := cmdRunner.Run(ctx, "dd", "if=/dev/zero", "of="+testFilePath, "bs=1", "count=1"); err != nil {
		s.Fatal("Failed to create test file: ", err)
	}
	defer func() {
		if _, err := cmdRunner.Run(ctx, "rm", "-f", testFilePath); err != nil {
			s.Error("Failed to remove tmp file: ", err)
		}
	}()

	// Try to write the NVRAM space with incorrect password to increase the counter.
	if _, err := tpmManager.WriteSpaceFromFile(ctx, testNVRAMIndex, testFilePath, testIncorrectPassword); err == nil {
		s.Fatal("Writing NVRAM Space should not succeed with incorrect password")
	}

	// Check counter, should be 0, and lockout shouldn't be in effect.
	err = hwseclocal.CheckDAIsZero(ctx, tpmManager)
	if err != nil {
		s.Fatal("Failed to check DA counter is zero: ", err)
	}
}
