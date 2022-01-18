// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FWMPAcrossTPMClear,
		Desc: "Verifies that FirmwareManagementParameters are working correctly across TPM clear",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@google.com",
		},
		SoftwareDeps: []string{"reboot", "tpm", "gsc"},
		Attr:         []string{"group:hwsec_destructive_func"},
		Timeout:      5 * time.Minute,
	})
}

const (
	// fwmpRemovedErrorCode is the error code returned by GetFirmwareManagementParameters when the FWMP is removed.
	fwmpRemovedErrorCode = "CRYPTOHOME_ERROR_FIRMWARE_MANAGEMENT_PARAMETERS_INVALID"

	testFlags1   = "00000006" // FWMP_DEV_DISABLE_RECOVERY | FWMP_DEV_ENABLE_USB
	testFlags2   = "0000000c" // FWMP_DEV_ENABLE_USB | FWMP_DEV_ENABLE_LEGACY
	clearedFlags = "00000000"

	testHash1   = "0123456789abcdef9876543210abcdef0123456789abcdef9876543210abcdef"
	testHash2   = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	clearedHash = "0000000000000000000000000000000000000000000000000000000000000000"
)

// checkFWMPCleared checks that FWMP is cleared, and returns nil iff it is cleared.
func checkFWMPCleared(ctx context.Context, cryptohome *hwsec.CryptohomeClient) error {
	flags, hash, err := cryptohome.GetFirmwareManagementParameters(ctx)

	// There are 2 possible good results, depending on the coreboot and cryptohome implementation.
	// Case 1: If the the FWMP index is owner-defined, invalid space is expected after the clear.
	// Case 2: Otherwise, the FWMP index is platform-defined, which will not be deleted by cryptohome. Instead, cleared flag and hash is expected.
	if err != nil { // Case 1.
		if err.ErrorCode != fwmpRemovedErrorCode {
			return errors.Errorf("call to GetFirmwareManagementParameters failed with an incorrect error code, got %q, want %q", err.ErrorCode, fwmpRemovedErrorCode)
		}
	} else { // Case 2.
		if flags != clearedFlags || hash != clearedHash {
			return errors.Errorf("flags or hash not cleared (expecting all 0s); flags: %q, hash: %q", flags, hash)
		}
	}
	return nil
}

// clearFWMPAndCheck clears FWMP and checks that it's cleared correctly. It return nil iff FWMP is successfully cleared.
func clearFWMPAndCheck(ctx context.Context, cryptohome *hwsec.CryptohomeClient) error {
	if _, err := cryptohome.RemoveFirmwareManagementParameters(ctx); err != nil {
		return errors.Wrap(err, "failed to clear fwmp")
	}

	// Note that the reason why we are checking if FWMP is cleared after a
	// successful call to RemoveFirmwareManagementParameters is we want to
	// verify RemoveFirmwareManagementParameters actually does remove FWMP.
	// i.e. We want to catch cases whereby RemoveFirmwareManagementParameters
	// succeeded but it wasn't cleared.
	if err := checkFWMPCleared(ctx, cryptohome); err != nil {
		return errors.Wrap(err, "failed to check fwmp is cleared")
	}

	return nil
}

// checkFWMPSet checks that FWMP is set to the expected values.
func checkFWMPSet(ctx context.Context, cryptohome *hwsec.CryptohomeClient, expectedFlags, expectedHash string) error {
	flags, hash, err := cryptohome.GetFirmwareManagementParameters(ctx)
	if err != nil {
		return errors.Wrap(err, "call to GetFirmwareManagementParameters failed when trying to check FWMP is set correctly")
	}

	if flags != expectedFlags {
		return errors.Errorf("flags are incorrect when checking FWMP is set correctly, got %q, want %q", flags, expectedFlags)
	}

	if hash != expectedHash {
		return errors.Errorf("hash is incorrect when checking FWMP is set correctly, got %q, want %q", hash, expectedHash)
	}

	return nil
}

// setFWMPAndCheck sets the FWMP and checks that it's set correctly. It return nil iff FWMP is successfully set.
func setFWMPAndCheck(ctx context.Context, cryptohome *hwsec.CryptohomeClient, flags, hash string) error {
	if _, err := cryptohome.SetFirmwareManagementParameters(ctx, flags, hash); err != nil {
		return errors.Wrap(err, "failed to set FWMP")
	}

	// Note that the reason why we are checking if FWMP is set after a
	// successful call to SetFirmwareManagementParameters is we want to
	// verify SetFirmwareManagementParameters actually does set FWMP.
	// i.e. We want to catch cases whereby SetFirmwareManagementParameters
	// succeeded but it wasn't set.
	if err := checkFWMPSet(ctx, cryptohome, flags, hash); err != nil {
		return errors.Wrap(err, "failed to check fwmp is set correctly")
	}

	return nil
}

// FWMPAcrossTPMClear checks that the firmware management parameters are functioning correctly across TPM clear.
func FWMPAcrossTPMClear(ctx context.Context, s *testing.State) {
	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())

	helper, err := hwsecremote.NewHelper(cmdRunner, s.DUT())
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	cryptohome := helper.CryptohomeClient()

	// Resets the TPM states before running the tests.
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	// Clear FWMP before the start of the test.
	if err := clearFWMPAndCheck(ctx, cryptohome); err != nil {
		s.Fatal("Failed to clear FWMP at the start of the test: ", err)
	}

	// Now try to set it with the first value, then read it back to check.
	if err := setFWMPAndCheck(ctx, cryptohome, testFlags1, testHash1); err != nil {
		s.Fatal("Failed to set FWMP with test case 1: ", err)
	}

	// Clear the FWMP to make sure it can be cleared.
	if err := clearFWMPAndCheck(ctx, cryptohome); err != nil {
		s.Fatal("Failed to clear FWMP after setting the first test case: ", err)
	}

	// Test again with the second test case.
	if err := setFWMPAndCheck(ctx, cryptohome, testFlags2, testHash2); err != nil {
		s.Fatal("Failed to set FWMP with test case 2: ", err)
	}

	// Reboot the DUT.
	if err := helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT: ", err)
	}

	// Ensure the FWMP still there after reboot.
	if err := checkFWMPSet(ctx, cryptohome, testFlags2, testHash2); err != nil {
		s.Fatal("Failed to check the second FWMP after reboot the DUT: ", err)
	}

	// Resets the TPM states.
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}

	// Ensure the FWMP still there after reset the TPM.
	if err := checkFWMPSet(ctx, cryptohome, testFlags2, testHash2); err != nil {
		s.Fatal("Failed to check the second FWMP after reset the TPM: ", err)
	}

	// Ensure TPM is ready.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	// Ensure the FWMP still there after TPM is ready.
	if err := checkFWMPSet(ctx, cryptohome, testFlags2, testHash2); err != nil {
		s.Fatal("Failed to check the second FWMP after TPM is ready: ", err)
	}
}
