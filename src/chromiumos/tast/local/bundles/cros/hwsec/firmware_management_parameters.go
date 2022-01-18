// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

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

func init() {
	testing.AddTest(&testing.Test{
		Func: FirmwareManagementParameters,
		Desc: "Verifies that FirmwareManagementParameters are working correctly",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// checkFWMPCleared checks that FWMP is cleared, and returns nil iff it is cleared.
func checkFWMPCleared(ctx context.Context, utility *hwsec.CryptohomeClient) error {
	flags, hash, err := utility.GetFirmwareManagementParameters(ctx)

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
func clearFWMPAndCheck(ctx context.Context, utility *hwsec.CryptohomeClient) error {
	if _, err := utility.RemoveFirmwareManagementParameters(ctx); err != nil {
		return errors.Wrap(err, "failed to clear fwmp")
	}

	// Note that the reason why we are checking if FWMP is cleared after a
	// successful call to RemoveFirmwareManagementParameters is we want to
	// verify RemoveFirmwareManagementParameters actually does remove FWMP.
	// i.e. We want to catch cases whereby RemoveFirmwareManagementParameters
	// succeeded but it wasn't cleared.
	if err := checkFWMPCleared(ctx, utility); err != nil {
		return errors.Wrap(err, "failed to check fwmp is cleared")
	}

	return nil
}

// checkFWMPSet checks that FWMP is set to the expected values.
func checkFWMPSet(ctx context.Context, utility *hwsec.CryptohomeClient, expectedFlags, expectedHash string) error {
	flags, hash, err := utility.GetFirmwareManagementParameters(ctx)
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
func setFWMPAndCheck(ctx context.Context, utility *hwsec.CryptohomeClient, flags, hash string) error {
	if _, err := utility.SetFirmwareManagementParameters(ctx, flags, hash); err != nil {
		return errors.Wrap(err, "failed to set FWMP")
	}

	// Note that the reason why we are checking if FWMP is set after a
	// successful call to SetFirmwareManagementParameters is we want to
	// verify SetFirmwareManagementParameters actually does set FWMP.
	// i.e. We want to catch cases whereby SetFirmwareManagementParameters
	// succeeded but it wasn't set.
	if err := checkFWMPSet(ctx, utility, flags, hash); err != nil {
		return errors.Wrap(err, "failed to check fwmp is set correctly")
	}

	return nil
}

// FirmwareManagementParameters checks that the firmware management parameters are functioning correctly.
func FirmwareManagementParameters(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	utility := helper.CryptohomeClient()

	// First backup the current FWMP so the test doesn't affect what's on DUT.
	fwmp, err := utility.BackupFWMP(ctx)
	if err != nil {
		s.Fatal("Failed to backup FWMP: ", err)
	}
	// Remember to restore it at the end.
	defer func() {
		if err := utility.RestoreFWMP(ctx, fwmp); err != nil {
			s.Error("Failed to restore FWMP: ", err)
		}
	}()

	// Clear FWMP before the start of the test.
	if err := clearFWMPAndCheck(ctx, utility); err != nil {
		s.Fatal("Failed to clear FWMP at the start of the test: ", err)
	}

	// Now try to set it with the first value, then read it back to check.
	if err := setFWMPAndCheck(ctx, utility, testFlags1, testHash1); err != nil {
		s.Fatal("Failed to set FWMP with test case 1: ", err)
	}

	// Clear the FWMP to make sure it can be cleared.
	if err := clearFWMPAndCheck(ctx, utility); err != nil {
		s.Fatal("Failed to clear FWMP after setting the first test case: ", err)
	}

	// Test again with the second test case.
	if err := setFWMPAndCheck(ctx, utility, testFlags2, testHash2); err != nil {
		s.Fatal("Failed to set FWMP with test case 2: ", err)
	}
}
