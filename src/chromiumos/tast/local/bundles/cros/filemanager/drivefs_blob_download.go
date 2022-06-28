// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DrivefsBlobDownload,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that a file created in Drive Web can be downloaded",
		Contacts: []string{
			"travislane@google.com",
			"chromeos-files-syd@google.com",
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
		Data: []string{
			"test_1KB.txt",
		},
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Fixture: "driveFsStarted",
		}, {
			Name:    "chrome_networking",
			Fixture: "driveFsStartedWithChromeNetworking",
		}},
	})
}

func DrivefsBlobDownload(ctx context.Context, s *testing.State) {
	const (
		retryAttempts = 20
		retryInterval = 5 * time.Second
	)
	fixt := s.FixtValue().(*drivefs.FixtureData)
	apiClient := fixt.APIClient
	driveFsClient := fixt.DriveFs

	// Give the Drive API enough time to remove the file.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer driveFsClient.SaveLogsOnError(cleanupCtx, s.HasError)

	// Create the test file with the Drive API
	testFileName := drivefs.GenerateTestFileName(s.TestName()) + ".txt"
	driveFile, err := apiClient.CreateFileFromLocalFile(ctx,
		testFileName, "root", s.DataPath("test_1KB.txt"))
	if err != nil {
		s.Fatal("Could not create test file: ", err)
	}
	s.Logf("Created %s with ID: %s", testFileName, driveFile.Id)
	// Cleanup: Remove the file on the cloud
	defer apiClient.RemoveFileByID(cleanupCtx, driveFile.Id)

	// Wait for file to be available locally
	testFilePath := driveFsClient.MyDrivePath(testFileName)
	testFile, err := driveFsClient.NewFile(testFilePath)
	if err != nil {
		s.Fatal("Could not build DriveFS file: ", err)
	}
	err = action.RetrySilently(retryAttempts, testFile.ExistsAction(), retryInterval)(ctx)
	if err != nil {
		s.Fatal("File not available locally: ", err)
	}

	// Now compare the uploaded data with what we have locally
	md5Sum, err := drivefs.MD5SumFile(testFilePath)
	if !strings.EqualFold(md5Sum, driveFile.Md5Checksum) {
		s.Errorf("Checksum mismatch! Got: %v Expected: %v", md5Sum, driveFile.Md5Checksum)
	}
}
