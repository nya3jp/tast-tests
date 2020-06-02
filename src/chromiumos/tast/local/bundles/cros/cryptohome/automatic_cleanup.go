// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/cryptohome/cleanup"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/syslog"
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
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func AutomaticCleanup(ctx context.Context, s *testing.State) {
	const (
		userHome = "/home/user"

		mib               uint64 = 1024 * 1024              // 1 MiB
		gib               uint64 = 1024 * mib               // 1 GiB
		minimalFreeSpace         = 512 * mib                // hard-coded in cryptohomed
		cleanupTrigger           = 2 * gib                  // hard-coded in cryptohomed
		homedirSize              = 900 * mib                // 900 Mib, used for testing
		startingFreeSpace        = cleanupTrigger + 200*mib // 2.2 GiB, used for testing

		user1    = "cleanup-user1"
		user2    = "cleanup-user2"
		password = "1234"
	)

	createCacheDir := func(ctx context.Context, user, pass, dir string, size uint64) (string, error) {
		if err := cryptohome.CreateVault(ctx, user, pass); err != nil {
			return "", errors.Wrap(err, "failed to create user vault")
		}
		ok := false
		defer func() {
			if !ok {
				cryptohome.RemoveVault(ctx, user)
			}
		}()

		hash, err := cryptohome.UserHash(ctx, user)
		if err != nil {
			return "", errors.Wrap(err, "failed to get user hash")
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := os.Stat(filepath.Join(userHome, hash, dir)); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return "", errors.Wrap(err, "folder not created")
		}

		fillFile, err := cleanup.Fill(filepath.Join(userHome, hash, dir), size)
		if err != nil {
			return "", errors.Wrap(err, "failed to fill space")
		}

		ok = true
		return fillFile, nil
	}

	// Start cryptohomed and wait for it to be available
	if err := upstart.EnsureJobRunning(ctx, "cryptohomed"); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	// Stay above trigger for cleanup
	fillFile, err := cleanup.FillUntil(userHome, startingFreeSpace)
	if err != nil {
		s.Fatal("Failed to fill space: ", err)
	}
	defer os.Remove(fillFile)

	if freeSpace, err := disk.FreeSpace(userHome); err != nil {
		s.Fatal("Failed get free space: ", err)
	} else if freeSpace < 2*homedirSize { // Sanity check
		s.Fatal("Too little free space is available: ", freeSpace)
	} else {
		s.Logf("%v bytes remaining", freeSpace)
	}

	// Create users with contents to fill up disk space
	if _, err := createCacheDir(ctx, user1, password, "Cache", homedirSize); err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, user1)

	fillFile2, err := createCacheDir(ctx, user2, password, "Cache", homedirSize)
	if err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, user2)

	// Make sure disk space is low
	freeSpace, err := disk.FreeSpace(userHome)
	if err != nil {
		s.Fatal("Failed get free space: ", err)
	} else if freeSpace > minimalFreeSpace {
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

	// Restart to trigger cleanup
	if err := upstart.RestartJob(ctx, "cryptohomed"); err != nil {
		s.Fatal("Failed to restart cryptohomed: ", err)
	}

	reader, err := syslog.NewReader()
	if err != nil {
		s.Fatal("Failed to start log reader: ", err)
	}
	defer reader.Close()

	s.Log("Waiting for cleanup to start")
	spaceBeforeCleanup := freeSpace
	// Wait for cleanup to start
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if freeSpace, err := disk.FreeSpace(userHome); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get free space"))
		} else if freeSpace <= spaceBeforeCleanup {
			return errors.Errorf("too little disk space %v", freeSpace)
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Space was not cleared: ", err)
	}

	// Wait for cleanup to finish
	if _, err := reader.Wait(ctx, 5*time.Second, func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, "Disk cleanup complete.")
	}); err != nil {
		s.Fatal("Cleanup not complete: ", err)
	}

	if freeSpace, err := disk.FreeSpace(userHome); err != nil {
		s.Fatal("Failed get free space: ", err)
	} else if freeSpace > minimalFreeSpace+homedirSize {
		s.Errorf("Mounted user was cleaned up, %v free space available", freeSpace)
	}
}
