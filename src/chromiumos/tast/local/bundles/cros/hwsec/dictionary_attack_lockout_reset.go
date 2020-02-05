// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DictionaryAttackLockoutReset,
		Desc: "Verifies that dictionary attack counter functions correctly and can be reset",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"informational"},
	})
}

// getDAInfo is a simple utility function that calls GetDAInfo from both tpm_manager and cryptohome, and see if they match. If both succeeded and the results agree with each other, then err is nil.
func getDAInfo(ctx context.Context, cryptohomeUtil *hwsec.UtilityCryptohomeBinary, tpmManagerUtil *hwsec.UtilityTpmManagerBinary) (counter int, threshold int, inEffect bool, remaining int, returnedError error) {
	// Initialize the return parameters first.
	counter = -1
	threshold = -1
	inEffect = false
	remaining = -1

	// Get values from tpm_manager.
	tpmManagerCounter, tpmManagerThreshold, tpmManagerInEffect, tpmManagerRemaining, err := tpmManagerUtil.GetDAInfo(ctx)
	if err != nil {
		returnedError = errors.Wrap(err, "failed to get dictionary attack info from TpmManager")
		return
	}

	// Get values from cryptohome.
	cryptohomeCounter, cryptohomeThreshold, cryptohomeInEffect, cryptohomeRemaining, err := cryptohomeUtil.GetDAInfo(ctx)
	if err != nil {
		returnedError = errors.Wrap(err, "failed to get dictionary attack info from cryptohome")
		return
	}

	// Now check the values.
	if cryptohomeCounter != tpmManagerCounter {
		returnedError = errors.Errorf("cryptohome (%d) and tpm_manager (%d) disagree on counter value", cryptohomeCounter, tpmManagerCounter)
		return
	}
	if cryptohomeThreshold != tpmManagerThreshold {
		returnedError = errors.Errorf("cryptohome (%d) and tpm_manager (%d) disagree on threshold value", cryptohomeThreshold, tpmManagerThreshold)
		return
	}
	if cryptohomeInEffect != tpmManagerInEffect {
		returnedError = errors.Errorf("cryptohome (%t) and tpm_manager (%t) disagree on in effect value", cryptohomeInEffect, tpmManagerInEffect)
		return
	}
	if cryptohomeRemaining != tpmManagerRemaining {
		returnedError = errors.Errorf("cryptohome (%d) and tpm_manager (%d) disagree on remaining value", cryptohomeRemaining, tpmManagerRemaining)
		return
	}

	counter = cryptohomeCounter
	threshold = cryptohomeThreshold
	inEffect = cryptohomeInEffect
	remaining = cryptohomeRemaining

	return
}

// DictionaryAttackLockoutReset checks that get dictionary attack info and reset dictionary attack lockout works as expected.
func DictionaryAttackLockoutReset(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	cryptohomeUtil, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}
	tpmManagerUtil, err := hwsec.NewUtilityTpmManagerBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityTpmManagerBinary: ", err)
	}
	helper, err := hwseclocal.NewHelper(cryptohomeUtil)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	// In this test, we want to check if DA counter increases, and then reset it to see if everything is correct.
	// Reset/Clear TPM => Check DA Counter => Create NVRAM Index => Write NVRAM Index => Check DA Counter => Reset DA Lockout => Check DA Counter
	// Write NVRAM Index is used to trigger an increase in DA counter.

	// Reset TPM and take ownership.
	if err := hwseclocal.ResetTPMAndSystemStates(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	// Check that the counter is 0 right after resetting.
	counter, _, inEffect, _, err := getDAInfo(ctx, cryptohomeUtil, tpmManagerUtil)
	if err != nil {
		s.Fatal("Failed to get dictionary attack info: ", err)
	}
	if counter != 0 {
		s.Fatalf("Incorrect counter, got %d expect 0", counter)
	}
	if inEffect {
		s.Fatal("Lockout in effect after reset")
	}

	const testNVRAMIndex = "0xBAADF00D"
	const testPassword = "1234"
	const testIncorrectPassword = "4321"
	const testFilePath = "/tmp/dictionary_attack_test_1byte.bin"

	// Create the NVRAM space so that we can attempt to increase the counter.
	if _, err := tpmManagerUtil.DefineSpace(ctx, 1, false, testNVRAMIndex, []string{hwsec.NVRAMAttributeWriteAuth}, testPassword); err != nil {
		s.Fatal("Failed to create NVRAM space: ", err)
	}
	// Cleanup by removing the NVRAM space.
	defer func() {
		if _, err := tpmManagerUtil.DestroySpace(ctx, testNVRAMIndex); err != nil {
			s.Error("Failed to destroy NVRAM space: ", err)
		}
	}()

	// Create a 1 byte file for writing
	if _, err := cmdRunner.Run(ctx, "dd", "if=/dev/zero", "of="+testFilePath, "bs=1", "count=1"); err != nil {
		s.Fatal("Failed to create test file: ", err)
	}
	defer func() {
		if _, err := cmdRunner.Run(ctx, "rm", "-f", testFilePath); err != nil {
			s.Error("Failed to cleanup tmp file: ", err)
		}
	}()

	// Try to write the NVRAM space with incorrect password.
	if _, err := tpmManagerUtil.WriteSpaceFromFile(ctx, testNVRAMIndex, testFilePath, testIncorrectPassword); err == nil {
		// It succeeded when it shouldn't.
		s.Fatal("Write NVRAM Space succeeded with incorrect password")
	}

	// Check counter again, should be 1.
	counter, _, inEffect, _, err = getDAInfo(ctx, cryptohomeUtil, tpmManagerUtil)
	if err != nil {
		s.Fatal("Failed to get dictionary attack info: ", err)
	}
	if counter != 1 {
		s.Fatalf("Incorrect counter, got %d expect 1", counter)
	}

	// Now try to reset the dictionary attack counter.
	if _, err := tpmManagerUtil.ResetDALock(ctx); err != nil {
		s.Fatal("Failed to reset dictionary attack lockout: ", err)
	}

	// Check counter again, should be 0, and lockout shouldn't be in effect.
	counter, _, inEffect, _, err = getDAInfo(ctx, cryptohomeUtil, tpmManagerUtil)
	if err != nil {
		s.Fatal("Failed to get dictionary attack info: ", err)
	}
	if counter != 0 {
		s.Fatalf("Incorrect counter, got %d expect 0", counter)
	}
	if inEffect {
		s.Fatal("Lockout in effect after reset")
	}
}
