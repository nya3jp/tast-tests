// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package pkcs11test

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"regexp"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// SetupP11TestToken configures a PKCS #11 database in scratchpadPath and return the slot number.
func SetupP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string) (string, error) {
	// If anything goes wrong, return an invalid slot number that is larger than the max slot chapsd supports
	errSlot := "4294967296"
	if CleanupP11TestToken(ctx, r, scratchpadPath) != nil {
		// Test goes on but record this event.
		testing.ContextLog(ctx, "Failed to cleanup PKCS11 test token")
	}
	if err := os.MkdirAll(scratchpadPath, 0750); err != nil {
		return errSlot, errors.Wrap(err, "failed to create scratchpad")
	}
	if _, err := r.Run(ctx, "chown", "chaps:chronos-access", scratchpadPath); err != nil {
		return errSlot, errors.Wrap(err, "failed to change owner")
	}
	// Use load token to create a user slot
	if LoadP11TestToken(ctx, r, scratchpadPath, "fake_auth") != nil {
		testing.ContextLog(ctx, "Failed to load PKCS11 test token")
	}

	// The output of chaps_client goes to stderr, so use RunWithCombinedOutput here
	lines, err := r.RunWithCombinedOutput(ctx, "chaps_client", "--list")
	if err != nil {
		return errSlot, errors.Wrap(err, "failed to list token path")
	}
	scanner := bufio.NewScanner(bytes.NewReader(lines))
	re := regexp.MustCompile(`Slot (\d+): ` + scratchpadPath)
	for scanner.Scan() {
		matches := re.FindStringSubmatch(scanner.Text())
		if len(matches) > 0 {
			// Unload token which is used to get the slot number but not for testing
			if UnloadP11TestToken(ctx, r, scratchpadPath) != nil {
				testing.ContextLog(ctx, "Failed to unload PKCS11 test token")
			}
			return matches[1], nil
		}
	}
	return errSlot, errors.New("failed to find slot")
}

// LoadP11TestToken loads the test token onto a slot stored in scratchpadPath.
func LoadP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath, authData string) error {
	if _, err := r.Run(ctx, "chaps_client", "--load", "--path="+scratchpadPath, "--auth="+authData); err != nil {
		return errors.Wrap(err, "failed to load PKCS11 token")
	}
	return nil
}

// UnloadP11TestToken unloads loaded test token stored in scratchpadPath.
func UnloadP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string) error {
	if _, err := r.Run(ctx, "chaps_client", "--unload", "--path="+scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to unload PKCS11 token")
	}
	return nil
}

// ChangeP11TestTokenAuthData changes authorization data auth_data by new_auth_data stored in scratchpadPath.
func ChangeP11TestTokenAuthData(ctx context.Context, r hwsec.CmdRunner, scratchpadPath, authData, newAuthData string) error {
	if _, err := r.Run(ctx, "chaps_client", "--load", "--path="+scratchpadPath, "--auth="+authData, "--new_auth="+newAuthData); err != nil {
		return errors.Wrap(err, "failed to change PKCS11 token auth data")
	}
	return nil
}

// CleanupP11TestToken deletes the test token stored in scratchpadPath.
func CleanupP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string) error {
	if UnloadP11TestToken(ctx, r, scratchpadPath) != nil {
		testing.ContextLog(ctx, "Could not unload token")
	}
	if _, err := r.Run(ctx, "rm", "-rf", scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to remove the scratchpad directory")
	}
	return nil
}
