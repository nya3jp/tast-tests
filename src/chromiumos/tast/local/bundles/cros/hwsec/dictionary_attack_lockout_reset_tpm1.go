// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/hwsec/dictattack"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// NOTE: This test is somewhat similar to hwsec.DictionaryAttackLockoutResetTPM2 (a local test), if change is
// made to one, it is likely that the other have to be changed as well.
// The referred test is specifically for TPMv2.0, while this test is for TPMv1.2.
// Both versions of TPM is incompatible with each other and the available NVRAM index differs across the 2 versions.
// Therefore, we need 2 versions of the test.
// This version uses existing NVRAM space (endorsement cert) on TPMv1.2. Reading it with incorrect auth on
// TPMv1.2 will generate dictionary attack counter increment needed by this test. However, on TPMv2.0, this
// behaviour is different so the same method is not used in the other test.

func init() {
	testing.AddTest(&testing.Test{
		Func: DictionaryAttackLockoutResetTPM1,
		Desc: "Verifies that for TPMv1.2 devices, dictionary attack counter functions correctly and can be reset",
		Contacts: []string{
			"zuan@chromium.org",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"tpm1"},
		Attr:         []string{"group:mainline"},
	})
}

// DictionaryAttackLockoutResetTPM1 checks that get dictionary attack info and reset dictionary attack lockout works as expected.
func DictionaryAttackLockoutResetTPM1(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	cryptohomeUtil, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}
	tpmManagerUtil, err := hwsec.NewUtilityTPMManagerBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityTPMManagerBinary: ", err)
	}

	// In this test, we want to check if DA counter increases, and then reset it to see if everything is correct.
	// Reset DA Lockout => Check DA Counter => Read NVRAM Index with incorrect password =>
	// Check DA Counter => Reset DA Lockout => Check DA Counter.
	// Read NVRAM Index with incorrect password is used to trigger an increase in DA counter.

	// Reset DA at the start of the test.
	if _, err := tpmManagerUtil.ResetDALock(ctx); err != nil {
		s.Fatal("Failed to reset dictionary attack lockout: ", err)
	}

	// Check that the counter is 0 right after resetting.
	info, err := dictattack.DAInfo(ctx, cryptohomeUtil, tpmManagerUtil)
	if err != nil {
		s.Fatal("Failed to get dictionary attack info: ", err)
	}
	if info.Counter != 0 {
		s.Fatalf("Incorrect counter: got %d, want 0", info.Counter)
	}
	if info.InEffect {
		s.Fatal("Lockout in effect after reset")
	}

	const testNVRAMIndex = "0x1000F000" // Endorsement cert in TPMv1.2, it's permanent.
	const testIncorrectPassword = "4321"

	// Prepare a test file.
	testFile, err := ioutil.TempFile("", "dictionary_attack_test")
	if err != nil {
		s.Fatal("Failed to create temp file: ", err)
	}
	ctx, _ = ctxutil.Shorten(ctx, time.Second) // Give it a second for cleanup.
	defer func() {
		// We setup the cleanup earlier than the read operation because if the read operation succeed and
		// the test fails, we still need to cleanup the file.
		if err := os.Remove(testFile.Name()); err != nil {
			s.Error("Failed to remove tmp file: ", err)
		}
	}()
	testFilePath, err := filepath.Abs(testFile.Name())
	if err != nil {
		s.Fatal("Failed to get absolute path of temp file: ", err)
	}
	if err := testFile.Close(); err != nil {
		s.Fatal("Failed to close the temp file: ", err)
	}

	// Try to write the NVRAM space with incorrect password to increase the counter.
	if _, err := tpmManagerUtil.ReadSpaceToFile(ctx, testNVRAMIndex, testFilePath, testIncorrectPassword); err == nil {
		s.Fatal("Reading NVRAM Space should not succeed with incorrect password")
	}

	// Check counter again, should be 1 because we tried to write NVRAM space with an incorrect password.
	info, err = dictattack.DAInfo(ctx, cryptohomeUtil, tpmManagerUtil)
	if err != nil {
		s.Fatal("Failed to get dictionary attack info: ", err)
	}
	if info.Counter != 1 {
		s.Fatalf("Incorrect counter: got %d, want 1", info.Counter)
	}

	// Now try to reset the dictionary attack lockout counter.
	if _, err := tpmManagerUtil.ResetDALock(ctx); err != nil {
		s.Fatal("Failed to reset dictionary attack lockout: ", err)
	}

	// Check counter again, should be 0, and lockout shouldn't be in effect.
	info, err = dictattack.DAInfo(ctx, cryptohomeUtil, tpmManagerUtil)
	if err != nil {
		s.Fatal("Failed to get dictionary attack info: ", err)
	}
	if info.Counter != 0 {
		s.Fatalf("Incorrect counter: got %d, want 0", info.Counter)
	}
	if info.InEffect {
		s.Fatal("Lockout in effect after reset")
	}
}
