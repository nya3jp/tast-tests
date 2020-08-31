// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/linuxfiles"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HomeDirectoryRenameFile,
		Desc:     "Test renaming a file in Linux files and container using a pre-built crostini image",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"keepState"},
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

func HomeDirectoryRenameFile(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	keyboard := s.PreValue().(crostini.PreData).Keyboard
	defer crostini.RunCrostiniPostTest(ctx,
		s.PreValue().(crostini.PreData).Container,
		s.PreValue().(crostini.PreData).Chrome.User())

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

	const (
		fileName     = "testfile.txt"
		newFileName  = "someotherdgsjtey"
		lastFileName = "lastFileName.txt"
	)

	// Create a file in container.
	if err := cont.Command(ctx, "touch", fileName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to create a file in the container: ", err)
	}

	if err := testRenameFileFromLinuxFiles(ctx, filesApp, cont, keyboard, fileName, newFileName); err != nil {
		s.Fatal("Failed to test Renaming files in Linux files: ", err)
	}

	if err := testRenameFileFromContainer(ctx, filesApp, cont, newFileName+".txt", lastFileName); err != nil {
		s.Fatal("Failed to test Renaming files in container: ", err)
	}
}

// testRenameFileFromLinuxFiles first renames a file in Linux file then checks it is also renamed in container.
func testRenameFileFromLinuxFiles(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, keyboard *input.KeyboardEventWriter, fileName, newFileName string) error {
	// Rename a file in Linux files.
	if err := filesApp.RenameFile(ctx, keyboard, linuxfiles.Title, fileName, newFileName, linuxfiles.DirName); err != nil {
		return errors.Wrapf(err, "failed to rename file %s", fileName)
	}

	// Check the old file does not exist in container.
	if err := cont.CheckFileDoesNotExistInDir(ctx, ".", fileName); err != nil {
		return err
	}

	// Check the new file exists in container.
	if err := cont.CheckFilesExistInDir(ctx, ".", fmt.Sprintf("%s.%s", newFileName, strings.Split(fileName, ".")[1])); err != nil {
		return err
	}
	return nil
}

// testRenameFileFromContainer first renames a file in container then checks it is also renamed in Linux files.
func testRenameFileFromContainer(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, fileName, newFileName string) error {
	// Rename a file in container.
	if err := cont.Command(ctx, "mv", fileName, newFileName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to rename file %s in container", fileName)
	}

	// The old file should not exist in Linux files.
	if err := filesApp.CheckFileDoesNotExist(ctx, linuxfiles.Title, fileName, linuxfiles.DirName); err != nil {
		return errors.Wrapf(err, "renamed file %s still exists in Linux files", fileName)
	}
	// The new file should exist in Linux files.
	if err := filesApp.WaitForFile(ctx, newFileName, 10*time.Second); err != nil {
		return errors.Wrapf(err, "file %s is not renamed in Linux files: ", fileName)
	}
	return nil
}
