// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"math/rand"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
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
		Timeout: 5 * time.Minute,
		Params: []testing.Param{{
			Fixture: "driveFsStarted",
		}, {
			Name:    "chrome_networking",
			Fixture: "driveFsStartedWithChromeNetworking",
		}},
	})
}

func writeRandomData(file *os.File, n int) error {
	const chunkBytesMax = 1000000
	chunk := make([]byte, chunkBytesMax)
	for n > 0 {
		chunkBytes := n
		if chunkBytes > chunkBytesMax {
			chunkBytes = chunkBytesMax
		}
		_, err := rand.Read(chunk)
		if err != nil {
			return err
		}
		_, err = file.Write(chunk[0:chunkBytes])
		if err != nil {
			return err
		}
		n -= chunkBytes
	}
	return nil
}

func DrivefsBlobUpload(ctx context.Context, s *testing.State) {
	const (
		retryAttempts = 20
		retryInterval = time.Second

		uploadFileBytes = 1000 // 1KiB
	)
	fixt := s.FixtValue().(*drivefs.FixtureData)
	apiClient := fixt.APIClient
	driveFsClient := fixt.DriveFs

	// Give the Drive API enough time to remove the file.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer driveFsClient.SaveLogsOnError(ctx, s.HasError)

	// Create a random file locally
	testFilePath := driveFsClient.MyDrivePath(drivefs.GenerateTestFileName(s.TestName()))
	testFile, err := driveFsClient.Create(testFilePath)
	if err != nil {
		s.Fatal("Could not create blob: ", err)
	}
	// Cleanup: Remove the file locally
	defer os.Remove(testFilePath)
	err = writeRandomData(testFile.File, uploadFileBytes)
	if err != nil {
		s.Fatal("Could not write random data: ", err)
	}
	testFile.Close()
	err = action.RetrySilently(retryAttempts, testFile.IDCreated(), retryInterval)(ctx)
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
	err = action.RetrySilently(retryAttempts, testFile.Committed(), retryInterval)(ctx)
	if err != nil {
		s.Fatal("File not committed: ", err)
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
