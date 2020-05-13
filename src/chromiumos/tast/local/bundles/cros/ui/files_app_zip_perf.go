// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/ui/faillog"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FilesAppZipPerf,
		Desc: "Measures performance for ZIP file operations",
		Contacts: []string{
			"jboulic@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"small_files.zip", "many_small_files.zip"},
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func FilesAppZipPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	// Load test zip file.
	const zipFile = "small_files.zip"

	zipFileLocation := filepath.Join(filesapp.DownloadPath, zipFile)
	if err := fsutil.CopyFile(s.DataPath(zipFile), zipFileLocation); err != nil {
		s.Fatalf("Failed to copy zip file to %s: %s", zipFileLocation, err)
	}
	defer os.Remove(zipFileLocation)

	// Add reading permission (-rw-r--r--).
	os.Chmod(zipFileLocation, 0644)

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s, tconn)

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

	// Open the Downloads folder.
	if err := files.OpenDownloads(ctx); err != nil {
		s.Fatal("Opening Downloads folder failed: ", err)
	}

	// Click zip file and wait for Open button in top bar.
	if err := files.WaitForFile(ctx, zipFile, 15*time.Second); err != nil {
		s.Fatal("Waiting for test file failed: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	testMountingZipFile(ctx, s, zipFile, files)

	testExtractingZipFile(ctx, s, zipFile, files, ew)

	testZippingFiles(ctx, tconn, s, zipFile, files, ew)
}

func testMountingZipFile(ctx context.Context, s *testing.State, zipFile string, files *filesapp.FilesApp) {
	// Start timer for zip file mounting operation.
	startTime := time.Now()

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

	// Click on the mounted zip file.
	params = ui.FindParams{
		Name: zipFile,
		Role: ui.RoleTypeTreeItem,
	}

	mountedZipFile, err := files.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		s.Fatal("Waiting for mounted zip file failed: ", err)
	}
	defer mountedZipFile.Release(ctx)

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

	// Get and save duration.
	duration := float64(time.Since(startTime).Milliseconds())

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "tast.mount.zip.large.number.of.files",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, duration)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	} else {
		s.Log("Saved perf data")
	}
}

func testExtractingZipFile(ctx context.Context, s *testing.State, zipFile string, files *filesapp.FilesApp, ew *input.KeyboardEventWriter) {
	// Get the Files App listBox.
	filesBox, err := files.Root.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeListBox}, 15*time.Second)
	if err != nil {
		s.Fatal("Failed getting filesBox: ", err)
	}
	defer filesBox.Release(ctx)

	// Move the focus to the file list.
	if err := filesBox.LeftClick(ctx); err != nil {
		s.Fatal("Failed selecting filesBox: ", err)
	}

	// Select all mounted files.
	if err := ew.Accel(ctx, "ctrl+A"); err != nil {
		s.Fatal("Failed selecting files with Ctrl+A: ", err)
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
	// if err := ew.Accel(ctx, "ctrl+E"); err != nil {
	// 	s.Fatal("Failed copying files with Ctrl+C: ", err)
	// }

	// // Name the new directory.
	// if err := ew.Type(ctx, "extracted_data"); err != nil {
	// 	s.Fatal("Failed renaming the new directory: ", err)
	// }

	// // Validate the new directory name.
	// if err := ew.Accel(ctx, "Enter"); err != nil {
	// 	s.Fatal("Failed validating the name of the new directory: ", err)
	// }

	// Enter the new directory.
	// if err := ew.Accel(ctx, "Enter"); err != nil {
	// 	s.Fatal("Failed navigating to the new directory: ", err)
	// }

	if err := ew.Accel(ctx, "ctrl+V"); err != nil {
		s.Fatal("Failed pasting files with Ctrl+V: ", err)
	}

	// Wait for the copy operation to start.
	params := ui.FindParams{
		Name:  "Copying data…",
		State: map[ui.StateType]bool{ui.StateTypeFocusable: true},
	}
	// params := ui.FindParams{
	// 	Name: "Files feedback window",
	// 	Role: ui.RoleTypeComplementary,
	// 	State: map[ui.StateType]bool{ui.StateTypeInvisible: false},
	// }

	if err := files.Root.WaitUntilDescendantExists(ctx, params, 5*time.Second); err != nil {
		s.Fatal("Waiting for feedback panel failed: ", err)
	}
	// else {
	// 	s.Fatal("Found it!!!!!!!")
	// }

	// Wait for the files feedback window to disappear, indicating the end of the copy operation.
	// params = ui.FindParams{
	// 	Name: "Files feedback window",
	// 	Role: ui.RoleTypeComplementary,
	// 	State: map[ui.StateType]bool{ui.StateTypeInvisible: true},
	// }

	// Start timer for zip file extracting operation.
	startTime := time.Now()

	if err := files.Root.WaitUntilDescendantGone(ctx, params, 2*time.Minute); err != nil {
		s.Fatal("Waiting for feedback panel to disappear failed: ", err)
	}

	// Get and save duration.
	duration := float64(time.Since(startTime).Milliseconds())

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "tast.unzip.large.number.of.files",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, duration)
}

func testZippingFiles(ctx context.Context, tconn *chrome.TestConn, s *testing.State, zipFile string, files *filesapp.FilesApp, ew *input.KeyboardEventWriter) {
	// Navigate to the data folder.
	params := ui.FindParams{
		Name: "data",
		Role: ui.RoleTypeListBoxOption,
	}
	dataFolder, err := files.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		s.Fatal("Failed to find data folder: ", err)
	}
	defer dataFolder.Release(ctx)
	if err := dataFolder.LeftClick(ctx); err != nil {
		s.Fatal("Failed to select data folder: ", err)
	}

	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed navigating to the data folder: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	// Get the Files App listBox.
	filesBox, err := files.Root.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeListBox}, 15*time.Second)
	if err != nil {
		s.Fatal("Failed getting filesBox: ", err)
	}
	defer filesBox.Release(ctx)

	// Move the focus to the file list.
	if err := filesBox.LeftClick(ctx); err != nil {
		s.Fatal("Failed selecting filesBox: ", err)
	}

	// Select all mounted files.
	if err := ew.Accel(ctx, "ctrl+A"); err != nil {
		s.Fatal("Failed selecting files with Ctrl+A: ", err)
	}

	// Open menu item.
	if err := filesBox.RightClick(ctx); err != nil {
		s.Fatal("Failed opening menu item: ", err)
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
	testing.Poll(ctx, func(ctx context.Context) error {
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
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	// Start timer for zipping operation.
	startTime := time.Now()

	// Wait until the Zip Archiver notification disappears.
	testing.Poll(ctx, func(ctx context.Context) error {
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
	}, &testing.PollOptions{Timeout: 20 * time.Second})

	// Get and save duration.
	duration := float64(time.Since(startTime).Milliseconds())

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "tast.zip.large.number.of.files",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, duration)
}
