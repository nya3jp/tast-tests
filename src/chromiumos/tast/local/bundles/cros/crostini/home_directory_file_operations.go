// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const linuxFilesFolder = "Linux files"

func init() {
	testing.AddTest(&testing.Test{
		Func:     HomeDirectoryFileOperations,
		Desc:     "Run a test against operations (add, remove, rename) on files in default share folder and container using a pre-built crostini image",
		Contacts: []string{"jinrongwu@google.org", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "artifact",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:              "artifact_unstable",
			Pre:               crostini.StartedByArtifact(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniUnstable,
		}, {
			Name:    "download_stretch",
			Pre:     crostini.StartedByDownloadStretch(),
			Timeout: 10 * time.Minute,
		}, {
			Name:    "download_buster",
			Pre:     crostini.StartedByDownloadBuster(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func HomeDirectoryFileOperations(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	shortCtx, shortCancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer shortCancel()

	// Open Files app.
	filesApp, err := filesapp.Launch(shortCtx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(shortCtx)

	// Define keyboard to perform keyboard shortcuts.
	keyboard, err := input.Keyboard(shortCtx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer keyboard.Close()

	testFiles := []string{"testfile1.txt", "testfile2.txt", "testfile3.txt"}
	s.Log("Test copying files to Linux files")
	if err := testCopyFilesToLinuxfiles(shortCtx, filesApp, keyboard, cont, testFiles); err != nil {
		s.Fatal("Failed to test copying files in Linux files: ", err)
	}

	s.Log("Test deleting a file in Linux files and it should also be deleted from container")
	if err := testDeleteFileFromLinuxFiles(shortCtx, filesApp, cont, testFiles[len(testFiles)-1]); err != nil {
		s.Fatal("Failed to test deleting files from Linux files: ", err)
	}
	// Delete the fileName from the array.
	testFiles = testFiles[:len(testFiles)-1]

	s.Log("Test deleting a file in container and it should also be deleted from Linux files")
	if err := testDeleteFileFromContainer(shortCtx, filesApp, cont, testFiles[len(testFiles)-1]); err != nil {
		s.Fatal("Failed to test deleting files in container: ", err)
	}
	// Delete the fileName from the array.
	testFiles = testFiles[:len(testFiles)-1]

	s.Log("Test renaming a file in Linux files and it should also be renamed in container")
	i := len(testFiles) - 1
	newFileName := "someotherdgsjtey"
	if err := testRenameFileFromLinuxfiles(shortCtx, filesApp, cont, keyboard, testFiles[i], newFileName); err != nil {
		s.Fatal("Failed to test Renaming files in Linux files: ", err)
	}
	// Change the fileName in the array.
	parts := strings.Split(testFiles[i], ".")
	if len(parts) > 1 {
		newFileName = newFileName + "." + parts[len(parts)-1]
	}
	testFiles[i] = newFileName

	s.Log("Test renaming a file in container and it should also be renamed in Linux files")
	newFileName = "newfilename" + testFiles[len(testFiles)-1]
	if err := testRenameFileFromContainer(shortCtx, filesApp, cont, testFiles[len(testFiles)-1], newFileName); err != nil {
		s.Fatal("Failed to test Renaming files in container: ", err)
	}
}

// testCopyFilesToLinuxfiles first copies then checks the copied files exist in container.
func testCopyFilesToLinuxfiles(shortCtx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, cont *vm.Container, testFiles []string) error {
	// Create some files in Downloads and copy them to Linux files.
	if err := copyFilesToLinuxfiles(shortCtx, filesApp, keyboard, testFiles); err != nil {
		return errors.Wrap(err, "failed to copy test files to Linux files")
	}

	// Check the files exist in container.
	if err := cont.CheckFileExistsInDir(shortCtx, ".", testFiles...); err != nil {
		return err
	}
	return nil
}

// copyFilesToLinuxfiles mounts zip file and copis to Linux files.
func copyFilesToLinuxfiles(shortCtx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, testFiles []string) error {
	// Create test files in Downloads.
	for _, file := range testFiles {
		if err := ioutil.WriteFile(filepath.Join(filesapp.DownloadPath, file), []byte("It is a test string to be written to some files."), 0644); err != nil {
			return err
		}
		defer os.Remove(filepath.Join(filesapp.DownloadPath, file))
	}

	// Open Downloads.
	if err := filesApp.OpenDownloads(shortCtx); err != nil {
		return err
	}

	// Wait the first file to display.
	if err := filesApp.SelectFile(shortCtx, testFiles[0]); err != nil {
		return errors.Wrap(err, "failed to find the first file")
	}

	// Select all.
	if err := keyboard.Accel(shortCtx, "ctrl+A"); err != nil {
		return err
	}

	// Copy.
	if err := keyboard.Accel(shortCtx, "ctrl+C"); err != nil {
		return err
	}

	// Open "Linux files" to paste.
	if err := filesApp.OpenDir(shortCtx, linuxFilesFolder, "Files - "+linuxFilesFolder); err != nil {
		return err
	}
	// Paste.
	if err := keyboard.Accel(shortCtx, "ctrl+V"); err != nil {
		return err
	}
	// Wait for the copy operation to finish.
	params := ui.FindParams{
		Name: "Copied to " + linuxFilesFolder + ".",
		Role: ui.RoleTypeStaticText,
	}

	if err := filesApp.Root.WaitUntilDescendantExists(shortCtx, params, 10*time.Second); err != nil {
		return err
	}
	return nil
}

// testDeleteFileFromLinuxFiles first deletes a file in Linux files then checks it is also deleted in container.
func testDeleteFileFromLinuxFiles(shortCtx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, fileName string) error {
	// Delete the first file in Linux file and it should also be deleted from container.
	if err := filesApp.DeleteAFileOrFolder(shortCtx, fileName); err != nil {
		return errors.Wrapf(err, "failed to delete %s in Linux files", fileName)
	}

	// Check the file has been deleted in container.
	return testing.Poll(shortCtx, func(shortCtx context.Context) error {
		if err := cont.CheckFileDoesNotExistInDir(shortCtx, ".", fileName); err != nil {
			return errors.Wrapf(err, "failed to delete file %s in container through deleting it from Linux files", fileName)
		}
		return nil
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 15 * time.Second})
}

// testDeleteFileFromContainer first deletes a file in container then checks it is also deleted in Linux files.
func testDeleteFileFromContainer(shortCtx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, fileName string) error {
	// Delete the file in container.
	if err := cont.Command(shortCtx, "rm", "-f", fileName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to delete file %s in container", fileName)
	}

	// Check the file does not exist in Linux files.
	if err := checkFileDoesNotExist(shortCtx, filesApp, fileName); err != nil {
		return errors.Wrapf(err, "failed to delete file %s in Linux files through deleting it from container", fileName)
	}

	return nil
}

// testRenameFileFromLinuxfiles first renames a file in Linux file then checks it is also renamed in container.
func testRenameFileFromLinuxfiles(shortCtx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, keyboard *input.KeyboardEventWriter, fileName, newFileName string) error {
	// Rename the first file in Linux files.
	if err := renameFile(shortCtx, filesApp, keyboard, fileName, newFileName); err != nil {
		return errors.Wrapf(err, "failed to rename file %s", fileName)
	}

	// Check the old file does not exist in container.
	if err := cont.CheckFileDoesNotExistInDir(shortCtx, ".", fileName); err != nil {
		return err
	}

	// Check the new file exists in container.
	if err := cont.CheckFileExistsInDir(shortCtx, ".", newFileName); err != nil {
		return err
	}
	return nil
}

// testRenameFileFromContainer first renames a file in container then checks it is also renamed in Linux files.
func testRenameFileFromContainer(shortCtx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, fileName, newFileName string) error {
	// Rename the first selected file in container.
	if err := cont.Command(shortCtx, "mv", fileName, newFileName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to rename file %s in container", fileName)
	}
	// The old file should not exist in Linux files.
	if err := checkFileDoesNotExist(shortCtx, filesApp, fileName); err != nil {
		return errors.Wrapf(err, "renamed file %s still exists in Linux files", fileName)
	}
	// The new file should exist in Linux files.
	if err := filesApp.WaitForFile(shortCtx, newFileName, 10*time.Second); err != nil {
		return errors.Wrapf(err, "file %s is not renamed in Linux files: ", fileName)
	}
	return nil
}

// checkFileDoesNotExist checks a file does not exist in Linux files.
// Return error if any occurs or the file exists in Linux files.
func checkFileDoesNotExist(shortCtx context.Context, filesApp *filesapp.FilesApp, fileName string) error {
	// Click Refresh.
	if err := filesApp.LeftClickItem(shortCtx, "Refresh", ui.RoleTypeButton); err != nil {
		return errors.Wrapf(err, "failed to click button Refresh on Files app %s ", fileName)
	}

	// Check the file has gone.
	return testing.Poll(shortCtx, func(shortCtx context.Context) error {
		params := ui.FindParams{
			Name: fileName,
			Role: ui.RoleTypeStaticText,
		}
		if err := filesApp.Root.WaitUntilDescendantGone(shortCtx, params, 10*time.Second); err != nil {
			return errors.Wrapf(err, "file %s still exists", fileName)
		}
		return nil
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 15 * time.Second})
}

// renameFile renames a file in Linux files.
func renameFile(shortCtx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, oldname, newname string) error {
	// Right click and select rename.
	if err := filesApp.SelectContextMenu(shortCtx, oldname, filesapp.Rename); err != nil {
		return errors.Wrapf(err, "failed to select Rename in context menu for file %s in Linux files", oldname)
	}

	// Wait for rename text field.
	params := ui.FindParams{
		Role:  ui.RoleTypeTextField,
		State: map[ui.StateType]bool{ui.StateTypeEditable: true, ui.StateTypeFocusable: true, ui.StateTypeFocused: true},
	}
	if err := filesApp.Root.WaitUntilDescendantExists(shortCtx, params, 15*time.Second); err != nil {
		return errors.Wrap(err, "failed finding rename input text field")
	}

	// Name the file with the new name.
	if err := keyboard.Type(shortCtx, newname); err != nil {
		return errors.Wrapf(err, "failed to rename the file %s", oldname)
	}

	// Validate the new name.
	if err := keyboard.Accel(shortCtx, "Enter"); err != nil {
		return errors.Wrapf(err, "failed validating the new name of file %s: ", newname)
	}
	return nil
}
