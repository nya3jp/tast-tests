// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
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

func AccountDiskUsage(ctx context.Context, s *testing.State) {
	const (
		// The size of the test file in MiB.
		testFileSize = 256
		// The margin we'll have for testing in MiB.
		// The test will pass if the result is +/- this margin.
		testFileMargin = 32
	)

	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}

	utility, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}

	// Cleanup before we start. Note that results are intentionally ignored because we are not sure if there's something for us to cleanup.
	utility.UnmountAll(ctx)
	utility.RemoveVault(ctx, util.FirstUsername)

	// Now create the vault.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true); err != nil {
		s.Fatal("Failed to create user vault for testing: ", err)
	}
	defer func() {
		// Remember to logout and delete vault.
		if err := utility.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to logout during cleanup: ", err)
		}
		if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to cleanup after the test: ", err)
		}
	}()

	var usageBefore int64
	if usageBefore, err = utility.GetAccountDiskUsage(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to get the account disk usage before writing data: ", err)
	}

	testFilePath, err := util.GetUserTestFilePath(ctx, util.FirstUsername, util.TestFileName)
	if err != nil {
		s.Fatal("Failed to get user test file path: ", err)
	}

	// Write a 64 MiB test file.
	// Note that we want the file to be random so that transparent filesystem
	// compression (if any is implemented) doesn't affect this test.
	// OpenSSL is used instead of /dev/urandom because it's much faster.
	if _, err := cmdRunner.Run(ctx, "sh", "-c", fmt.Sprintf("openssl enc -aes-128-ctr -pass pass:CrOS4tw -nosalt </dev/zero | dd 'of=%s' bs=1M count=%d iflag=fullblock", testFilePath, testFileSize)); err != nil {
		s.Fatal("Failed to write the test file: ", err)
	}

	// Synchronize cached writes to persistent storage before we query again.
	if _, err := cmdRunner.Run(ctx, "sync"); err != nil {
		s.Fatal("Failed to synchronize cached writes to persistent storage: ", err)
	}

	var usageAfter int64
	if usageAfter, err = utility.GetAccountDiskUsage(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to get the account disk usage after writing data: ", err)
	}

	expectedAfter := usageBefore + testFileSize*1024*1024
	expectedAfterUpperLimit := expectedAfter + testFileMargin*1024*1024
	expectedAfterLowerLimit := expectedAfter - testFileMargin*1024*1024
	if expectedAfterLowerLimit > usageAfter || expectedAfterUpperLimit < usageAfter {
		// Convert the unit to MiB so that it's easier to read.
		usageAfterFloat := float64(usageAfter) / (1024.0 * 1024.0)
		expectedAfterLowerLimitFloat := float64(expectedAfterLowerLimit) / (1024.0 * 1024.0)
		expectedAfterUpperLimitFloat := float64(expectedAfterUpperLimit) / (1024.0 * 1024.0)
		s.Fatalf("Disk usage after writing data is out of range, got %g MiB, want %g MiB ~ %g Mib", usageAfterFloat, expectedAfterLowerLimitFloat, expectedAfterUpperLimitFloat)
	}
}
