// Copyright 2022 The ChromiumOS Authors
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

// SetupP11TestToken configures a PKCS #11 database in scratchpadPath.
func SetupP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string) error {
	if err := CleanupP11TestToken(ctx, r, scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to cleanup PKCS11 test token")
	}
	if err := os.MkdirAll(scratchpadPath, 0750); err != nil {
		return errors.Wrap(err, "failed to create scratchpad")
	}
	if _, err := r.Run(ctx, "chown", "chaps:chronos-access", scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to change owner")
	}
	return nil
}

// LoadP11TestToken loads the test token onto a slot stored in scratchpadPath and return the slot number.
// LoadP11TestToken returns an invalid slot number that is larger than the max slot chapsd supports with non-nil error when it fails.
func LoadP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath, authData string) (string, error) {
	errSlot := "4294967296"
	if _, err := r.Run(ctx, "chaps_client", "--load", "--path="+scratchpadPath, "--auth="+authData); err != nil {
		return errSlot, errors.Wrap(err, "failed to load PKCS11 token")
	}

	// The output of chaps_client goes to stderr, so use RunWithCombinedOutput here.
	lines, err := r.RunWithCombinedOutput(ctx, "chaps_client", "--list")
	if err != nil {
		return errSlot, errors.Wrap(err, "failed to list token path")
	}
	scanner := bufio.NewScanner(bytes.NewReader(lines))
	re := regexp.MustCompile(`Slot (\d+): ` + scratchpadPath)
	for scanner.Scan() {
		matches := re.FindStringSubmatch(scanner.Text())
		if len(matches) > 0 {
			return matches[1], nil
		}
	}
	return errSlot, errors.New("failed to find slot")
}

// UnloadP11TestToken unloads loaded test token stored in scratchpadPath.
func UnloadP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string) error {
	if _, err := r.Run(ctx, "chaps_client", "--unload", "--path="+scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to unload PKCS11 token")
	}
	return nil
}

// CleanupP11TestToken deletes the test token stored in scratchpadPath.
func CleanupP11TestToken(ctx context.Context, r hwsec.CmdRunner, scratchpadPath string) error {
	if err := UnloadP11TestToken(ctx, r, scratchpadPath); err != nil {
		// Test goes on but record this event.
		testing.ContextLog(ctx, "Could not unload token")
	}
	if _, err := r.Run(ctx, "rm", "-rf", scratchpadPath); err != nil {
		return errors.Wrap(err, "failed to remove the scratchpad directory")
	}
	return nil
}
