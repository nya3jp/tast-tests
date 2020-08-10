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

	if err := testCreateFileFromLinuxFiles(ctx, filesApp, cont, keyboard, fileName, newFileName); err != nil {
		s.Fatal("Failed to test creating files in Linux files: ", err)
	}

	if err := testCreateFileFromContainer(ctx, filesApp, cont, newFileName+".txt", lastFileName); err != nil {
		s.Fatal("Failed to test creating files in container: ", err)
	}
}

func testCreateFileFromLinuxFiles(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, keyboard *input.KeyboardEventWriter) error {
	const (
		fileName = "testfile1.txt"
	)

	// Check that the file initially does not exist in the container.
	if err := cont.CheckFileDoesNotExistInDir(ctx, ".", fileName); err != nil {
		return errors.Wrap(err, "file to be created already existed")
	}

	// Files app doesn't have a way to directly create a file, but
	// we can create a folder, which is just as good.
	if err := filesApp.CreateFolder(ctx, fileName, keyboard); err != nil {
		return errors.Wrap(err, "failed to create new folder")
	}

	// Check that the file now exists in the container.
	if err := cont.CheckFilesExistInDir(ctx, ".", fileName); err != nil {
		return errors.Wrap(err, "file creation did not propogate to container")
	}
	return nil
}

func testCreateFileFromContainer(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container) error {
	const (
		fileName = "testfile2.txt"
	)

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
		s.Fatal("Failed to click button Refresh on Files app: ", err)
	}
	// Check that the file now exists in the Files app.
	if err := filesApp.WaitForFile(ctx, fileName, 10*time.Second); err != nil {
		return errors.Wrap(err, "file creation did not propogare to Files app")
	}
	return nil

}
