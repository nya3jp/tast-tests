// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// IncreaseDAForTpm1 uses tpm_manager_client to increase the dictionary attack counter, and should be only used on TPMv1.2 devices.
func IncreaseDAForTpm1(ctx context.Context, tpmManager *hwsec.TPMManagerClient) error {
	const testNVRAMIndex = "0x1000F000" // Endorsement cert in TPMv1.2, it's permanent.
	const testIncorrectPassword = "4321"

	// Prepare a test file.
	testFile, err := ioutil.TempFile("", "dictionary_attack_test")
	if err != nil {
		return errors.Wrap(err, "failed to create temp file: ")
	}
	ctx, _ = ctxutil.Shorten(ctx, time.Second) // Give it a second for cleanup.
	defer func() {
		// We setup the cleanup earlier than the read operation because if the read operation succeed and
		// the test fails, we still need to cleanup the file.
		if err := os.Remove(testFile.Name()); err != nil {
			testing.ContextLog(ctx, "Failed to remove tmp file: ", err)
		}
	}()
	testFilePath, err := filepath.Abs(testFile.Name())
	if err != nil {
		return errors.Wrap(err, "failed to get absolute path of temp file: ")
	}
	if err := testFile.Close(); err != nil {
		return errors.Wrap(err, "failed to close the temp file: ")
	}

	// Try to write the NVRAM space with incorrect password to increase the counter.
	if _, err := tpmManager.ReadSpaceToFile(ctx, testNVRAMIndex, testFilePath, testIncorrectPassword); err == nil {
		return errors.New("Reading NVRAM Space should not succeed with incorrect password")
	}

	return nil
}

// CheckDAIsZero uses tpm_manager_client to check if the dictionary attack counter is zero.
func CheckDAIsZero(ctx context.Context, tpmManager *hwsec.TPMManagerClient) error {
	info, err := tpmManager.GetDAInfo(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get dictionary attack info: ")
	}
	if info.Counter != 0 {
		return errors.Errorf("Incorrect counter: got %d, want 0", info.Counter)
	}
	if info.InEffect {
		return errors.New("Lockout in effect after reset")
	}
	return nil
}
