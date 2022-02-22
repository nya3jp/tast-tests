// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DrivefsUI,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that drivefs can be accessed through the UI",
		Contacts: []string{
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
		},
		VarDeps: []string{
			"filemanager.DrivefsUI.username",
			"filemanager.DrivefsUI.password",
		},
	})
}

func DrivefsUI(ctx context.Context, s *testing.State) {
	const testFileName = "drivefs"
	username := s.RequiredVar("filemanager.DrivefsUI.username")
	password := s.RequiredVar("filemanager.DrivefsUI.password")

	// Start up Chrome.
	cr, err := chrome.New(ctx, chrome.GAIALogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	mountPath, err := drivefs.WaitForDriveFs(ctx, username)
	if err != nil {
		s.Fatal("Failed to wait for DriveFS to be mounted: ", err)
	}

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API Connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Create a test file inside Drive.
	drivefsRoot := filepath.Join(mountPath, "root")
	testFile, err := os.Create(filepath.Join(drivefsRoot, testFileName))
	if err != nil {
		s.Fatalf("Failed to create test file inside %q: %v", drivefsRoot, err)
	}
	testFile.Close()
	// Don't delete the test file after the test as there may not be enough time
	// after the test for the deletion to be synced to Drive.

	// Launch Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}

	if err := uiauto.Combine("check Drive",
		// Open the Google Drive folder and check for the test file.
		files.OpenDrive(),
		// Wait for the file, if it can't find it try to maximize the window and find again.
		files.PerformActionAndRetryMaximizedOnFail(files.WaitForFile(testFileName)),
	)(ctx); err != nil {
		s.Fatal("Failed to wait for the test file in Drive: ", err)
	}
}
