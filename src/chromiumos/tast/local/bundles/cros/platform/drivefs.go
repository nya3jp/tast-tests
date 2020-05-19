// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
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
			return errors.Wrap(err, "IO error trying to list mounts")
		}
		if len(matches) == 0 {
			return errors.Wrap(os.ErrNotExist, "The matching mount was not found")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second})
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

func Drivefs(ctx context.Context, s *testing.State) {
	const (
		mountPointTimeout = 15 * time.Second
		fuseIoTimeout     = 40 * time.Second
		filesAppUITimeout = 15 * time.Second
		testFileName      = "drivefs"
	)

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

	normUser, err := session.NormalizeEmail(cr.User(), true)
	if err != nil {
		s.Fatal("Failed to normalize user name: ", err)
	}
	s.Log("Logged in as ", normUser)

	// Check that cache folder was created by cryptohome.
	homePath, err := cryptohome.UserPath(ctx, normUser)
	if err != nil {
		s.Fatal("Failed to get home path: ", err)
	}
	cachePath := path.Join(homePath, "GCache", "v2")
	if dir, err := os.Stat(cachePath); !dir.IsDir() {
		s.Fatal("Cache dir ", cachePath, " is missing: ", err)
	}

	// It takes some time for request to mount Drive to be handled by CrosDisks
	// that creates the mount point. Poll for a mount point until timeout.
	if err := waitForMatchingMount(ctx, mountPointTimeout, isDriveFs); err != nil {
		s.Fatal("Timeout while waiting for mountpoint creation: ", err)
	}
	mounts, err := findMatchingMount(isDriveFs)
	if err != nil {
		s.Fatal("Could not obtain mounts: ", err)
	}
	if len(mounts) != 1 {
		s.Fatal("Expected only one drivefs mount but found ", len(mounts), ". Found mounts ", mounts)
	}
	mountPath := mounts[0].MountPath
	s.Log("drivefs is mounted into ", mountPath)

	// We expect to find at least this folder in the mount point.
	drivefsRoot := path.Join(mountPath, "root")

	// As drivefs may not be fully initialized yet all access to the mount point
	// may fail inside FUSE driver until the daemon is ready.
	// Poll for stat to succeed in case the drivefs daemon is never ready due to
	// some bug.
	if err := waitForMountConnected(ctx, fuseIoTimeout, drivefsRoot); err != nil {
		s.Fatal("Failed while waiting for stat: ", err)
	}
	dir, err := os.Stat(drivefsRoot)
	if err != nil {
		s.Fatal("Could not stat ", drivefsRoot, ": ", err)
	}
	if !dir.IsDir() {
		s.Fatal("Could not find root folder inside ", mountPath, ": ", err)
	}
	s.Log("drivefs fully started")

	// Now we are relatively confident that drivefs started correctly.
	// Check for team_drives.
	drivefsTeamDrives := path.Join(mountPath, "team_drives")
	dir, err = os.Stat(drivefsTeamDrives)
	if err != nil {
		s.Fatal("Could not stat ", drivefsTeamDrives, ": ", err)
	}
	if !dir.IsDir() {
		s.Fatal("Could not find team_drives folder inside ", mountPath, ": ", err)
	}

	// Create a test file inside Drive.
	testFile, err := os.Create(path.Join(drivefsRoot, testFileName))
	if err != nil {
		s.Fatal("Could not create the test file inside ", drivefsRoot, ": ", err)
	}
	testFile.Close()
	// Don't delete the test file after the test as there may not be enough time
	// after the test for the deletion to be synced to Drive.

	// Launch Files App and check that Drive is accessible.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not create test API connection: ", err)
	}
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not launch the Files App: ", err)
	}
	// Instead of closing the Files App, just release the memory reference.
	// Otherwise, when this test fails, the screenshot will be of an empty desktop/closing app.
	defer filesApp.Root.Release(ctx)

	// Navigate to Google Drive via the Files App ui.
	if err := filesApp.OpenDrive(ctx); err != nil {
		s.Fatal("Could not open Google Drive folder: ", err)
	}

	// Check for the test file created earlier.
	if err := filesApp.WaitForFile(ctx, testFileName, filesAppUITimeout); err != nil {
		s.Fatal("Could not find test file '", testFileName, "' in Drive: ", err)
	}
}
