// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/crostini/listset"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/settings"
	"chromiumos/tast/local/crostini/ui/sharedfolders"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShareDrive,
		Desc:     "Test sharing Google Drive with Crostini",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"crostini.gaiaUsername", "crostini.gaiaPassword", "crostini.gaiaID", "keepState"},
		Params: []testing.Param{{
			Name:              "artifact_gaia",
			Pre:               crostini.StartedByArtifactWithGaiaLogin(),
			ExtraData:         []string{crostini.ImageArtifact},
			Timeout:           7 * time.Minute,
			ExtraHardwareDeps: crostini.CrostiniStable,
		}, {
			Name:    "download_buster_gaia",
			Pre:     crostini.StartedByDownloadBusterWithGaiaLogin(),
			Timeout: 10 * time.Minute,
		}},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func ShareDrive(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	cr := s.PreValue().(crostini.PreData).Chrome

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	sharedFolders := sharedfolders.NewSharedFolders()
	// Clean up shared folders in the end.
	defer func() {
		if err := sharedFolders.UnshareAll(cleanupCtx, tconn, cont); err != nil {
			s.Error("Failed to unshare all folders: ", err)
		}
	}()

	s.Run(ctx, "test_share_drive", func(ctx context.Context, s *testing.State) {
		// Open Files app.
		filesApp, err := filesapp.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to open Files app: ", err)
		}
		// Close the Files app in the end.
		defer func() {
			if err := filesApp.Close(cleanupCtx); err != nil {
				s.Error("Failed to close the Files app: ", err)
			}
		}()

		if err := sharedFolders.ShareDriveOK(ctx, filesApp, tconn); err != nil {
			s.Fatal("Failed to share Google Drive: ", err)
		}

		if err := checkDriveResults(ctx, tconn, cont); err != nil {
			s.Fatal("Failed to verify sharing results: ", err)
		}
	})

	s.Run(ctx, "add_files_to_Drive", func(ctx context.Context, s *testing.State) {
		const (
			testFile   = "testD.sh"
			testString = "ls\n"
		)
		testFolder := fmt.Sprintf("testFolderD_%d", rand.Intn(1000000000))

		mountPath, err := drivefs.WaitForDriveFs(ctx, cr.User())
		if err != nil {
			s.Fatal("Failed waiting for DriveFS to start: ", err)
		}
		drivefsRoot := filepath.Join(mountPath, "root")
		folderPath := filepath.Join(drivefsRoot, testFolder)

		// Delete newly created files in the end.
		defer os.RemoveAll(folderPath)

		// Add a file and a folder in Drive.
		if err := os.MkdirAll(folderPath, 0755); err != nil {
			s.Fatal("Failed to create test folder in Drive: ", err)
		}
		if err := ioutil.WriteFile(filepath.Join(folderPath, testFile), []byte(testString), 0644); err != nil {
			s.Fatal("Failed to create file in Drive: ", err)
		}

		// Check file list in the container.
		filesInCont, err := cont.GetFileList(ctx, sharedfolders.MountPathMyDrive)
		if err != nil {
			s.Fatal("Failed to get file list of /mnt/chromeos/MyFiles/MyDrive: ", err)
		}
		list, err := ioutil.ReadDir(drivefsRoot)
		if err != nil {
			s.Fatal("Failed to read files in Drive: ", err)
		}
		var filesInDrive []string
		for _, f := range list {
			filesInDrive = append(filesInDrive, f.Name())
		}
		if err := listset.CheckListsMatch(filesInCont, filesInDrive...); err != nil {
			s.Fatal("Failed to verify the files list in container: ", err)
		}

		shared, err := cont.GetFileList(ctx, filepath.Join(sharedfolders.MountPathMyDrive, testFolder))
		if err != nil {
			s.Fatal("Failed to get file list of /mnt/chromeos/MyFiles/MyDrive: ", err)
		}
		if want := []string{testFile}; !reflect.DeepEqual(shared, want) {
			s.Fatalf("Failed to verify shared folders list, got %s, want %s", shared, want)
		}

		sharedFilePath := filepath.Join(sharedfolders.MountPathMyDrive, testFolder, testFile)

		// Check the content of the test file in the container.
		if err := cont.CheckFileContent(ctx, sharedFilePath, testString); err != nil {
			s.Fatal("Failed to verify the content of the test file: ", err)
		}

		// Check the file does not have execution permission.
		result, err := cont.Command(ctx, "ls", "-l", sharedFilePath).Output()
		if err != nil {
			s.Fatal("Failed to run ls on the test file in the container: ", err)
		}
		permission := strings.Split(string(result), " ")[0]
		if strings.Contains(permission, "x") {
			s.Fatalf("Failed to verify the permission of shared file, got %s, want %s", permission, "-rw-rw----")
		}
	})
}

func checkDriveResults(ctx context.Context, tconn *chrome.TestConn, cont *vm.Container) error {
	// Check the shared folders on Settings.
	s, err := settings.OpenLinuxSettings(ctx, tconn, settings.ManageSharedFolders)
	if err != nil {
		return errors.Wrap(err, "failed to find Manage shared folders")
	}
	defer s.Close(ctx)

	if shared, err := s.GetSharedFolders(ctx); err != nil {
		return errors.Wrap(err, "failed to find the shared folders list")
	} else if want := []string{sharedfolders.SharedDrive}; !reflect.DeepEqual(shared, want) {
		return errors.Errorf("failed to verify shared folders list, got %s, want %s", shared, want)
	}

	// Check the file list in the container.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if list, err := cont.GetFileList(ctx, sharedfolders.MountPath); err != nil {
			return err
		} else if want := []string{"fonts", sharedfolders.MountFolderGoogleDrive}; !reflect.DeepEqual(list, want) {
			return errors.Errorf("failed to verify file list in /mnt/chromeos, got %s, want %s", list, want)
		}

		if list, err := cont.GetFileList(ctx, sharedfolders.MountPathGoogleDrive); err != nil {
			return err
		} else if want := []string{sharedfolders.MountFolderMyDrive}; !reflect.DeepEqual(list, want) {
			return errors.Errorf("failed to verify file list in /mnt/chromeos/GoogleDrive, got %s, want %s", list, want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify file list in container")
	}

	return nil
}
