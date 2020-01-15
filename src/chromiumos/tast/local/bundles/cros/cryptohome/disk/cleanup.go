// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package disk

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Constants taken from cryptohome.
const (
	// Location of user directories.
	UserHome = "/home/user"

	MiB              uint64 = 1024 * 1024 // 1 MiB
	GiB              uint64 = 1024 * MiB  // 1 GiB
	MinimalFreeSpace        = 512 * MiB   // hard-coded in cryptohomed
	CleanupTarget           = 2 * GiB     // hard-coded in cryptohomed
)

// RunAutomaticCleanup triggers cleanup by restarting cryptohome and waits until it is done.
func RunAutomaticCleanup(ctx context.Context) error {
	testing.ContextLog(ctx, "Performing automatic cleanup")

	reader, err := syslog.NewReader(syslog.Program("cryptohomed"))
	if err != nil {
		return errors.Wrap(err, "failed to start log reader")
	}
	defer reader.Close()

	// Restart to trigger cleanup
	if err := upstart.RestartJob(ctx, "cryptohomed", "VMODULE_ARG=*=1"); err != nil {
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

// ForceAutomaticCleanup forces automatic cleanup by filling the disk before triggering cleanup.
func ForceAutomaticCleanup(ctx context.Context) error {
	fillFile, err := FillUntil(UserHome, MinimalFreeSpace)
	if err != nil {
		return errors.Wrap(err, "failed to fill disk")
	}
	defer os.Remove(fillFile)

	if err := RunAutomaticCleanup(ctx); err != nil {
		return errors.Wrap(err, "failed to run cleanup")
	}

	return nil
}

// CleanupExistingUsers cleans up existing users so they don't affect the test by:
// 1. Create a temporary user that takes up the remaining free space.
// 2. Unmount all users.
// 3. Repeatedly trigger cleanup to clear all other users.
func CleanupExistingUsers(ctx context.Context) error {
	const (
		temporaryUser = "cleanup-removal-user"
		password      = "1234"
	)
	removalLogMessages := []string{
		"Deleting Cache",
		"Deleting GCache",
		"Deleting Android Cache",
		"Freeing disk space by deleting user",
	}

	testing.ContextLog(ctx, "Cleaning up existing users")

	// Unmount all users.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount users")
	}

	// Cleanups are currently repeated so we need to remember what was already done.
	performedCleanups := make(map[string]bool)

	// Repeat cleanup until cleanup is ineffective.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		reader, err := syslog.NewReader(syslog.Program("cryptohomed"))
		if err != nil {
			return errors.Wrap(err, "failed to start log reader")
		}
		defer reader.Close()

		if err := ForceAutomaticCleanup(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to run cleanup"))
		}

		// True iff any new cleanup was performed.
		cleanupPerformed := false
		for {
			entry, err := reader.Read()
			if err != nil {
				if err == io.EOF {
					if cleanupPerformed {
						return errors.New("logged deletion, repeating cleanup")
					}

					return nil
				}

				return testing.PollBreak(errors.Wrap(err, "failed to read syslog"))
			}

			for _, message := range removalLogMessages {
				if strings.Contains(entry.Content, message) && !performedCleanups[entry.Content] {
					performedCleanups[entry.Content] = true
					cleanupPerformed = true
				}
			}
		}

		return nil
	}, &testing.PollOptions{
		Timeout: 30 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to remove users")
	}

	if freeSpace, err := FreeSpace(UserHome); err != nil {
		return errors.Wrap(err, "failed get free space")
	} else if freeSpace < CleanupTarget { // Sanity check
		return errors.Errorf("too little free space is available: %d", freeSpace)
	} else {
		testing.ContextLogf(ctx, "%d bytes available after automatic cleanup", freeSpace)
	}

	return nil
}

// CreateFilledUserHomedir creates a user taking up size space by filling dir.
func CreateFilledUserHomedir(ctx context.Context, user, pass, dir string, size uint64) (string, error) {
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
		if _, err := os.Stat(filepath.Join(UserHome, hash, dir)); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return "", errors.Wrap(err, "folder not created")
	}

	fillFile, err := Fill(filepath.Join(UserHome, hash, dir), size)
	if err != nil {
		return "", errors.Wrap(err, "failed to fill space")
	}

	ok = true
	return fillFile, nil
}
