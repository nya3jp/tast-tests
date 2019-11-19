// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

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

const shadowRoot = "/home/.shadow"
const userHome = "/home/user"

const gib uint64 = 1024 * 1024 * 1024 // 1 GiB
const homedirSize = gib / 10 * 8      // 800 Mib
const minimalFreeSpace = gib / 2      // 500 MiB

func createCacheDirWithContent(ctx context.Context, user, pass, dir string, size uint64) error {
	if err := cryptohome.CreateVault(ctx, user, pass); err != nil {
		return errors.Wrap(err, "failed to create user vault")
	}

	hash, err := cryptohome.UserHash(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to get user hash")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(filepath.Join(userHome, hash, dir)); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "folder not created")
	}

	_, err = disk.Fill(filepath.Join(userHome, hash, dir), size)
	if err != nil {
		return errors.Wrap(err, "failed to fill space")
	}

	return nil
}

func AutomaticCleanup(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "cryptohomed"); err != nil {
		s.Fatal("Failed to run cryptohomed: ", err)
	}

	// Stay above trigger for cleanup
	file, err := disk.FillUntil("/home/user/", 2*gib)
	if err != nil {
		s.Fatal("Failed to fill space: ", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	freeSpace, err := disk.FreeSpace("/home/user/")
	if err != nil {
		s.Fatal("Failed get free space: ", err)
	}

	if freeSpace < 2*homedirSize { // Sanity check
		s.Fatal("Too little free space is available: ", freeSpace)
	}

	s.Logf("%v bytes remaining", freeSpace)

	// Create users with contents
	if err := createCacheDirWithContent(ctx, "cleanup-user1", "1234", "Cache", homedirSize); err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, "cleanup-user1")

	if err := createCacheDirWithContent(ctx, "cleanup-user2", "1234", "Cache", homedirSize); err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, "cleanup-user2")

	// Make sure disk space is low
	freeSpace, err = disk.FreeSpace("/home/user/")
	if err != nil {
		s.Fatal("Failed get free space: ", err)
	}

	if freeSpace > gib/2 {
		s.Errorf("Space was not filled, %v available", freeSpace)
	}

	// Unmount first user
	if err := cryptohome.UnmountVault(ctx, "cleanup-user1"); err != nil {
		s.Fatal("Failed to unmount user vault: ", err)
	}

	// Restart cryptohome to trigger cleanup
	upstart.RestartJob(ctx, "cryptohomed")

	// Wait for contents to be deleted
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		freeSpace, err := disk.FreeSpace("/home/user/")
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get free space"))
		}

		if freeSpace < gib/2 {
			return nil
		}

		return errors.Errorf("too little disk space %v", freeSpace)
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Space was not cleared: ", err)
	}

	freeSpace, err = disk.FreeSpace("/home/user/")
	if err != nil {
		s.Fatal("Failed get free space: ", err)
	}

	if freeSpace > minimalFreeSpace+homedirSize {
		s.Errorf("Mounted user was cleaned up, %v free space available", freeSpace)
	}
}
