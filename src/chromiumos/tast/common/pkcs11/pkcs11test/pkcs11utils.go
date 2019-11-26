// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pkcs11test

import (
	"context"
	"fmt"
	"path/filepath"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/errors"
)

// PrepareScratchpadAndTestFiles prepares the scratchpad space at ScratchpadPath by ensuring that it is empty before the test, and exists after the call. Also, this creates 2 test files in it for testing.
// The path to the 2 test files are returned, and err is nil iff the operation is successful.
// This is usually called at the start of pkcs#11 related tests.
func PrepareScratchpadAndTestFiles(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string) (testfile1, testfile2 string, returnedError error) {
	// Check that the scratchpad is empty/doesn't exist.
	if _, err := r.Run(ctx, "ls", scratchpadPath); err == nil {
		return "", "", errors.New("scratchpad is not empty")
	}

	// Prepare the scratchpad.
	if _, err := r.Run(ctx, "mkdir", "-p", scratchpadPath); err != nil {
		return "", "", errors.Wrap(err, "failed to create scratchpad")
	}

	// Prepare the test files.
	f1 := filepath.Join(scratchpadPath, "testfile1.txt")
	if _, err := r.Run(ctx, "sh", "-c", fmt.Sprintf("echo test1 > %s", f1)); err != nil {
		return "", "", errors.Wrap(err, "failed to create test file 1")
	}
	f2 := filepath.Join(scratchpadPath, "testfile2.txt")
	if _, err := r.Run(ctx, "sh", "-c", fmt.Sprintf("echo test2 > %s", f2)); err != nil {
		return "", "", errors.Wrap(err, "failed to create test file 2")
	}

	return f1, f2, nil
}

// CleanupScratchpad removes the scratchpad at ScratchpadPath. This is usually called at the end of the test.
func CleanupScratchpad(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string) error {
	if _, err := r.Run(ctx, "rm", "-rf", scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to remove the scratchpad")
	}
	return nil
}

// SignAndVerify is just a convenient runner to test both signing and verification.
// altInput is path to another test file that differs in content to input. It is used to check that verify() indeed reject corrupted input.
func SignAndVerify(ctx context.Context, p *pkcs11.Util, key *pkcs11.KeyInfo, input string, altInput string, mechanism *pkcs11.MechanismInfo) error {
	// Test signing.
	if err := key.Sign(ctx, p, input, input+".sig", mechanism); err != nil {
		return err
	}
	// Test verification of signed message.
	if err := key.Verify(ctx, p, input, input+".sig", mechanism); err != nil {
		return err
	}
	// Test verification of another message (should fail).
	if err := key.Verify(ctx, p, altInput, input+".sig", mechanism); err == nil {
		// Should not happen.
		return errors.Errorf("verification functionality for %s failed, corrupted message is verified", mechanism.Name)
	}
	return nil
}
