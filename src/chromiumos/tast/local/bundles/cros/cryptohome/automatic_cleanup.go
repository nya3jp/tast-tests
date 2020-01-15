// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"

	"chromiumos/tast/local/bundles/cros/cryptohome/disk"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AutomaticCleanup,
		Desc: "Test automatic disk cleanup",
		Contacts: []string{
			"vsavu@google.com",     // Test author
			"gwendal@chromium.com", // Lead for Chrome OS Storage
			"chromeos-commercial-stability@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func AutomaticCleanup(ctx context.Context, s *testing.State) {
	const (
		homedirSize       = 900 * disk.MiB                    // 900 Mib, used for testing
		startingFreeSpace = disk.CleanupTarget + 200*disk.MiB // 2.2 GiB, used for testing

		temporaryUser = "tmp-user"
		user1         = "cleanup-user1"
		user2         = "cleanup-user2"
		password      = "1234"
	)

	// Start cryptohomed and wait for it to be available
	if err := upstart.EnsureJobRunning(ctx, "cryptohomed"); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohomed not running as expected: ", err)
	}
	defer upstart.RestartJob(ctx, "cryptohomed")

	if err := disk.CleanupExistingUsers(ctx); err != nil {
		s.Fatal("Failed to perform initial cleanup: ", err)
	}

	// Stay above trigger for cleanup
	fillFile, err := disk.FillUntil(disk.UserHome, startingFreeSpace)
	if err != nil {
		s.Fatal("Failed to fill space: ", err)
	}
	defer os.Remove(fillFile)

	if freeSpace, err := disk.FreeSpace(disk.UserHome); err != nil {
		s.Fatal("Failed get free space: ", err)
	} else if freeSpace < 2*homedirSize { // Sanity check
		s.Fatal("Too little free space is available: ", freeSpace)
	} else {
		s.Logf("%v bytes available after fill", freeSpace)
	}

	// Create users with contents to fill up disk space
	fillFile1, err := disk.CreateFilledUserHomedir(ctx, user1, password, "Cache", homedirSize)
	if err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, user1)

	fillFile2, err := disk.CreateFilledUserHomedir(ctx, user2, password, "Cache", homedirSize)
	if err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, user2)

	// Make sure disk space is low
	freeSpace, err := disk.FreeSpace(disk.UserHome)
	if err != nil {
		s.Fatal("Failed get free space: ", err)
	} else if freeSpace > disk.MinimalFreeSpace {
		s.Errorf("Space was not filled, %v available", freeSpace)
	}

	// Unmount just the first user
	if err := cryptohome.UnmountVault(ctx, user1); err != nil {
		s.Fatal("Failed to unmount user vault: ", err)
	}

	// Remount the second user
	if err := cryptohome.CreateVault(ctx, user2, password); err != nil {
		s.Fatal("Failed to remount user vault: ", err)
	}

	if err := cryptohome.WaitForUserMount(ctx, user2); err != nil {
		s.Fatal("Failed to remount user vault: ", err)
	}

	// Keep file reference to prevent unmount on restart
	file, err := os.Open(fillFile2)
	if err != nil {
		s.Fatal("Failed to open file: ", err)
	}
	defer cryptohome.UnmountVault(ctx, user2)
	defer file.Close()

	if err := disk.RunAutomaticCleanup(ctx); err != nil {
		s.Fatal("Failed to run cleanup: ", err)
	}

	if _, err := os.Stat(fillFile1); err == nil {
		s.Error("fillFile for user1 still present")
	} else if !os.IsNotExist(err) {
		s.Fatal("Failed to check if fill file exists: ", err)
	}

	if _, err := os.Stat(fillFile2); err != nil {
		if os.IsNotExist(err) {
			s.Error("fillFile for user2  was removed")
		} else {
			s.Fatal("Failed to check if fill file exists: ", err)
		}
	}

	if freeSpace, err := disk.FreeSpace(disk.UserHome); err != nil {
		s.Fatal("Failed get free space: ", err)
	} else {
		s.Logf("%v bytes available after cleanup", freeSpace)
	}
}
