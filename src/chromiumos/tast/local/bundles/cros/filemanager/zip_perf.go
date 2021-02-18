// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

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
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=FilesZipMount", "--disable-features=FilesZipPack", "--disable-features=FilesZipUnpack"))
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Load ZIP files.
	zipBaseNames := []string{"100000_files_in_one_folder", "500_small_files", "various_documents"}
	for _, zipBaseName := range zipBaseNames {
		zipFile := zipBaseName + ".zip"
		zipFileLocation := filepath.Join(filesapp.DownloadPath, zipFile)
		if err := fsutil.CopyFile(s.DataPath(zipFile), zipFileLocation); err != nil {
			s.Fatalf("Failed to copy zip file to %s: %s", zipFileLocation, err)
		}

		// Remove zip files and extraction folders when the test finishes.
		defer os.Remove(zipFileLocation)
		defer os.RemoveAll(filepath.Join(filesapp.DownloadPath, zipBaseName))

		// Add reading permission (-rw-r--r--).
		os.Chmod(zipFileLocation, 0644)
	}

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

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

	for _, zipBaseName := range zipBaseNames {
		s.Run(ctx, zipBaseName, func(ctx context.Context, s *testing.State) {
			zipFile := zipBaseName + ".zip"

			duration := testMountingZipFile(ctx, s, files, zipFile)

			if zipBaseName == "100000_files_in_one_folder" {
				// Mounting a file is an operation that is much faster than zipping and extracting.
				// This specific file is created to test this operation. Zipping and extracting
				// would not complete within the timeout set for this test.
				pv.Set(perf.Metric{
					Name:      fmt.Sprintf("tast_mount_zip_%s", zipBaseName),
					Unit:      "ms",
					Direction: perf.SmallerIsBetter,
				}, duration)

				return
			}

			duration = testExtractingZipFile(ctx, s, files, ew, zipBaseName)

			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("tast_unzip_%s", zipBaseName),
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			}, duration)

			duration = testZippingFiles(ctx, tconn, s, files, ew, zipBaseName)

			pv.Set(perf.Metric{
				Name:      fmt.Sprintf("tast_zip_%s", zipBaseName),
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
	if err := uiauto.Run(ctx,
		// Open the Downloads folder.
		files.OpenDownloads(),
		// Click zip file and wait for Open button in top bar.
		files.WaitForFile(zipFile),
		// Mount zip file.
		files.SelectFile(zipFile),
		// Click "Open" to mount the selected zip file.
		files.LeftClick(nodewith.Name("Open").Role(role.Button)),
	); err != nil {
		s.Fatal("Failed to open Downloads and start mounting the ZIP file: ", err)
	}

	// Start timer for zip file mounting operation.
	startTime := time.Now()

	if err := files.WithTimeout(time.Minute).WaitUntilExists(nodewith.Name("Files - " + zipFile).Role(role.RootWebArea))(ctx); err != nil {
		s.Fatal("Failed to find mounted ZIP file: ", err)
	}

	return float64(time.Since(startTime).Milliseconds())
}

func testExtractingZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, zipBaseName string) float64 {
	// Define action for selecting contents of mounted file before extraction.
	selectAllInMountedFileAction := func() uiauto.Action {
		// Define the number of files that we expect to select for the extraction operation.
		var selectionLabel string
		switch zipBaseName {
		case "various_documents":
			selectionLabel = "102 items selected"
		case "500_small_files":
			selectionLabel = "500 files selected"
		default:
			s.Fatal("Unexpected test zip file")
		}
		return func(ctx context.Context) error {
			return testing.Poll(ctx, func(ctx context.Context) error {
				// Select all mounted files.
				if err := ew.Accel(ctx, "ctrl+A"); err != nil {
					s.Fatal("Failed selecting files with Ctrl+A: ", err)
				}
				return files.Exists(nodewith.Name(selectionLabel).Role(role.StaticText))(ctx)
			}, &testing.PollOptions{Timeout: 15 * time.Second})
		}
	}

	if err := uiauto.Run(ctx,
		// Move the focus to the file list.
		files.FocusAndWait(nodewith.Role(role.ListBox)),
		selectAllInMountedFileAction(),
		ew.AccelAction("ctrl+C"),
		files.OpenDownloads(),
		// Create receiving directory for extraction operation.
		ew.AccelAction("ctrl+E"),
		// Wait for rename text field.
		files.WaitUntilExists(nodewith.Role(role.TextField).State(state.Editable, true).State(state.Focusable, true).State(state.Focused, true)),
		// Name the new directory with the name of the zip file.
		ew.TypeAction(zipBaseName),
		ew.AccelAction("Enter"),
		files.WaitUntilGone(nodewith.Role(role.TextField).State(state.Editable, true).State(state.Focusable, true).State(state.Focused, true)),
		// Enter the new directory.
		files.OpenFile(zipBaseName),
		// Before pasting, ensure the Files App has switched to the new location.
		files.WaitUntilExists(nodewith.Name("Files - "+zipBaseName).Role(role.RootWebArea)),
		ew.AccelAction("ctrl+V"),
	); err != nil {
		s.Fatal("Failed to start ZIP file extraction: ", err)
	}

	// Start timer for zip file extracting operation.
	startTime := time.Now()

	// Similarly to the selection label, define the number of items we're expecting to copy.
	notificationRE := regexp.MustCompile("Copying (102|500) items to *")

	if err := files.WithTimeout(time.Minute).WaitUntilExists(nodewith.Name("Complete").Role(role.StaticText).Ancestor(nodewith.NameRegex(notificationRE).Role(role.GenericContainer)))(ctx); err != nil {
		s.Fatal("Failed to wait for end of copy operation: ", err)
	}

	// Return duration.
	return float64(time.Since(startTime).Milliseconds())
}

func testZippingFiles(ctx context.Context, tconn *chrome.TestConn, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, zipBaseName string) float64 {
	if err := uiauto.Run(ctx,
		// The Files app listBox, which should be in a focused state.
		files.WaitUntilExists(nodewith.Role(role.ListBox).State(state.Focused, true)),
		// Select all extracted files.
		ew.AccelAction("ctrl+A"),
		// Right click on the Files app listBox.
		files.RightClick(nodewith.Role(role.ListBox).State(state.Focused, true)),
		// Select "Zip selection".
		files.LeftClick(nodewith.Name("Zip selection").Role(role.MenuItem)),
	); err != nil {
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
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
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
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Failed to wait for the Zip archiver zipping notification to disappear: ", err)
	}

	// Return duration.
	return float64(time.Since(startTime).Milliseconds())
}
