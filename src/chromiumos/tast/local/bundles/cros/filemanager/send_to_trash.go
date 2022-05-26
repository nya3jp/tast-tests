// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SendToTrash,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that local and DriveFS files can be trashed",
		Contacts: []string{
			"benreich@chromium.org",
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
		Fixture: "driveFsStartedTrashEnabled",
	})
}

// sendToTrashTestData contains the test data for individual subtests on
// filesystems that are enabled for trash.
type sendToTrashTestData struct {
	name        string
	fileName    string
	openDirFunc uiauto.Action
	cleanupFunc func(ctx context.Context, cr *chrome.Chrome, s *testing.State, name string)
}

func SendToTrash(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*drivefs.FixtureData).Chrome
	mountPath := s.FixtValue().(*drivefs.FixtureData).MountPath
	tconn := s.FixtValue().(*drivefs.FixtureData).TestAPIConn

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Launch Files App.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Could not launch the Files App: ", err)
	}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Could not create keyboard: ", err)
	}

	// Setup the DriveFS test case data.
	drivefsRoot := filepath.Join(mountPath, "root")
	drivefs, err := createTestData(drivefsRoot, "../.Trash-1000")
	if err != nil {
		s.Fatal("Failed to create test data for DriveFS: ", err)
	}
	drivefs.name = "drivefs"
	drivefs.openDirFunc = filesApp.OpenDrive()

	// Setup the local files test case data.
	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users MyFiles path: ", err)
	}
	local, err := createTestData(downloadsPath, ".Trash")
	if err != nil {
		s.Fatal("Failed to create test data for DriveFS: ", err)
	}
	local.name = "local"
	local.openDirFunc = filesApp.OpenDownloads()

	for _, tc := range []sendToTrashTestData{drivefs, local} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			defer tc.cleanupFunc(cleanupCtx, cr, s, tc.name)
			trashedFileFinder := nodewith.NameContaining(tc.fileName).First()
			if err := uiauto.Combine("try to trash a file from "+tc.name,
				tc.openDirFunc,
				filesApp.TrashFileOrFolder(ew, tc.fileName),
				filesApp.OpenTrash(),
				filesApp.WaitUntilExists(trashedFileFinder),
			)(ctx); err != nil {
				s.Fatalf("Failed to trash a file from %q: %v", tc.name, err)
			}
		})
	}
}

// createTestData sets up and returns all the relevant data to send a file to trash
// from a directory that supports trashing.
func createTestData(folderPath, relativeTrashPath string) (sendToTrashTestData, error) {
	fileName := fmt.Sprintf("trashable-file-%d-%d.txt", time.Now().UnixNano(), rand.Intn(10000))

	trashedFilePath := filepath.Join(folderPath, relativeTrashPath, "files", fileName)
	trashedMetadataPath := filepath.Join(folderPath, relativeTrashPath, "info", fileName+".trashinfo")
	originalFilePath := filepath.Join(folderPath, fileName)
	if err := ioutil.WriteFile(originalFilePath, []byte("fake-content"), 0644); err != nil {
		return sendToTrashTestData{}, errors.Wrapf(err, "failed to create the test file inside %q", folderPath)
	}
	cleanupFunc := func(ctx context.Context, cr *chrome.Chrome, s *testing.State, name string) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_test_"+name)
		if s.HasError() {
			// If we have an error, attempt to remove the original file in case it
			// still exists.
			testing.ContextLog(ctx, s.HasError())
			if err := os.Remove(originalFilePath); err != nil {
				testing.ContextLog(ctx, "Failed to remove original file: ", err)
			}
		}
		if err := os.Remove(trashedMetadataPath); err != nil {
			testing.ContextLog(ctx, "Failed to remove info file: ", err)
		}
		if err := os.Remove(trashedFilePath); err != nil {
			testing.ContextLog(ctx, "Failed to remove trashed file: ", err)
		}
	}

	return sendToTrashTestData{
		name:        "",
		fileName:    fileName,
		openDirFunc: nil,
		cleanupFunc: cleanupFunc,
	}, nil
}
