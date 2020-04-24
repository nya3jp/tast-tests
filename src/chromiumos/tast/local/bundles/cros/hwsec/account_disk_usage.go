// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AccountDiskUsage,
		Desc: "Verifies that GetAccountDiskUsage API works as expected",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      3 * time.Minute,
	})
}

// setupVault will setup a user and its vault.
// Caller of this assumes the responsibility of umounting/cleaning up the vault regardless of whether the function returned an error.
func setupVault(ctx context.Context, s *testing.State, username, password, label string, utility *hwsec.UtilityCryptohomeBinary) error {
	// Now create the vault.
	if err := utility.MountVault(ctx, username, password, label, true, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to create user vault for testing")
	}
	// Note: Caller of this method is responsible for cleaning up the

	return nil
}

// testAccountUsage will test a given account and see if GetAccountDiskUsage() works correctly with it or not.
// Note that the account's home directory should be empty or nearly empty before calling this.
// This method doesn't return anything, it'll just output the error or abort the test.
func testAccountUsage(ctx context.Context, s *testing.State, cmdRunner hwsec.CmdRunner, username string, utility *hwsec.UtilityCryptohomeBinary) {
	const (
		// The size of the test file in MiB.
		testFileSize = 256
		// The margin we'll have for testing in MiB.
		// The test will pass if the result is +/- this margin.
		testFileMargin = 32
	)

	usageBefore, err := utility.GetAccountDiskUsage(ctx, username)
	if err != nil {
		s.Fatal("Failed to get the account disk usage before writing data: ", err)
	}

	testFilePath, err := util.GetUserTestFilePath(ctx, username, util.TestFileName)
	if err != nil {
		s.Fatal("Failed to get user test file path: ", err)
	}

	// Write a test file.
	// Note that we want the file to be random so that transparent filesystem
	// compression (if any is implemented) doesn't affect this test.
	// OpenSSL is used instead of /dev/urandom because it's much faster.
	if _, err := cmdRunner.Run(ctx, "sh", "-c", fmt.Sprintf("openssl enc -aes-128-ctr -pass pass:CrOS4tw -nosalt </dev/zero | dd of=%s bs=1M count=%d iflag=fullblock", shutil.Escape(testFilePath), testFileSize)); err != nil {
		s.Fatal("Failed to write the test file: ", err)
	}

	// Synchronize cached writes to persistent storage before we query again.
	if _, err := cmdRunner.Run(ctx, "sync"); err != nil {
		s.Fatal("Failed to synchronize cached writes to persistent storage: ", err)
	}

	usageAfter, err := utility.GetAccountDiskUsage(ctx, username)
	if err != nil {
		s.Fatal("Failed to get the account disk usage after writing data: ", err)
	}

	// *1024*1024 because 1MiB is 1024*1024 bytes.
	expectedAfter := usageBefore + testFileSize*1024*1024
	expectedAfterUpperLimit := expectedAfter + testFileMargin*1024*1024
	expectedAfterLowerLimit := expectedAfter - testFileMargin*1024*1024
	if expectedAfterLowerLimit > usageAfter || expectedAfterUpperLimit < usageAfter {
		// Convert the unit to MiB so that it's easier to read.
		usageAfterFloat := float64(usageAfter) / (1024 * 1024)
		expectedAfterLowerLimitFloat := float64(expectedAfterLowerLimit) / (1024 * 1024)
		expectedAfterUpperLimitFloat := float64(expectedAfterUpperLimit) / (1024 * 1024)
		s.Errorf("Disk usage for user %q after writing data is out of range, got %g MiB, want %g MiB ~ %g Mib", username, usageAfterFloat, expectedAfterLowerLimitFloat, expectedAfterUpperLimitFloat)
	}
}

func AccountDiskUsage(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}

	utility, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}

	// Cleanup before we start.
	if err := utility.UnmountAll(ctx); err != nil {
		s.Log("Fails to unmount all before test starts: ", err)
	}
	if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
		s.Log("Fails to remove vault before test starts: ", err)
	}

	err = setupVault(ctx, s, util.FirstUsername, util.FirstPassword, util.PasswordLabel, utility)
	defer func() {
		// Remember to logout and delete vault.
		if err := utility.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to logout during cleanup: ", err)
		}
		if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to cleanup after the test: ", err)
		}
	}()
	if err != nil {
		s.Fatal("Failed to setup vault and user as owner: ", err)
	}

	testAccountUsage(ctx, s, cmdRunner, util.FirstUsername, utility)
}
