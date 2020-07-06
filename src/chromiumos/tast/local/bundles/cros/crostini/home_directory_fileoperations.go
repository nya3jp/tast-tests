// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	// These two lines should be deleted once 2275643 is merged
	linuxfiles      = "Linux files"
	filesLinuxfiles = "Files - " + linuxfiles

	testzipfilename = "home_directory_fileoperations.zip"
	newzipfilename  = "test.zip"
)

var testfiles = []string{"android-cts-verifier-9.0_r1-linux_x86-x86.zip", "Manual.txt", "user_guide2_draft.pdf", "crowd1080.mp4", "dawn-environment-fall-906759.jpg", "clouds-daylight-fog-1296396.jpg"}

func init() {
	testing.AddTest(&testing.Test{
		Func:     HomeDirectoryFileoperations,
		Desc:     "Run a test against operations (add, remove, rename) on files in default share folder and container using a pre-built crostini image",
		Contacts: []string{"jinrongwu@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
		Data:     []string{testzipfilename},
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

func HomeDirectoryFileoperations(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Root.Release(ctx)

	// Define keyboard to perform keyboard shortcuts.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer keyboard.Close()

	// Download test zip file.
	testFileLocation := filepath.Join(filesapp.DownloadPath, testzipfilename)
	if err := fsutil.CopyFile(s.DataPath(testzipfilename), testFileLocation); err != nil {
		s.Fatalf("Failed to copy the test image to %s: %s", testFileLocation, err)
	}
	defer os.Remove(testFileLocation)

	// Rename the file because the original file name is too long that filesapp could not find it.
	os.Rename(filepath.Join(filesapp.DownloadPath, testzipfilename), filepath.Join(filesapp.DownloadPath, newzipfilename))

	s.Log("Test copying files to Linux files")
	if err := testCopyFilesToLinuxfiles(ctx, filesApp, keyboard, cont); err != nil {
		s.Fatal("Failed to test copying files in Linux files: ", err)
	}

	s.Log("Test deleting a random file in Linux files and it should also be deleted from container")
	if err := testDeleteFilesFromLinuxfiles(ctx, tconn, filesApp, cont); err != nil {
		s.Fatal("Failed to test deleting files to Linux files: ", err)
	}

	s.Log("Test deleting a random file in container and it should also be deleted from Linux files")
	if err := testDeleteFilesFromContainer(ctx, filesApp, cont); err != nil {
		s.Fatal("Failed to test deleting files in container: ", err)
	}

	s.Log("Test renaming a random file in Linux files and it should also be renamed in container")
	if err := testRenameFilesFromLinuxfiles(ctx, filesApp, cont, keyboard); err != nil {
		s.Fatal("Failed to test Renaming files in Linux files: ", err)
	}

	s.Log("Test renaming a random file in container and it should also be renamed in Linux files")
	if err := testRenameFilesFromContainer(ctx, filesApp, cont); err != nil {
		s.Fatal("Failed to test Renaming files in container: ", err)
	}
}

// testCopyFilesToLinuxfiles first copies then checks the copied files exist in container.
// Return error if any.
func testCopyFilesToLinuxfiles(ctx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, cont *vm.Container) error {
	// Mount the test zip file and copy content to Linux files.
	if err := copyFilesToLinuxfiles(ctx, filesApp, keyboard); err != nil {
		errors.Wrap(err, "failed to copy test files to Linux files")
		return err
	}

	// Check that the files are listed in container.
	cmd := cont.Command(ctx, "ls", " .")
	result, err := cmd.Output()
	if err != nil {
		errors.Wrap(err, "failed to list the content of home directory in container")
		return err
	}
	fileslist := string(result)
	for _, file := range testfiles {
		if strings.Contains(fileslist, file) != true {
			errors.Wrapf(err, "failed to find %s in container", file)
			return err
		}
	}
	return nil
}

// copyFilesToLinuxfiles mounts zip file and copis to Linux files.
// Return error if any.
func copyFilesToLinuxfiles(ctx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter) error {
	if err := filesApp.OpenDownloads(ctx); err != nil {
		return err
	}
	// Select the downloaded zip file.
	if err := filesApp.WaitForFile(ctx, newzipfilename, 10*time.Second); err != nil {
		return err
	}

	// Right click the file and select open.
	if err := filesApp.SelectContextMenu(ctx, newzipfilename, filesapp.Open); err != nil {
		return err
	}

	// Wait the first file to display.
	if err := filesApp.WaitForFile(ctx, testfiles[0], 10*time.Second); err != nil {
		errors.Wrap(err, "failed to find the first file")
		return err
	}

	// Select all.
	if err := keyboard.Accel(ctx, "ctrl+A"); err != nil {
		return err
	}

	// Copy.
	if err := keyboard.Accel(ctx, "ctrl+C"); err != nil {
		return err
	}

	// Open "Linux files" to paste.
	if err := filesApp.OpenDir(ctx, linuxfiles, filesLinuxfiles); err != nil {
		return err
	}
	// Paste.
	if err := keyboard.Accel(ctx, "ctrl+V"); err != nil {
		return err
	}
	// Wait for the copy operation to finish.
	params := ui.FindParams{
		Name: "Copied to " + linuxfiles + ".",
		Role: ui.RoleTypeStaticText,
	}

	if err := filesApp.Root.WaitUntilDescendantExists(ctx, params, 5*time.Minute); err != nil {
		return err
	}
	return nil
}

// testDeleteFilesFromLinuxfiles first deletes a file in Linux files then checks it is also deleted in container.
// Return error if any.
func testDeleteFilesFromLinuxfiles(ctx context.Context, tconn *chrome.TestConn, filesApp *filesapp.FilesApp, cont *vm.Container) error {
	// Delete a file randomly in Linux file and it should also be deleted from container.
	i := rand.Intn(len(testfiles))
	if err := filesApp.SelectContextMenu(ctx, testfiles[i], filesapp.Delete); err != nil {
		errors.Wrapf(err, "failed to delete %s in Linux files", testfiles[i])
		return err
	}

	filesApp.Root.Update(ctx)
	ui.WaitForLocationChangeCompleted(ctx, tconn)
	params := ui.FindParams{
		ClassName: "cr-dialog-ok",
		Name:      filesapp.Delete,
		Role:      ui.RoleTypeButton,
	}
	delete, err := filesApp.Root.DescendantWithTimeout(ctx, params, 15*time.Second)
	if err != nil {
		return err
	}
	// Click button "Delete".
	if err := delete.LeftClick(ctx); err != nil {
		errors.Wrapf(err, "failed to click button Delete on file %s ", testfiles[i])
		return err
	}

	// Check the file has been deleted in container.
	testing.Sleep(ctx, 5*time.Second)
	cmd := cont.Command(ctx, "ls", " .")
	result, err := cmd.Output()
	if err != nil {
		errors.Wrap(err, "failed to list the content of home directory in container")
		return err
	}
	fileslist := string(result)
	if strings.Contains(fileslist, testfiles[i]) {
		return errors.Errorf("failed to delete file %s in container through deleting it from Linux files", testfiles[i])
	}

	// Delete the filename from the array.
	testfiles[i] = testfiles[len(testfiles)-1]
	testfiles = testfiles[:len(testfiles)-1]
	return nil
}

// testDeleteFilesFromContainer first deletes a file in container then checks it is also deleted in Linux files.
// Return error if any.
func testDeleteFilesFromContainer(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container) error {
	// Delete the randomly selected file in container.
	i := rand.Intn(len(testfiles))
	if err := cont.Command(ctx, "rm", "-f", testfiles[i]).Run(testexec.DumpLogOnError); err != nil {
		errors.Wrapf(err, "failed to delete file %s in container", testfiles[i])
		return err
	}

	// Check the file does not exist in Linux files.
	if err := checkFileNotExits(ctx, filesApp, testfiles[i]); err != nil {
		errors.Wrapf(err, "failed to delete file %s in Linux files through deleting it from container", testfiles[i])
		return err
	}

	// Delete the filename from the array.
	testfiles[i] = testfiles[len(testfiles)-1]
	testfiles = testfiles[:len(testfiles)-1]
	return nil
}

// testRenameFilesFromLinuxfiles first renames a file in Linux file then checks it is also renamed in container.
// Return error if any.
func testRenameFilesFromLinuxfiles(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, keyboard *input.KeyboardEventWriter) error {
	i := rand.Intn(len(testfiles))
	// Rename the randomly selected file in Linux files.
	newFileName := "someotherdgsjtey"
	if err := renameFile(ctx, filesApp, keyboard, testfiles[i], newFileName); err != nil {
		errors.Wrapf(err, "failed to rename file %s", testfiles[i])
		return err
	}

	// Check the old file does not exist and the new file exists in container.
	cmd := cont.Command(ctx, "ls", " .")
	result, err := cmd.Output()
	if err != nil {
		errors.Wrap(err, "failed to list the content of home directory in container")
		return err
	}
	fileslist := string(result)
	if strings.Contains(fileslist, testfiles[i]) || !strings.Contains(fileslist, newFileName) {
		errors.Wrapf(err, "failed to rename file %s in container through renaming it from Linux files", testfiles[i])
		return errors.New("Rename failed")
	}

	// Change the filename in the array.
	parts := strings.Split(testfiles[i], ".")
	if len(parts) > 1 {
		newFileName = newFileName + "." + parts[len(parts)-1]
	}
	testfiles[i] = newFileName
	return nil
}

// testRenameFilesFromContainer first renames a file in container then checks it is also renamed in Linux files.
// Return error if any.
func testRenameFilesFromContainer(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container) error {
	i := rand.Intn(len(testfiles))
	// Rename the randomly selected file in container.
	newFileName := "newfilename" + testfiles[i]
	if err := cont.Command(ctx, "mv", testfiles[i], newFileName).Run(testexec.DumpLogOnError); err != nil {
		errors.Wrapf(err, "failed to rename file %s in container", testfiles[i])
		return err
	}
	// The old file should not exist in Linux files.
	if err := checkFileNotExits(ctx, filesApp, testfiles[i]); err != nil {
		errors.Wrapf(err, "renamed file %s still exists in Linux files", testfiles[i])
		return err
	}
	// The new file should exist in Linux files.
	if err := filesApp.WaitForFile(ctx, newFileName, 10*time.Second); err != nil {
		errors.Wrapf(err, "file %s is not renamed in Linux files: ", testfiles[i])
		return err
	}
	return nil
}

// checkFileNotExits checks a file does not exist in Linux files.
// Return error if any occurs or the file exists in Linux files.
func checkFileNotExits(ctx context.Context, filesApp *filesapp.FilesApp, filename string) error {
	// Click Refresh
	if err := filesApp.LeftClickItem(ctx, "Refresh", ui.RoleTypeButton); err != nil {
		errors.Wrapf(err, "failed to click button Refresh on Files app %s ", filename)
		return err
	}
	// Wait the UI to refresh.
	testing.Sleep(ctx, 5*time.Second)

	// Check the file has gone.
	params := ui.FindParams{
		Name: filename,
		Role: ui.RoleTypeStaticText,
	}
	if err := filesApp.Root.WaitUntilDescendantGone(ctx, params, 10*time.Second); err != nil {
		errors.Wrapf(err, "file %s still exists", filename)
		return err
	}
	return nil
}

// renameFile renames a file in Linux files.
// Return error if any.
func renameFile(ctx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, oldname, newname string) error {
	// Right click and select rename.
	if err := filesApp.SelectContextMenu(ctx, oldname, filesapp.Rename); err != nil {
		errors.Wrapf(err, "failed to select Rename in context menu for file %s in Linux files", oldname)
		return err
	}

	// Wait for rename text field.
	params := ui.FindParams{
		Role:  ui.RoleTypeTextField,
		State: map[ui.StateType]bool{ui.StateTypeEditable: true, ui.StateTypeFocusable: true, ui.StateTypeFocused: true},
	}
	if err := filesApp.Root.WaitUntilDescendantExists(ctx, params, 15*time.Second); err != nil {
		errors.Wrap(err, "failed finding rename input text field")
		return err
	}

	// Name the file with the new name.
	if err := keyboard.Type(ctx, newname); err != nil {
		errors.Wrapf(err, "failed to rename the file %s", oldname)
		return err
	}

	// Validate the new name.
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		errors.Wrapf(err, "failed validating the new name of file %s: ", newname)
		return err
	}
	return nil
}
