// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"os"
	"path"
	"time"

	"chromiumos/tast/local/bundles/cros/drivefs/pre"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Smoke,
		Desc: "Verifies that drivefs mounts on sign in",
		Contacts: []string{
			"dats@chromium.org",
			"austinct@chromium.org",
			"benreich@chromium.org",
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
		Pre:  pre.DrivefsStarted,
		Vars: []string{"drivefs.username", "drivefs.password"},
	})
}

func Smoke(ctx context.Context, s *testing.State) {
	d := s.PreValue().(drivefs.PreData)
	cr := d.Chrome
	mountPath := d.MountPath
	drivefsRoot := path.Join(mountPath, "root")

	const (
		filesAppUITimeout = 15 * time.Second
		testFileName      = "drivefs"
	)

	// Check for team_drives.
	drivefsTeamDrives := path.Join(mountPath, "team_drives")
	dir, err := os.Stat(drivefsTeamDrives)
	if err != nil {
		s.Fatalf("Failed trying to stat %s: %v", drivefsTeamDrives, err)
	}
	if !dir.IsDir() {
		s.Fatalf("Failed finding team_drives folder inside %s: %v", mountPath, err)
	}

	// Create a test file inside Drive.
	testFile, err := os.Create(path.Join(drivefsRoot, testFileName))
	if err != nil {
		s.Fatalf("Failed trying to create test file inside %s: %v", drivefsRoot, err)
	}
	testFile.Close()
	// Don't delete the test file after the test as there may not be enough time
	// after the test for the deletion to be synced to Drive.

	// Launch Files App and check that Drive is accessible.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed creating test API connection: ", err)
	}
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed launching files app: ", err)
	}
	// Instead of closing the Files App, just release the memory reference.
	// Otherwise, when this test fails, the screenshot will be of an empty desktop/closing app.
	defer filesApp.Root.Release(ctx)

	// Navigate to Google Drive via the Files App ui.
	if err := filesApp.OpenDrive(ctx); err != nil {
		s.Fatal("Failed trying to open Google Drive folder: ", err)
	}

	// Check for the test file created earlier.
	if err := filesApp.WaitForFile(ctx, testFileName, filesAppUITimeout); err != nil {
		s.Fatalf("Failed trying to find test file %s in Google Drive: %v", testFileName, err)
	}
}
