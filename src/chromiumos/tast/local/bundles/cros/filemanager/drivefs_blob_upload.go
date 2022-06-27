// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DrivefsBlobUpload,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that a file created in DriveFS is uploaded",
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

func DrivefsBlobUpload(ctx context.Context, s *testing.State) {
	const (
		retryAttempts = 20
		retryInterval = time.Second
	)
	fixt := s.FixtValue().(*drivefs.FixtureData)
	apiClient := fixt.APIClient
	driveFsClient := fixt.DriveFs

	// Give the Drive API enough time to remove the file.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer driveFsClient.SaveLogsOnError(cleanupCtx, s.HasError)

	// Create a random file locally
	testFileName := drivefs.GenerateTestFileName(s.TestName()) + ".txt"
	testFilePath := driveFsClient.MyDrivePath(testFileName)
	if err := fsutil.CopyFile(s.DataPath("test_1KB.txt"), testFilePath); err != nil {
		s.Fatal("Failed to copy test file: ", err)
	}
	// Cleanup: Remove the file locally
	defer os.Remove(testFilePath)
	testFile, err := driveFsClient.NewFile(testFilePath)
	if err != nil {
		s.Fatal("Could not build DriveFS file: ", err)
	}
	err = action.RetrySilently(retryAttempts, testFile.CloudIDCreatedAction(), retryInterval)(ctx)
	if err != nil {
		s.Fatal("File not uploaded: ", err)
	}
	id, err := testFile.ItemID()
	if err != nil {
		s.Fatal("Failed to get ID: ", err)
	}
	// Cleanup: Remove the file on the cloud
	defer apiClient.RemoveFileByID(cleanupCtx, id)

	// Wait for all data to upload
	err = action.RetrySilently(retryAttempts, testFile.UploadedAction(), retryInterval)(ctx)
	if err != nil {
		s.Fatal("File not uploaded: ", err)
	}

	// Now compare the uploaded data with what we have locally
	driveFile, err := apiClient.GetFileByID(ctx, id)
	if err != nil {
		s.Fatal("Failed to get file metadata: ", err)
	}
	md5Sum, err := drivefs.MD5SumFile(testFilePath)
	if !strings.EqualFold(md5Sum, driveFile.Md5Checksum) {
		s.Errorf("Checksum mismatch! Got: %v Expected: %v", md5Sum, driveFile.Md5Checksum)
	}
}
