// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/filemanager/pre"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DrivefsGoogleDoc,
		Desc: "Verify that a google doc created via Drive API syncs to DriveFS",
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
		Pre: pre.DriveFsStarted,
		Vars: []string{
			"filemanager.user",
			"filemanager.password",
			"filemanager.drive_credentials",
		},
	})
}

func DrivefsGoogleDoc(ctx context.Context, s *testing.State) {
	driveAPI := s.PreValue().(drivefs.PreData).DriveAPI
	tconn := s.PreValue().(drivefs.PreData).TestAPIConn

	const filesAppUITimeout = 30 * time.Second
	testDocFileName := fmt.Sprintf("doc-drivefs-%d", time.Now().UnixNano())

	// Create a test file using Drive API.
	if err := driveAPI.CreateBlankGoogleDoc(ctx, testDocFileName, []string{"root"}); err != nil {
		s.Fatal("Could not create blank google doc: ", err)
	}

	// Files are fresh when files app first launches as opposed to refreshes.
	testing.Sleep(ctx, time.Minute)

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
	testFileNameWithExt := fmt.Sprintf("%s.gdoc", testDocFileName)
	if err := filesApp.WaitForFile(ctx, testFileNameWithExt, filesAppUITimeout); err != nil {
		s.Fatalf("Could not find test file %q in Drive: %v", testFileNameWithExt, err)
	}
}
