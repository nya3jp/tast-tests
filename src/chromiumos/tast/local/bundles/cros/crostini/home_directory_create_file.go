// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui"
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
		Func:     HomeDirectoryCreateFile,
		Desc:     "Test creating a file/folder in Linux files and container using a pre-built crostini image",
		Contacts: []string{"jinrongwu@chromium.org"},
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

func HomeDirectoryCreateFile(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	keyboard := s.PreValue().(crostini.PreData).Keyboard
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

	if err := filesApp.OpenLinuxFiles(ctx); err != nil {
		s.Fatal("Failed to open Linux files: ", err)
	}

	if err := testCreateFolderFromLinuxFiles(ctx, filesApp, cont, keyboard); err != nil {
		s.Fatal("Failed to test creating files in Linux files: ", err)
	}

	if err := testCreateFileFromContainer(ctx, filesApp, cont); err != nil {
		s.Fatal("Failed to test creating files in container: ", err)
	}
}

func testCreateFolderFromLinuxFiles(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, keyboard *input.KeyboardEventWriter) error {
	const dirName = "test_folder"

	// Check that the file initially does not exist in the container.
	if err := cont.CheckFileDoesNotExistInDir(ctx, ".", dirName); err != nil {
		return errors.Wrapf(err, "folder %q to be created already existed", dirName)
	}

	// Files app doesn't have a way to directly create a file, but
	// we can create a folder, which is just as good.
	if err := filesApp.CreateFolder(ctx, dirName, keyboard); err != nil {
		return errors.Wrapf(err, "failed to create new folder %q", dirName)
	}

	// Check that the file now exists in the container.
	if err := cont.CheckFilesExistInDir(ctx, ".", dirName); err != nil {
		return errors.Wrapf(err, "creation of folder %q did not propogate to container", dirName)
	}
	return nil
}

func testCreateFileFromContainer(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container) error {
	const fileName = "testfile.txt"

	// Check that the file initially does not exist in the Files app.
	if err := filesApp.CheckFileDoesNotExist(ctx, linuxfiles.Title, fileName, linuxfiles.DirName); err != nil {
		return errors.Wrap(err, "file to be created already existed")
	}

	// Create file in container.
	if err := cont.Command(ctx, "touch", fileName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to create test file in the container")
	}

	// Click Refresh.
	if err := filesApp.LeftClickItem(ctx, "Refresh", ui.RoleTypeButton); err != nil {
		return errors.Wrap(err, "failed to click button Refresh on Files app")
	}
	// Check that the file now exists in the Files app.
	if err := filesApp.WaitForFile(ctx, fileName, 10*time.Second); err != nil {
		return errors.Wrap(err, "file creation did not propogare to Files app")
	}
	return nil

}
