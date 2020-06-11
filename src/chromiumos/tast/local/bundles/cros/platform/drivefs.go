// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"path"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/drivefs"
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

func Drivefs(ctx context.Context, s *testing.State) {
	const (
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

	mountPath, err := drivefs.WaitForDriveFs(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}

	// We expect to find at least this folder in the mount point.
	drivefsRoot := path.Join(mountPath, "root")

	// Now we are relatively confident that drivefs started correctly.
	// Check for team_drives.
	drivefsTeamDrives := path.Join(mountPath, "team_drives")
	dir, err := os.Stat(drivefsTeamDrives)
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
