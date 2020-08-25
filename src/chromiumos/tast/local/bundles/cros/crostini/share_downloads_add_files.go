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
	"chromiumos/tast/local/bundles/cros/crostini/listset"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/sharedfolders"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShareDownloadsAddFiles,
		Desc:     "Test sharing Downloads with Crostini",
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

func ShareDownloadsAddFiles(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer crostini.RunCrostiniPostTest(ctx, cont)

	sharedFolders := sharedfolders.NewSharedFolders()
	// Clean up in the end.
	defer func() {
		if err := sharedFolders.UnshareAll(cleanupCtx, tconn, cont); err != nil {
			s.Error("Failed to unshare all folders: ", err)
		}
		files, err := ioutil.ReadDir(filesapp.DownloadPath)
		if err != nil {
			s.Fatal("Failed to read files in Downloads: ", err)
		}
		for _, f := range files {
			os.RemoveAll(filepath.Join(filesapp.DownloadPath, f.Name()))
		}
	}()

	// Open the Files app.
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer filesApp.Close(cleanupCtx)

	// Right click My files and select Share with Linux.
	if err = filesApp.SelectDirectoryContextMenuItem(ctx, filesapp.Downloads, sharedfolders.ShareWithLinux); err != nil {
		s.Fatal("Failed to share Downloads with Crostini: ", err)
	}
	sharedFolders.AddFolder(sharedfolders.SharedDownloads)

	// Check the file list in the container. It takes sometime for the container to mount the shared folder.
	// This step is necessary, without this step,
	// the following test will fail because it runs faster than mounting.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		list, err := cont.GetFileList(ctx, sharedfolders.MountPath)
		if err != nil {
			return err
		}
		if err := listset.CheckListsMatch(list, "fonts", sharedfolders.MountFolderMyFiles); err != nil {
			return err
		}

		list, err = cont.GetFileList(ctx, sharedfolders.MountPathMyFiles)
		if err != nil {
			return err
		}
		if err := listset.CheckListsMatch(list, filesapp.Downloads); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to verify file list in container after sharing Downloads: ", err)
	}

	s.Run(ctx, "add_files_to_downloads", func(ctx context.Context, s *testing.State) {
		const (
			testFile   = "testD.txt"
			testFolder = "testFolderD"
			testString = "This is a test string. Downloads. \n"
		)

		// Add a file and a folder in Downloads.
		filePath := filepath.Join(filesapp.DownloadPath, testFile)
		if err := ioutil.WriteFile(filePath, []byte(testString), 0644); err != nil {
			s.Fatal("Failed to create file in Downloads: ", err)
		}
		folderPath := filepath.Join(filesapp.DownloadPath, testFolder)
		if err := os.MkdirAll(folderPath, 0755); err != nil {
			s.Fatal("Failed to create test folder in Downloads: ", err)
		}

		// Check file list in the container.
		fileList, err := cont.GetFileList(ctx, filepath.Join(sharedfolders.MountPathMyFiles, filesapp.Downloads))
		if err != nil {
			s.Fatal("Failed to get file list of /mnt/chromeos/MyFiles/Downloads: ", err)
		}
		if err := listset.CheckListsMatch(fileList, testFile, testFolder); err != nil {
			s.Fatal("Failed to verify the files list in container: ", err)
		}

		// Check the content of the test file in the container.
		if err := cont.CheckFileContent(ctx, filepath.Join(sharedfolders.MountPathMyFiles, filesapp.Downloads, testFile), testString); err != nil {
			s.Fatal("Failed to verify the content of the test file: ", err)
		}
	})

	s.Run(ctx, "add_files_to_container", func(ctx context.Context, s *testing.State) {
		const (
			testFile   = "testC.txt"
			testFolder = "testFolderC"
			testString = "This is a test string. Container. \n"
		)

		// Add a folder in the container.
		if err := cont.Command(ctx, "mkdir", filepath.Join(sharedfolders.MountPathMyFiles, filesapp.Downloads, testFolder)).Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to create a folder in : ", err)
		}

		// Create a file in a temp directory in Chrome OS and push it to the container.
		dir, err := ioutil.TempDir("", "tempDir")
		if err != nil {
			s.Fatal("Failed to create a temp directory: ", err)
		}
		defer os.RemoveAll(dir)

		filePath := filepath.Join(dir, testFile)
		if err := ioutil.WriteFile(filePath, []byte(testString), 0644); err != nil {
			s.Fatal("Failed to create file in Chrome OS: ", err)
		}
		defer os.Remove(filePath)
		if err := cont.PushFile(ctx, filePath, filepath.Join(sharedfolders.MountPathDownloads, testFile)); err != nil {
			s.Fatal("Failed to push test file to the container: ", err)
		}

		if err := filesApp.OpenDownloads(ctx); err != nil {
			s.Fatal("Failed to open Downloads: ", err)
		}
		// Check the newly created file is listed in Linux files.
		if err = filesApp.WaitForFile(ctx, testFile, 10*time.Second); err != nil {
			s.Fatal("Failed to find the test file in Files app: ", err)
		}
		if err = filesApp.WaitForFile(ctx, testFolder, 10*time.Second); err != nil {
			s.Fatal("Failed to find the test folder in Files app: ", err)
		}

		// Check the content of the test file in Chrome OS.
		b, err := ioutil.ReadFile(filepath.Join(filesapp.DownloadPath, testFile))
		if err != nil {
			s.Fatal("Failed to read the file in Chrome OS: ", err)
		}
		if string(b) != testString {
			s.Fatalf("Failed to verify the content of the file: got %s, want %s", string(b), testString)
		}
	})

}
