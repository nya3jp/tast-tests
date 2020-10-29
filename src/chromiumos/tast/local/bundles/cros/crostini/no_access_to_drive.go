// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/sharedfolders"
	"chromiumos/tast/local/drivefs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     NoAccessToDrive,
		Desc:     "Run a test to make sure crostini does not have access to GoogleDrive",
		Contacts: []string{"jinrong@google.com", "cros-containers-dev@google.com"},
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

func NoAccessToDrive(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	cr := s.PreValue().(crostini.PreData).Chrome

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer crostini.RunCrostiniPostTest(cleanupCtx, s.PreValue().(crostini.PreData))

	if err := cont.CheckFileDoesNotExistInDir(ctx, sharedfolders.MountPath, sharedfolders.MountFolderGoogleDrive); err != nil {
		s.Fatalf("GoogleDrive is unexpectedly listed in %s in the container: %s", sharedfolders.MountPath, err)
	}

	// Generate a random folder name to avoid duplicate across devices.
	newFolder := fmt.Sprintf("NoAccessToDrive_%d", rand.Intn(1000000000))
	s.Log("The new folder name is ", newFolder)

	// Create a new folder in Drive.
	mountPath, err := drivefs.WaitForDriveFs(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed waiting for DriveFS to start: ", err)
	}
	folderPath := filepath.Join(mountPath, "root", newFolder)

	// Add a file and a folder in Drive.
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		s.Fatal("Failed to create test folder in Drive: ", err)
	}
	defer os.RemoveAll(folderPath)

	fileList, err := cont.GetFileList(ctx, ".")
	if err != nil {
		s.Fatal("Failed to list the content of home directory in container: ", err)
	}
	if len(fileList) != 0 {
		s.Fatalf("Failed to verify file list in home directory in the container: got %q, want []", fileList)
	}

	if err := cont.CheckFileDoesNotExistInDir(ctx, sharedfolders.MountPath, sharedfolders.MountFolderGoogleDrive); err != nil {
		s.Fatalf("GoogleDrive is unexpectedly listed in %s in the container: %s", sharedfolders.MountPath, err)
	}
}
