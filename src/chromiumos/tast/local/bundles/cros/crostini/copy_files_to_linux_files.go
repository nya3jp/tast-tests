// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CopyFilesToLinuxFiles,
		Desc:         "Tests copying files to Linux files using a pre-built crostini image",
		Contacts:     []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Params:       crostini.MakeTestParams(crostini.TestInformational),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func CopyFilesToLinuxFiles(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, cont)

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Clean up the home directory in the end.
	defer func() {
		if err := cont.Cleanup(cleanupCtx, "."); err != nil {
			s.Error("Failed to remove all files in home directory in the container: ", err)
		}
	}()

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	// Define keyboard to perform keyboard shortcuts.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer keyboard.Close()

	testFiles := []string{"testfile1.txt", "testfile2.txt", "testfile3.txt"}
	s.Log("Test copying files to Linux files")

	// Create some files in Downloads.
	for _, file := range testFiles {
		path := filepath.Join(filesapp.DownloadPath, file)
		if err := ioutil.WriteFile(path, []byte("test"), 0644); err != nil {
			s.Fatal("Failed to create file in Downloads: ", err)
		}
		defer os.Remove(path)
	}

	// Copy files from Downloads to Linux files.
	if err := copyFilesToLinuxfiles(ctx, filesApp, keyboard, testFiles); err != nil {
		s.Fatal("Failed to copy test files to Linux files: ", err)
	}

	// Check the file list in home directory is equal to the copied file list.
	fileList, err := cont.GetFileList(ctx, ".")
	if err != nil {
		s.Fatal("Failed to get files in home directory in container: ", err)
	}
	if !reflect.DeepEqual(testFiles, fileList) {
		s.Fatalf("Found unexpected files in Linux files; got %q, want %q", fileList, testFiles)
	}
}

// copyFilesToLinuxfiles copies all files in Downloads to Linux files.
func copyFilesToLinuxfiles(ctx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, testFiles []string) error {
	// Open Downloads.
	if err := filesApp.OpenDownloads(ctx); err != nil {
		return errors.Wrap(err, "failed to open Downloads in Files app")
	}

	// Wait all files to display.
	for _, file := range testFiles {
		if err := filesApp.SelectFile(ctx, file); err != nil {
			return errors.Wrapf(err, "failed to find the file %s", file)
		}
	}

	// Select all files.
	if err := keyboard.Accel(ctx, "ctrl+A"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+A in Downloads")
	}

	// Copy all files.
	if err := keyboard.Accel(ctx, "ctrl+C"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+C in Downloads")
	}

	const linuxFilesFolder = "Linux files"

	// Open "Linux files" to paste.
	if err := filesApp.OpenDir(ctx, linuxFilesFolder, "Files - "+linuxFilesFolder); err != nil {
		return errors.Wrap(err, "failed to open Linux files in Files app")
	}

	// Paste all files.
	if err := keyboard.Accel(ctx, "ctrl+V"); err != nil {
		return errors.Wrap(err, "failed to press ctrl+V in Linux files")
	}

	// Wait for the copy operation to finish.
	params := ui.FindParams{
		Name: fmt.Sprintf("Copied to %s.", linuxFilesFolder),
		Role: ui.RoleTypeStaticText,
	}

	if err := filesApp.Root.WaitUntilDescendantExists(ctx, params, time.Minute); err != nil {
		return errors.Wrap(err, "Coping files to Linux files failed to finish in 1 minute")
	}
	return nil
}
