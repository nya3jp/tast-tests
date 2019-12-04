// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

const (
	mountPointTimeout = 10 * time.Second
	fuseIoTimeout     = 10 * time.Second
	stillRunningDelay = 20 * time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Drivefs,
		Desc: "Verifies that drivefs mounts on sign in",
		Contacts: []string{
			"dats@chromium.org",
			"austinct@chromium.org",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
			"drivefs",
		},
		Attr: []string{
			"group:mainline",
			"informational",
		},
		Vars: []string{
			"platform.Drivefs.user",     // GAIA username.
			"platform.Drivefs.password", // GAIA password.
		},
	})
}

type stat struct {
	stat os.FileInfo
	err  error
}

func statAsync(path string, timeout time.Duration) (os.FileInfo, error) {
	ch := make(chan stat, 1)
	defer close(ch)
	go func() {
		s, e := os.Stat(path)
		ch <- stat{s, e}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case s := <-ch:
		return s.stat, s.err
	case <-timer.C:
		return nil, context.DeadlineExceeded
	}
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

func waitForMatchingMount(ctx context.Context, timeout time.Duration, matcher func(sysutil.MountInfo) bool) error {
	testing.ContextLogf(ctx, "Waiting %v for a matching mount to appear", timeout)
	return testing.Poll(ctx, func(ctx context.Context) error {
		matches, err := findMatchingMount(matcher)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return os.ErrNotExist
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second})
}

func isDriveFs(info sysutil.MountInfo) bool {
	return info.Fstype == "fuse.drivefs"
}

func Drivefs(ctx context.Context, s *testing.State) {
	user := s.RequiredVar("platform.Drivefs.user")
	password := s.RequiredVar("platform.Drivefs.password")

	// Sign in a real user.
	cr, err := chrome.New(
		ctx,
		chrome.ARCDisabled(),
		chrome.Auth(user, password, ""),
		chrome.GAIALogin(),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Check that home folder where we store data files is properly mounted.
	ok, err := cryptohome.IsMounted(ctx, cr.User())
	if err != nil || !ok {
		s.Fatal("Failed to mount cryptohome: ", err)
	}

	homePath, err := cryptohome.UserPath(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to get home path: ", err)
	}

	// Check that cache folder was created by cryptohome.
	cachePath := path.Join(homePath, "GCache", "v2")
	if dir, err := os.Stat(cachePath); !dir.IsDir() {
		s.Fatal("Cache dir ", cachePath, " is missing: ", err)
	}

	// It takes some time for request to mount Drive to be handled by CrosDisks
	// that creates the mount point. Poll for a mount point until timeout.
	err = waitForMatchingMount(ctx, mountPointTimeout, isDriveFs)
	if err != nil {
		s.Fatal("Timeout while waiting for mountpoint creation: ", err)
	}
	mounts, err := findMatchingMount(isDriveFs)
	if err != nil {
		s.Fatal("Could not obtain mounts: ", err)
	}
	if len(mounts) != 1 {
		s.Fatal("Expected only one drivefs mount but found ", len(mounts))
	}
	mountPath := mounts[0].MountPath
	s.Log("drivefs is mounted into ", mountPath)

	// We expect to find at least this folder in the mount point.
	drivefsRoot := path.Join(mountPath, "root")

	// As drivefs may not be fully initialized yet all access to the mount point
	// may block inside FUSE driver until the daemon is ready.
	// Run those stats async in case the drivefs daemon is never ready due to
	// some bug.
	if dir, err := statAsync(drivefsRoot, fuseIoTimeout); !dir.IsDir() {
		s.Fatal("Could not find root folder inside ", mountPath, ": ", err)
	}

	// Now we are relatively confident that drivefs started correctly.
	// Check for team_drives the easy way.
	drivefsTeamDrives := path.Join(mountPath, "team_drives")
	if dir, err := os.Stat(drivefsTeamDrives); !dir.IsDir() {
		s.Fatal("Could not find team_drives folder inside ", mountPath, ": ", err)
	}

	// Now wait a bit in case there is a delayed crash.
	s.Log("Waiting for ", stillRunningDelay, " to check that drivefs mount did not disappear.")
	testing.Sleep(ctx, stillRunningDelay)

	mounts, err = findMatchingMount(isDriveFs)
	if err != nil {
		s.Fatal("Could not obtain mounts: ", err)
	}
	if len(mounts) != 1 {
		s.Fatal("Expected one drivefs mount to remain but found ", len(mounts))
	}
	if mounts[0].MountPath != mountPath {
		s.Fatal("Mount point changed from ", mountPath, " to ", mounts[0].MountPath)
	}
}
