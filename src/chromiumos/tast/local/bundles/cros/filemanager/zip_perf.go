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
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/filesapp"
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
		Pre:          chrome.LoggedIn(),
		Timeout:      5 * time.Minute,
	})
}

func ZipPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

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
	// Instead of closing the Files App, just release the memory reference.
	// Otherwise, when this test fails, the screenshot will be of an empty desktop/closing app.
	defer files.Root.Release(ctx)

	// Wait until cpu idle before starting performance measures.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	pv := perf.NewValues()

	for _, zipBaseName := range []string{"100000_files_in_one_folder", "500_small_files", "various_documents"} {
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

		duration := testMountingZipFile(ctx, s, files, zipFile)

		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("tast_mount_zip_%s", zipBaseName),
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, duration)

		if zipBaseName == "100000_files_in_one_folder" {
			// Mounting a file is an operation that is much faster than zipping and extracting.
			// This specific file is created to better test this operation. However zipping and
			// extracting would not complete within the timeout set for this test.
			continue
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

		// Open the Downloads folder.
		if err := files.OpenDownloads(ctx); err != nil {
			s.Fatal("Opening Downloads folder failed: ", err)
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	} else {
		s.Log("Saved perf data")
	}
}

func testMountingZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, zipFile string) float64 {
	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}

	// Click zip file and wait for Open button in top bar.
	if err := files.WaitForFile(ctx, zipFile, 15*time.Second); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}

	// Mount zip file.
	if err := files.SelectFile(ctx, zipFile); err != nil {
		s.Fatal("Failed to mount zip file: ", err)
	}

	params := ui.FindParams{
		Name: "Open",
		Role: ui.RoleTypeButton,
	}

	open, err := files.Root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find Open menu item: ", err)
	}
	defer open.Release(ctx)

	if err := open.LeftClick(ctx); err != nil {
		s.Fatal("Mounting zip file failed: ", err)
	}

	// Start timer for zip file mounting operation.
	startTime := time.Now()

	// Click on the mounted zip file.
	params = ui.FindParams{
		Name: zipFile,
		Role: ui.RoleTypeTreeItem,
	}

	mountedZipFile, err := files.Root.DescendantWithTimeout(ctx, params, time.Minute)
	if err != nil {
		s.Fatal("Waiting for mounted zip file failed: ", err)
	}
	defer mountedZipFile.Release(ctx)

	duration := float64(time.Since(startTime).Milliseconds())

	if err := mountedZipFile.LeftClick(ctx); err != nil {
		s.Fatal("Selecting mounted zip file failed: ", err)
	}

	// Ensure that the Files App is displaying the content of the mounted zip file.
	params = ui.FindParams{
		Name: "Files - " + zipFile,
		Role: ui.RoleTypeRootWebArea,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 15*time.Second); err != nil {
		s.Fatal("Opening mounted zip file failed: ", err)
	}

	return duration
}

