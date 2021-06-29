// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cleanup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// Constants taken from cryptohome.
const (
	// Location of user directories.
	UserHome = "/home/user"

	MiB                   uint64 = 1024 * 1024 // 1 MiB
	GiB                   uint64 = 1024 * MiB  // 1 GiB
	MinimalFreeSpace             = 512 * MiB   // hard-coded in cryptohomed
	CleanupTarget                = 2 * GiB     // Default value in cryptohomed, when to stop cleaning.
	NotificationThreshold        = 1 * GiB     // Default value in cryptohomed, threshold for sending the low-disk-space notification.
)

// RunAutomaticCleanup triggers cleanup by restarting cryptohome and waits until it is done.
func RunAutomaticCleanup(ctx context.Context, cleanupThreshold, aggressiveCleanupThreshold, targetThreshold uint64) error {
	testing.ContextLog(ctx, "Performing automatic cleanup")

	cleanupThresholdsArgs := fmt.Sprintf("--cleanup_threshold=%d --aggressive_cleanup_threshold=%d --target_free_space=%d", cleanupThreshold, aggressiveCleanupThreshold, targetThreshold)

	reader, err := syslog.NewReader(ctx, syslog.Program(syslog.Cryptohomed))
	if err != nil {
		return errors.Wrap(err, "failed to start log reader")
	}
	defer reader.Close()

	// Restart to trigger cleanup.
	if err := upstart.RestartJob(ctx, "cryptohomed", upstart.WithArg("VMODULE_ARG", "*=1"), upstart.WithArg("CRYPTOHOMED_ARGS", cleanupThresholdsArgs)); err != nil {
		return errors.Wrap(err, "failed to restart cryptohomed")
	}

	// Wait for cleanup to finish.
	if _, err := reader.Wait(ctx, 60*time.Second, func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, "Disk cleanup complete.")
	}); err != nil {
		return errors.Wrap(err, "cleanup not complete")
	}

	return nil
}

// ForceAutomaticCleanup sets the cleanup thresholds above the current free space, forcing a cleanup.
// cleanupThreshold, aggressiveCleanupThreshold = freeSpace + MinimalFreeSpace
// targetThreshold := freeSpace + CleanupTarget
func ForceAutomaticCleanup(ctx context.Context) error {
	freeSpace, err := disk.FreeSpace(UserHome)
	if err != nil {
		return errors.Wrap(err, "failed to get the amount of free space")
	}

	cleanupThreshold := freeSpace + MinimalFreeSpace
	targetThreshold := freeSpace + 10*CleanupTarget

	return RunAutomaticCleanup(ctx, cleanupThreshold, targetThreshold, targetThreshold)
}

// RunOnExistingUsers cleans up existing users so they don't affect the test by:
// 1. Unmount all users.
// 2. Trigger cleanup to clear all users.
// 3. If any cleanup was performed repeat 2.
func RunOnExistingUsers(ctx context.Context) error {
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
		reader, err := syslog.NewReader(ctx, syslog.Program(syslog.Cryptohomed))
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
			if err == io.EOF {
				if cleanupPerformed {
					return errors.New("logged deletion, repeating cleanup")
				}

				return nil
			} else if err != nil {
				return testing.PollBreak(errors.Wrap(err, "failed to read syslog"))
			}

			for _, message := range removalLogMessages {
				if strings.Contains(entry.Content, message) && !performedCleanups[entry.Content] {
					performedCleanups[entry.Content] = true
					cleanupPerformed = true
				}
			}
		}
	}, &testing.PollOptions{
		Timeout: 60 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to remove users")
	}

	if freeSpace, err := disk.FreeSpace(UserHome); err != nil {
		return errors.Wrap(err, "failed get free space")
	} else if freeSpace < CleanupTarget { // Validity check
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

	fillFile, err := disk.Fill(filepath.Join(UserHome, hash, dir), size)
	if err != nil {
		return "", errors.Wrap(err, "failed to fill space")
	}

	ok = true
	return fillFile, nil
}
