// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

const zipPerfUITimeout = 15 * time.Second

const zipOperationTimeout = time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func: ZipPerf,
		Desc: "Measures performance for ZIP file operations",
		Contacts: []string{
			"jboulic@google.com",
			"chromeos-files-syd@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"100000_files_in_one_folder.zip", "500_small_files.zip", "various_documents.zip"},
		Timeout:      5 * time.Minute,
	})
}

func ZipPerf(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.DisableFeatures("FilesZipPack"))
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// subTestData contains strings associated to each subtest.
	type subTestData struct {
		zipBaseName    string
		selectionLabel string
		copyLabel      string
	}

	subTests := []subTestData{
		{
			zipBaseName: "100000_files_in_one_folder",
		},
		{
			zipBaseName:    "500_small_files",
			selectionLabel: "500 files selected",
			copyLabel:      "Copying 500 items to 500_small_files",
		},
		{
			zipBaseName:    "various_documents",
			selectionLabel: "102 items selected",
			copyLabel:      "Copying 102 items to various_documents",
		},
	}

	// Load ZIP files.
	for _, data := range subTests {
		zipFile := data.zipBaseName + ".zip"
		zipFileLocation := filepath.Join(filesapp.DownloadPath, zipFile)
		if err := fsutil.CopyFile(s.DataPath(zipFile), zipFileLocation); err != nil {
			s.Fatalf("Failed to copy zip file to %s: %s", zipFileLocation, err)
		}

		// Remove zip files and extraction folders when the test finishes.
		defer os.Remove(zipFileLocation)
		defer os.RemoveAll(filepath.Join(filesapp.DownloadPath, data.zipBaseName))

		// Add reading permission (-rw-r--r--).
		os.Chmod(zipFileLocation, 0644)
	}

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Define keyboard to perform keyboard shortcuts.
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer ew.Close()

	// Open the Files App.
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}

	// Wait until cpu idle before starting performance measures.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	pv := perf.NewValues()

	for _, data := range subTests {
		s.Run(ctx, data.zipBaseName, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+data.zipBaseName)
			zipFile := data.zipBaseName + ".zip"

			duration := testMountingZipFile(ctx, s, files, zipFile)

			if data.zipBaseName == "100000_files_in_one_folder" {
				// Mounting a file is an operation that is much faster than zipping and extracting.
				// This specific file is created to test this operation. Zipping and extracting
				// would not complete within the timeout set for this test.
				pv.Set(perf.Metric{
					Name:      fmt.Sprintf("tast_mount_zip_%s", data.zipBaseName),
					Unit:      "ms",
					Direction: perf.SmallerIsBetter,
				}, duration)

				return
			}

			duration = testExtractingZipFile(ctx, s, files, ew, data.zipBaseName, data.selectionLabel, data.copyLabel)

			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("tast_unzip_%s", data.zipBaseName),
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			}, duration)

			duration = testZippingFiles(ctx, tconn, s, files, ew, data.zipBaseName)

			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("tast_zip_%s", data.zipBaseName),
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			}, duration)
		})
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	} else {
		s.Log("Saved perf data")
	}
}

func testMountingZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFile string) float64 {
	if err := uiauto.Combine("open downloads and mount ZIP file",
		// Open the Downloads folder.
		files.OpenDownloads(),
		files.WaitForFile(zipFile),
		files.SelectFile(zipFile),
		// Click "Open" to mount the selected zip file.
		files.LeftClick(nodewith.Name("Open").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to test mounting the ZIP file: ", err)
	}

	// Start timer for zip file mounting operation.
	startTime := time.Now()

	if err := files.WithTimeout(zipOperationTimeout).WaitUntilExists(nodewith.Name("Files - " + zipFile).Role(role.RootWebArea))(ctx); err != nil {
		s.Fatal("Failed to find mounted ZIP file: ", err)
	}

	return float64(time.Since(startTime).Milliseconds())
}

func testExtractingZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, zipBaseName, selectionLabel, copyLabel string) float64 {
	// Define action for selecting contents of mounted file before extraction.
	selectAllInMountedFileAction := func() uiauto.Action {
		return func(ctx context.Context) error {
			return testing.Poll(ctx, func(ctx context.Context) error {
				// Select all mounted files.
				if err := ew.Accel(ctx, "ctrl+A"); err != nil {
					s.Fatal("Failed selecting files with Ctrl+A: ", err)
				}
				return files.Exists(nodewith.Name(selectionLabel).Role(role.StaticText))(ctx)
			}, &testing.PollOptions{Timeout: zipPerfUITimeout})
		}
	}

	if err := uiauto.Combine("select and copy mounted files and paste them into a new folder",
		// Move the focus to the file list.
		files.FocusAndWait(nodewith.Role(role.ListBox)),
		selectAllInMountedFileAction(),
		ew.AccelAction("ctrl+C"),
		files.OpenDownloads(),
		files.CreateFolder(ew, zipBaseName),
		// Enter the new directory.
		files.OpenFile(zipBaseName),
		// Before pasting, ensure the Files App has switched to the new location.
		files.WaitUntilExists(nodewith.Name("Files - "+zipBaseName).Role(role.RootWebArea)),
		ew.AccelAction("ctrl+V"),
	)(ctx); err != nil {
		s.Fatal("Failed to start ZIP file extraction: ", err)
	}

	// Start timer for zip file extracting operation.
	startTime := time.Now()

	// Find "Complete" within the copy notification panel, to wait for the copy operation to finish.
	if err := files.WithTimeout(zipOperationTimeout).WaitUntilExists(nodewith.Name("Complete").Role(role.StaticText).Ancestor(nodewith.Name(copyLabel).Role(role.GenericContainer)))(ctx); err != nil {
		s.Fatal("Failed to wait for end of copy operation: ", err)
	}

	// Return duration.
	return float64(time.Since(startTime).Milliseconds())
}

func testZippingFiles(ctx context.Context, tconn *chrome.TestConn, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, zipBaseName string) float64 {
	if err := uiauto.Combine("select Zip selection on all files",
		// The Files app listBox, which should be in a focused state.
		files.WaitUntilExists(nodewith.Role(role.ListBox).Focused()),
		// Select all extracted files.
		ew.AccelAction("ctrl+A"),
		// Right click on the Files app listBox.
		files.RightClick(nodewith.Role(role.ListBox).Focused()),
		// Select "Zip selection".
		files.LeftClick(nodewith.Name("Zip selection").Role(role.MenuItem)),
	)(ctx); err != nil {
		s.Fatal("Failed to start zipping files: ", err)
	}

	zipArchiverExtensionID := "dmboannefpncccogfdikhmhpmdnddgoe"

	// Wait until the Zip Archiver notification exists.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ns, err := ash.Notifications(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		for _, n := range ns {
			// Check if our notification exists.
			if strings.Contains(n.ID, zipArchiverExtensionID) {
				return nil
			}
		}
		return errors.New("notification does not exist")
	}, &testing.PollOptions{Timeout: zipPerfUITimeout}); err != nil {
		s.Fatal("Failed to find Zip archiver zipping notification: ", err)
	}

	// Start timer for zipping operation.
	startTime := time.Now()

	// Wait until the Zip Archiver notification disappears.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ns, err := ash.Notifications(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		for _, n := range ns {
			// Check if our notification exists.
			if strings.Contains(n.ID, zipArchiverExtensionID) {
				return errors.New("notification still exists")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: zipOperationTimeout}); err != nil {
		s.Fatal("Failed to wait for the Zip archiver zipping notification to disappear: ", err)
	}

	// Return duration.
	return float64(time.Since(startTime).Milliseconds())
}
