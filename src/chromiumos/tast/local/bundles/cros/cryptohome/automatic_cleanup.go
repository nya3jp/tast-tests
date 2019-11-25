// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const userHome = "/home/user"

const mib uint64 = 1024 * 1024                     // 1 MiB
const gib uint64 = 1024 * mib                      // 1 GiB
const minimalFreeSpace = 512 * mib                 // hard-coded in cryptohomed
const cleanupTrigger = 2 * gib                     // hard-coded in cryptohomed
const startingFreeSpace = cleanupTrigger + 200*mib // 2.2 GiB, used for testing

func init() {
	testing.AddTest(&testing.Test{
		Func: AutomaticCleanup,
		Desc: "Test automatic disk cleanup",
		Contacts: []string{
			"vsavu@chromium.org", // Test author
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func createUserHomeAndFill(ctx context.Context, user, pass, fillDirectory string, clearedDirectories []string, size uint64) error {
	if err := cryptohome.CreateVault(ctx, user, pass); err != nil {
		return errors.Wrap(err, "failed to create user vault")
	}

	hash, err := cryptohome.UserHash(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to get user hash")
	}

	waitReady := func(dir string) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := os.Stat(filepath.Join(userHome, hash, dir)); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second})
	}

	// Create a small file to check later if cleared
	for _, dir := range clearedDirectories {
		if err := waitReady(dir); err != nil {
			return err
		}

		_, err = disk.Fill(filepath.Join(userHome, hash, dir), 10)
		if err != nil {
			return errors.Wrap(err, "failed to fill space")
		}
	}

	if err := waitReady(fillDirectory); err != nil {
		return err
	}

	_, err = disk.Fill(filepath.Join(userHome, hash, fillDirectory), size)
	if err != nil {
		return errors.Wrap(err, "failed to fill space")
	}

	return nil
}

func checkAutomaticCleanup(ctx context.Context, s *testing.State, fillDirectory string, clearedDirectories []string, threshold uint64) {
	// By using homedirSize bytes for each homedir we will be 100 MiB bellow the threshold
	homedirSize := (startingFreeSpace-threshold)/2 + 50*mib

	if err := upstart.EnsureJobRunning(ctx, "cryptohomed"); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	// Wait for DBus to be available
	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	// Stay above trigger for cleanup
	fillFile, err := disk.FillUntil(userHome, startingFreeSpace)
	if err != nil {
		s.Fatal("Failed to fill space: ", err)
	}
	defer os.Remove(fillFile.Name())
	defer fillFile.Close()

	freeSpace, err := disk.FreeSpace(userHome)
	if err != nil {
		s.Fatal("Failed get free space: ", err)
	}

	if freeSpace < 2*homedirSize { // Sanity check
		s.Fatal("Too little free space is available: ", freeSpace)
	}

	s.Logf("%v bytes remaining", freeSpace)

	// Create users with contents
	if err := createUserHomeAndFill(ctx, "cleanup-user1", "1234", fillDirectory, clearedDirectories, homedirSize); err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, "cleanup-user1")

	if err := createUserHomeAndFill(ctx, "cleanup-user2", "1234", fillDirectory, clearedDirectories, homedirSize); err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, "cleanup-user2")

	// Make sure disk space is low
	freeSpace, err = disk.FreeSpace(userHome)
	if err != nil {
		s.Fatal("Failed get free space: ", err)
	}

	if freeSpace > threshold {
		s.Errorf("Space was not filled, %v available", freeSpace)
	}

	// Unmount just the first user
	if err := cryptohome.UnmountVault(ctx, "cleanup-user1"); err != nil {
		s.Fatal("Failed to unmount user vault: ", err)
	}

	if err := cryptohome.CreateVault(ctx, "cleanup-user2", "1234"); err != nil {
		s.Fatal("Failed to remount user vault: ", err)
	}

	if err := cryptohome.WaitForUserMount(ctx, "cleanup-user2"); err != nil {
		s.Fatal("Failed to remount user vault: ", err)
	}

	// Keep file reference to prevent unmount on restart
	hash, err := cryptohome.UserHash(ctx, "cleanup-user2")
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}

	files, err := ioutil.ReadDir(filepath.Join(userHome, hash, "Cache"))
	if err != nil {
		s.Fatal("Failed to read directory: ", err)
	}

	if len(files) == 0 {
		s.Fatalf("Directory %s empty", filepath.Join(userHome, hash, "Cache"))
	}

	file, err := os.Open(filepath.Join(userHome, hash, "Cache", files[0].Name()))
	if err != nil {
		s.Fatal("Failed to open file: ", err)
	}
	defer cryptohome.UnmountVault(ctx, "cleanup-user2")
	defer file.Close()

	// Restart to trigger cleanup
	if err := upstart.RestartJob(ctx, "cryptohomed"); err != nil {
		s.Fatal("Failed to restart cryptohomed: ", err)
	}

	s.Log("Waiting for cleanup to start")
	spaceBeforeCleanup := freeSpace
	// Wait for cleanup to start
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		freeSpace, err := disk.FreeSpace(userHome)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get free space"))
		}

		if freeSpace > spaceBeforeCleanup {
			return nil
		}

		return errors.Errorf("too little disk space %v", freeSpace)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Error("Space was not cleared: ", err)
	}

	// Wait for cleanup to finish
	testing.Sleep(ctx, 2*time.Second)

	freeSpace, err = disk.FreeSpace(userHome)
	if err != nil {
		s.Fatal("Failed get free space: ", err)
	}

	if freeSpace > threshold+homedirSize {
		s.Errorf("Mounted user was cleaned up, %v free space available", freeSpace)
	}
}

func AutomaticCleanup(ctx context.Context, s *testing.State) {
	for _, param := range []struct {
		name               string
		threshold          uint64
		fillDirectory      string
		clearedDirectories []string
	}{
		{
			name:               "cache",
			threshold:          gib,
			fillDirectory:      "Cache",
			clearedDirectories: []string{},
		}, {
			name:               "gcache",
			threshold:          gib,
			fillDirectory:      "GCache/v1/tmp",
			clearedDirectories: []string{"Cache"},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			checkAutomaticCleanup(ctx, s, param.fillDirectory, param.clearedDirectories, param.threshold)
		})
	}
}
