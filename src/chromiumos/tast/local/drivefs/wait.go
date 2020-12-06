// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"os"
	"path"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	mountPointTimeout = 15 * time.Second // timeout waiting for CrosDisks to mount Drivefs.
	fuseIoTimeout     = 40 * time.Second // timeout waiting for the FUSE to be operational.
	restartDelay      = 4 * time.Second  // delay waiting for DriveFS to restart.
)

// WaitForDriveFs checks that the Drivefs mount is ready for IO.
// The username supplied must come from a user logged in using GAIA login.
func WaitForDriveFs(ctx context.Context, username string) (string, error) {
	// Attempt to normalize the logged in user
	normUser, err := session.NormalizeEmail(username, true)
	if err != nil {
		return "", errors.Wrap(err, "failed to normalize user name")
	}

	// Check that cache folder was created by cryptohome.
	homePath, err := cryptohome.UserPath(ctx, normUser)
	if err != nil {
		return "", errors.Wrap(err, "failed to get home path")
	}
	cachePath := path.Join(homePath, "GCache", "v2")
	if dir, err := os.Stat(cachePath); !dir.IsDir() {
		return "", errors.Wrapf(err, "failed as cache dir %s is missing", cachePath)
	}

	// It takes some time for request to mount Drive to be handled by CrosDisks
	// that creates the mount point. Poll for a mount point until timeout.
	mounts, err := waitForMatchingMount(ctx, mountPointTimeout, isDriveFs)
	if err != nil {
		return "", errors.Wrap(err, "failed with timeout while waiting for mountpoint creation")
	}
	if len(mounts) != 1 {
		return "", errors.Wrapf(err, "failed one drivefs mount expected found %d. Mounts found: %v", len(mounts), mounts)
	}
	mountPath := mounts[0].MountPath
	testing.ContextLog(ctx, "drivefs is mounted into ", mountPath)

	// On a clean start DriveFS would fetch some settings from the server and will
	// want to restart to apply them. Let's wait for a bit to allow it to do this.
	testing.ContextLogf(ctx, "Waiting %v for drivefs to stabilize", restartDelay)
	if testing.Sleep(ctx, restartDelay) != nil {
		return "", errors.Wrap(err, "failed while waiting for drivefs to stabilize")
	}

	// We expect to find at least this folder in the mount point.
	drivefsRoot := path.Join(mountPath, "root")

	// As drivefs may not be fully initialized yet all access to the mount point
	// may fail inside FUSE driver until the daemon is ready.
	// Poll for stat to succeed in case the drivefs daemon is never ready due to
	// some bug.
	if err := waitForMountConnected(ctx, fuseIoTimeout, drivefsRoot); err != nil {
		return "", errors.Wrap(err, "failed while waiting for stat")
	}
	dir, err := os.Stat(drivefsRoot)
	if err != nil {
		return "", errors.Wrapf(err, "failed trying to stat %s", drivefsRoot)
	}
	if !dir.IsDir() {
		return "", errors.Wrapf(err, "failed with no root folder inside %s", mountPath)
	}

	return mountPath, nil
}

func findMatchingMount(matcher func(sysutil.MountInfo) bool) (matches []sysutil.MountInfo, err error) {
	info, err := sysutil.MountInfoForPID(sysutil.SelfPID)
	if err != nil {
		return
	}
	for i := range info {
		if matcher(info[i]) {
			matches = append(matches, info[i])
		}
	}
	return
}

func waitForMatchingMount(ctx context.Context, timeout time.Duration, matcher func(sysutil.MountInfo) bool) ([]sysutil.MountInfo, error) {
	var matches []sysutil.MountInfo
	var err error
	testing.ContextLogf(ctx, "Waiting %v for a matching mount to appear", timeout)
	err = testing.Poll(ctx, func(ctx context.Context) error {
		matches, err = findMatchingMount(matcher)
		if err != nil {
			return errors.Wrap(err, "io error trying to list mounts")
		}
		if len(matches) == 0 {
			return errors.Wrap(os.ErrNotExist, "matching mount was not found")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second})
	return matches, err
}

func isDriveFs(info sysutil.MountInfo) bool {
	return info.Fstype == "fuse.drivefs"
}

func waitForMountConnected(ctx context.Context, timeout time.Duration, path string) error {
	testing.ContextLogf(ctx, "Waiting %v for mount to become connected", timeout)
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := os.Stat(path)
		return err
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second})
}
