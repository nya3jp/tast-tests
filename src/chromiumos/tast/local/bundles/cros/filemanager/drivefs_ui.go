// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path"
	"time"

	"chromiumos/tast/local/bundles/cros/filemanager/pre"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DrivefsUI,
		Desc: "Verifies that drivefs can be accessed through the UI",
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
		Pre:  pre.DriveFsStarted,
		Vars: []string{"filemanager.user", "filemanager.password"},
	})
}

func DrivefsUI(ctx context.Context, s *testing.State) {
	mountPath := s.PreValue().(drivefs.PreData).MountPath
	tconn := s.PreValue().(drivefs.PreData).TestAPIConn

	const (
		filesAppUITimeout = 15 * time.Second
		testFileName      = "drivefs"
	)

	// Create a test file inside Drive.
	drivefsRoot := path.Join(mountPath, "root")
	testFile, err := os.Create(path.Join(drivefsRoot, testFileName))
	if err != nil {
		s.Fatal("Could not create the test file inside ", drivefsRoot, ": ", err)
	}
	testFile.Close()
	// Don't delete the test file after the test as there may not be enough time
	// after the test for the deletion to be synced to Drive.

	// Launch Files App and check that Drive is accessible.
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
		s.Fatalf("Could not find test file %q in Drive: %v", testFileName, err)
	}
}
