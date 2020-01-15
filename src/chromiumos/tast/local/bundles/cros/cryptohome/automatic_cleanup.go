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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/cryptohome/disk"
	"chromiumos/tast/local/cryptohome"
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

		temporaryUser = "tmp-user"
		user1         = "cleanup-user1"
		user2         = "cleanup-user2"
		password      = "1234"
	)

	// createCacheDir creates a user taking up size space by filling dir.
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

		fillFile, err := disk.Fill(filepath.Join(userHome, hash, dir), size)
		if err != nil {
			return "", errors.Wrap(err, "failed to fill space")
		}

		ok = true
		return fillFile, nil
	}

	// runCleanup trigger cleanup by restarting cryptohome and waiting until it is done
	runCleanup := func(ctx context.Context) error {
		reader, err := syslog.NewReader(syslog.Program("cryptohomed"))
		if err != nil {
			return errors.Wrap(err, "failed to start log reader")
		}
		defer reader.Close()

		testing.ContextLog(ctx, "Performing automatic cleanup")
		// Restart to trigger cleanup
		if err := upstart.RestartJob(ctx, "cryptohomed"); err != nil {
			return errors.Wrap(err, "failed to restart cryptohomed")
		}

		// Wait for cleanup to finish
		if _, err := reader.Wait(ctx, 10*time.Second, func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, "Disk cleanup complete.")
		}); err != nil {
			return errors.Wrap(err, "cleanup not complete")
		}

		return nil
	}

	// initialCleanUp removes existing users so they don't affect the current test by:
	// 1. Create a temporary user that takes up the remaining free space.
	// 2. Unmount all users.
	// 3. Trigger cleanup to clear all Cache directories.
	initialCleanUp := func(ctx context.Context) error {
		testing.ContextLog(ctx, "Starting initial cleanup")

		freeSpace, err := disk.FreeSpace(userHome)
		if err != nil {
			return errors.Wrap(err, "failed get free space")
		}

		if _, err := createCacheDir(ctx, temporaryUser, password, "Cache", freeSpace-minimalFreeSpace); err != nil {
			return errors.Wrap(err, "failed to create temporary user")
		}
		defer cryptohome.RemoveVault(ctx, temporaryUser)

		// Give cleanup time to finish
		ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
		defer cancel()

		// Unmount all users
		if err := cryptohome.UnmountAll(ctx); err != nil {
			return errors.Wrap(err, "failed to unmount users")
		}

		if err := runCleanup(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to run cleanup"))
		}

		freeSpace, err = disk.FreeSpace(userHome)
		if err != nil {
			return errors.Wrap(err, "failed get free space")
		}
		testing.ContextLogf(ctx, "%v bytes available after initial cleanup", freeSpace)

		return nil
	}

	// Start cryptohomed and wait for it to be available
	if err := upstart.EnsureJobRunning(ctx, "cryptohomed"); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohomed not running as expected: ", err)
	}

	if err := initialCleanUp(ctx); err != nil {
		s.Fatal("Failed to perform initial cleanup: ", err)
	}

	// Stay above trigger for cleanup
	fillFile, err := disk.FillUntil(userHome, startingFreeSpace)
	if err != nil {
		s.Fatal("Failed to fill space: ", err)
	}
	defer os.Remove(fillFile)

	if freeSpace, err := disk.FreeSpace(userHome); err != nil {
		s.Fatal("Failed get free space: ", err)
	} else if freeSpace < 2*homedirSize { // Sanity check
		s.Fatal("Too little free space is available: ", freeSpace)
	} else {
		s.Logf("%v bytes available after fill", freeSpace)
	}

	// Create users with contents to fill up disk space
	fillFile1, err := createCacheDir(ctx, user1, password, "Cache", homedirSize)
	if err != nil {
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

	if err := runCleanup(ctx); err != nil {
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

	if freeSpace, err := disk.FreeSpace(userHome); err != nil {
		s.Fatal("Failed get free space: ", err)
	} else {
		s.Logf("%v bytes available after cleanup", freeSpace)
	}
}
