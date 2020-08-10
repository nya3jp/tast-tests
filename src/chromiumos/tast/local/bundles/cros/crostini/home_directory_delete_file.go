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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HomeDirectoryDeleteFile,
		Desc:     "Test deleting a file in Linux files and container using a pre-built crostini image",
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

func HomeDirectoryDeleteFile(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, cont)

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	// Open Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	const fileName = "testfile.txt"

	s.Run(ctx, "delete_from_linuxfiles", func(ctx context.Context, s *testing.State) {
		// Rename a file from Linux files and check it get renamed in container.
		if err := testDeleteFileFromLinuxFiles(ctx, filesApp, cont, fileName); err != nil {
			s.Error("Failed to test deleting files from Linux files: ", err)
		}
	})

	s.Run(ctx, "delete_from_container", func(ctx context.Context, s *testing.State) {
		// Rename a file from container and check it get renamed in Linux files.
		if err := testDeleteFileFromContainer(ctx, filesApp, cont, fileName); err != nil {
			s.Error("Failed to test deleting files in container: ", err)
		}
	})
}

// testDeleteFileFromLinuxFiles first deletes a file in Linux files then checks it is also deleted in container.
func testDeleteFileFromLinuxFiles(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, fileName string) error {
	// Create a file inside container.
	if err := cont.Command(ctx, "touch", fileName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to create a file in the container")
	}

	// Open "Linux files".
	if err := filesApp.OpenDir(ctx, linuxfiles.DirName, linuxfiles.Title); err != nil {
		return errors.Wrap(err, "failed to open Linux files")
	}

	// Delete the file in Linux file.
	if err := filesApp.DeleteFileOrFolder(ctx, fileName); err != nil {
		return errors.Wrapf(err, "failed to delete %s in Linux files", fileName)
	}

	// Check the file has been deleted in the container.
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := cont.CheckFileDoesNotExistInDir(ctx, ".", fileName); err != nil {
			return errors.Wrapf(err, "failed to delete file %s in the container through deleting it from Linux files", fileName)
		}
		return nil
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 15 * time.Second})
}

// testDeleteFileFromContainer first deletes a file in the container then checks it is also deleted in Linux files.
func testDeleteFileFromContainer(ctx context.Context, filesApp *filesapp.FilesApp, cont *vm.Container, fileName string) error {
	// Create a file in the container.
	if err := cont.Command(ctx, "touch", fileName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to create a file in the container")
	}

	// Open "Linux files".
	if err := filesApp.OpenDir(ctx, linuxfiles.DirName, linuxfiles.Title); err != nil {
		return errors.Wrap(err, "failed to open Linux files after creating files in the container")
	}

	// Click Refresh.
	if err := filesApp.LeftClickItem(ctx, "Refresh", ui.RoleTypeButton); err != nil {
		return errors.Wrap(err, "failed to click button Refresh on Files app")
	}

	// Check the newly created file is listed in Linux files.
	if err := filesApp.WaitForFile(ctx, fileName, 10*time.Second); err != nil {
		return errors.Wrap(err, "failed to list the file created from crostini in Files app: ")
	}

	// Delete the file in container.
	if err := cont.Command(ctx, "rm", "-f", fileName).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to delete file %s in container", fileName)
	}

	// Check the file does not exist in Linux files.
	if err := filesApp.CheckFileDoesNotExist(ctx, linuxfiles.Title, fileName, linuxfiles.DirName); err != nil {
		return errors.Wrapf(err, "failed to delete file %s in Linux files through deleting it from container", fileName)
	}

	return nil
}
