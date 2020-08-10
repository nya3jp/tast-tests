// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesApp,
		Desc:         "Tests integration between crostini and files app",
		Contacts:     []string{"sidereal@chromium.org", "cros-containers-dev@google.com"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniStable,
			},
			{
				Name:              "artifact_unstable",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
			},
			{
				Name:    "download_stretch",
				Pre:     crostini.StartedByDownloadStretch(),
				Timeout: 10 * time.Minute,
			},
			{
				Name:    "download_buster",
				Pre:     crostini.StartedByDownloadBuster(),
				Timeout: 10 * time.Minute,
			},
		},
	})
}

const (
	fileName1 = "test_file"
	fileName2 = "test_file_2"
	fileName3 = "test_file_3"
	dirName   = "test_folder"
)

func FilesApp(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	keyboard := s.PreValue().(crostini.PreData).Keyboard

	// Make sure we clean up any lingering files in the home
	// directory if the test fails.
	defer cont.Cleanup(ctx, ".")

	f, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open the files app: ", err)
	}
	defer f.Close(ctx)
	err = f.OpenLinuxFiles(ctx)
	if err != nil {
		s.Fatal("Failed to open Linux files folder: ", err)
	}

	// Test the creating a file on one side is reflected on the
	// other. There's no way to create a file in the files app,
	// but we can create a folder, so do that instead.
	err = cont.Command(ctx, "touch", fileName1).Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to create file in container: ", err)
	}
	err = f.WaitForFile(ctx, fileName1, 30*time.Second)
	if err != nil {
		s.Fatal("File in container didn't appear in files app: ", err)
	}
	err = f.SelectContextMenu(ctx, fileName1, "New folder")
	if err != nil {
		s.Fatal("Couldn't create new folder in files app: ", err)
	}
	err = enterFileName(ctx, keyboard, dirName)
	if err != nil {
		s.Fatal("Failed to enter dir name in files app: ", err)
	}
	err = cont.CheckFilesExistInDir(ctx, ".", dirName)
	if err != nil {
		s.Fatal("Folder created in files app not reflected in container: ", err)
	}

	// Test renaming files. Files renamed in the container should
	// appear in the files app and vice versa.
	err = cont.Command(ctx, "mv", fileName1, fileName2).Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to rename file in container: ", err)
	}
	err = f.WaitForFile(ctx, fileName2, 30*time.Second)
	if err != nil {
		s.Fatal("File renamed in container didn't appear in files app: ", err)
	}
	err = f.SelectContextMenu(ctx, fileName2, "Rename")
	if err != nil {
		s.Fatal("Couldn't select rename in files app: ", err)
	}
	err = enterFileName(ctx, keyboard, fileName3)
	if err != nil {
		s.Fatal("Failed to enter new file name in files app: ", err)
	}
	err = cont.CheckFilesExistInDir(ctx, ".", fileName3)
	if err != nil {
		s.Fatal("Rename in files app not reflected in container: ", err)
	}

	// Test deleting files.
	err = f.DeleteFileOrFolder(ctx, dirName)
	if err != nil {
		s.Fatal("Failed to delete test folder: ", err)
	}
	err = cont.CheckFileDoesNotExistInDir(ctx, ".", dirName)
	if err != nil {
		s.Fatal("Deleted folder still present in container: ", err)
	}
	err = cont.Command(ctx, "rm", fileName3).Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to delete file in container: ", err)
	}
	err = f.WaitForFileGone(ctx, fileName3, 30*time.Second)
	if err != nil {
		s.Fatal("Deleted file still present in files app: ", err)
	}
}

func enterFileName(ctx context.Context, keyboard *input.KeyboardEventWriter, filename string) error {
	if err := keyboard.Accel(ctx, "Ctrl+A"); err != nil {
		return errors.Wrap(err, "failed to select input box")
	}
	if err := keyboard.Type(ctx, filename); err != nil {
		return errors.Wrap(err, "failed to type filename")
	}
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		return errors.Wrap(err, "failed to type enter")
	}
	return nil
}
