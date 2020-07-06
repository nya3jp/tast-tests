// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const linuxFilesFolder = "Linux files"

func init() {
	testing.AddTest(&testing.Test{
		Func:     AddFilesToLinuxfiles,
		Desc:     "Run a test to add files to Linux files using a pre-built crostini image",
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
	// Create some files in Downloads and copy them to Linux files.
	if err := createFilesAndCopyToLinuxfiles(shortCtx, filesApp, keyboard, testFiles); err != nil {
		s.Fatal("Failed to copy test files to Linux files: ", err)
	}

	// Check the files exist in container.
	if err := cont.CheckFileExistsInDir(shortCtx, ".", testFiles...); err != nil {
		s.Fatal("Failed to test copying files to Linux files: ", err)
	}
}

// createFilesAndCopyToLinuxfiles creates some files in Downloads and copies them to Linux files.
func createFilesAndCopyToLinuxfiles(shortCtx context.Context, filesApp *filesapp.FilesApp, keyboard *input.KeyboardEventWriter, testFiles []string) error {
	// Create test files in Downloads.
	for _, file := range testFiles {
		if err := ioutil.WriteFile(filepath.Join(filesapp.DownloadPath, file), []byte("test"), 0644); err != nil {
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