func testExtractingZipFile(ctx context.Context, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, zipBaseName string) float64 {
	// Get the Files App listBox.
	filesBox, err := files.Root.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeListBox}, 15*time.Second)
	if err != nil {
		s.Fatal("Failed getting filesBox: ", err)
	}
	defer filesBox.Release(ctx)

	// Move the focus to the file list.
	if err := filesBox.FocusAndWait(ctx, 15*time.Second); err != nil {
		s.Fatal("Failed selecting filesBox: ", err)
	}

	// Define the number of files that we expect to select for extraction and zipping operations.
	var selectionLabel string
	switch zipBaseName {
	case "various_documents":
		selectionLabel = "102 items selected"
	case "500_small_files":
		selectionLabel = "500 files selected"
	default:
		s.Fatal("Unexpected test zip file")
	}

	// Ensure that the right number of files is selected.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Select all mounted files.
		if err := ew.Accel(ctx, "ctrl+A"); err != nil {
			s.Fatal("Failed selecting files with Ctrl+A: ", err)
		}

		// Ensure that the right number of files is selected.
		params := ui.FindParams{
			Name: selectionLabel,
			Role: ui.RoleTypeStaticText,
		}
		selectionLabelFound, err := files.Root.DescendantExists(ctx, params)
		if err != nil {
			return err
		}
		if !selectionLabelFound {
			return errors.New("expected selection label still not found")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Cannot check that the right number of files is selected: ", err)
	}

	// Copy.
	if err := ew.Accel(ctx, "ctrl+C"); err != nil {
		s.Fatal("Failed copying files with Ctrl+C: ", err)
	}

	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}

	// Create receiving directory for extraction operation.
	if err := ew.Accel(ctx, "ctrl+E"); err != nil {
		s.Fatal("Failed renaming folder with Ctrl+E: ", err)
	}

	// Wait for rename text field.
	params := ui.FindParams{
		Role:  ui.RoleTypeTextField,
		State: map[ui.StateType]bool{ui.StateTypeEditable: true, ui.StateTypeFocusable: true, ui.StateTypeFocused: true},
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 15*time.Second); err != nil {
		s.Fatal("Failed finding rename input text field: ", err)
	}

	// Name the new directory with the name of the zip file.
	if err := ew.Type(ctx, zipBaseName); err != nil {
		s.Fatal("Failed renaming the new directory: ", err)
	}

	// Validate the new directory name.
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed validating the name of the new directory: ", err)
	}

	// Wait for the input field to disappear.
	if err := files.Root.WaitUntilDescendantGone(ctx, params, 15*time.Second); err != nil {
		s.Fatal("Failed waiting for input field to disappear: ", err)
	}

	// Enter the new directory.
	if err := files.OpenFile(ctx, zipBaseName); err != nil {
		s.Fatal("Failed navigating to the new directory: ", err)
	}

	// Before pasting, ensure the Files App has switched to the new location.
	params = ui.FindParams{
		Name: "Files - " + zipBaseName,
		Role: ui.RoleTypeRootWebArea,
	}
	if err := files.Root.WaitUntilDescendantExists(ctx, params, 15*time.Second); err != nil {
		s.Fatal("Opening "+zipBaseName+" folder failed: ", err)
	}

	if err := ew.Accel(ctx, "ctrl+V"); err != nil {
		s.Fatal("Failed pasting files with Ctrl+V: ", err)
	}

	// Start timer for zip file extracting operation.
	startTime := time.Now()

	// Wait for the copy operation to finish.
	params = ui.FindParams{
		Name: "Copied to " + zipBaseName + ".",
		Role: ui.RoleTypeStaticText,
	}

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 5*time.Minute); err != nil {
		s.Fatal("Waiting for end of copy operation failed: ", err)
	}

	// Return duration.
	return float64(time.Since(startTime).Milliseconds())
}

func testZippingFiles(ctx context.Context, tconn *chrome.TestConn, s *testing.State, files *filesapp.FilesApp, ew *input.KeyboardEventWriter, zipBaseName string) float64 {
	// Get the Files App listBox, which should be in a focused state.
	params := ui.FindParams{
		Role:  ui.RoleTypeListBox,
		State: map[ui.StateType]bool{ui.StateTypeFocused: true},
	}

	filesBox, err := files.Root.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeListBox}, 15*time.Second)
	if err != nil {
		s.Fatal("Failed getting filesBox: ", err)
	}
	defer filesBox.Release(ctx)

	// Select all extracted files.
	if err := ew.Accel(ctx, "ctrl+A"); err != nil {
		s.Fatal("Failed selecting files with Ctrl+A: ", err)
	}

	// Open menu item.
	if err := filesBox.RightClick(ctx); err != nil {
		s.Fatal("Failed opening menu item: ", err)
	}

	// Wait for location change events to be propagated (b/161438238).
	if err := ui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for location change completed: ", err)
	}

	// Zip selection.
	params = ui.FindParams{
		Name: "Zip selection",
		Role: ui.RoleTypeMenuItem,
	}
	zipSelection, err := files.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		s.Fatal("Failed zipping files: ", err)
	}
	defer zipSelection.Release(ctx)

	if err := zipSelection.LeftClick(ctx); err != nil {
		s.Fatal("Failed unzipping menu item: ", err)
	}

	zipArchiverExtensionID := "dmboannefpncccogfdikhmhpmdnddgoe"

	// Wait until the Zip Archiver notification exists.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ns, err := ash.VisibleNotifications(ctx, tconn)
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
		ns, err := ash.VisibleNotifications(ctx, tconn)
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
