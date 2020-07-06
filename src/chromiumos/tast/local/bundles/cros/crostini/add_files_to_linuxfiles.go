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
	"strings"
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
		Func:     AddFilesToLinuxfiles,
		Desc:     "Tests adding files to Linux files using a pre-built crostini image",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
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

func AddFilesToLinuxfiles(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(ctx)

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

	// Check the files exist in container.
	if err := cont.CheckFileExistsInDir(ctx, ".", testFiles...); err != nil {
		s.Fatal("Failed to test copying files to Linux files: ", err)
	}

	// Compare the number of files in Linux files and that in home directory in container to make sure that no extra file is copied.
	filesInContainer, err := cont.GetFiles(ctx, ".")
	if err != nil {
		s.Fatal("Failed to get files in home directory in container: ", err)
	}
	if len(testFiles) != len(strings.Split(filesInContainer, "\n"))-1 {
		filesInLinuxfiles := ""
		for _, f := range testFiles {
			filesInLinuxfiles = fmt.Sprintf("%s %s", filesInLinuxfiles, f)
		}
		s.Fatalf("File lists in Linux files and home directory in container do not equal: files in Linux files are %s, files in container are %s", filesInLinuxfiles, filesInContainer)
	}
}

// copyFilesToLinuxfiles creates some files in Downloads and copies them to Linux files.
func copyFilesToLinuxfiles(ctx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, testFiles []string) error {
	// Open Downloads.
	if err := filesApp.OpenDownloads(ctx); err != nil {
		return errors.Wrap(err, "failed to open Downloads in Files app")
	}

	// Wait the first file to display.
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
