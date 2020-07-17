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
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     NoAccessToDownloads,
		Desc:     "Ran a test to make sure linux does not have access to downloads on chrome using a pre-built crostini image",
		Contacts: []string{"jinrong@google.com", "cros-containers-dev@google.com"},
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

func NoAccessToDownloads(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container

	// Use a shortened context for test operations to reserve time for cleanup.
	shortCtx, shortCancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer shortCancel()

	s.Log("Test home directory in container is empty by default")
	if err := checkHomeDirInContainerEmpty(shortCtx, cont); err != nil {
		s.Fatal("Home directory in container is not empty by default: ", err)
	}
	s.Log("Test MyFiles are not in the container by default")
	myFiles := "MyFiles"
	mntPath := "/mnt/chromeos"
	if err := cont.CheckFileDoesNotExistInDir(shortCtx, mntPath, myFiles); err != nil {
		s.Fatal("MyFiles is unexpectedly listed in /mnt/chromeos in container by default: ", err)
	}

	// Create a file in Downloads.
	filename := "test.txt"
	if err := ioutil.WriteFile(filepath.Join(filesapp.DownloadPath, filename), []byte("teststring"), 0644); err != nil {
		s.Fatal("Failed to create a file in Downloads: ", err)
	}
	defer os.Remove(filepath.Join(filesapp.DownloadPath, filename))

	s.Log("Test home directory in container is empty after creating a file in Downloads in Chrome")
	if err := checkHomeDirInContainerEmpty(shortCtx, cont); err != nil {
		s.Fatal("Home directory in container is not empty after creating a file in Downloads in Chrome: ", err)
	}
	s.Log("Test MyFiles are not in the container after creating a file in Downloads in Chrome")
	if err := cont.CheckFileDoesNotExistInDir(shortCtx, mntPath, myFiles); err != nil {
		s.Fatal("MyFiles is unexpectedly listed in /mnt/chromeos in container: ", err)
	}
}

func checkHomeDirInContainerEmpty(shortCtx context.Context, cont *vm.Container) error {
	// Get file list in home directory in container
	fileList, err := cont.GetFileList(shortCtx, ".")
	if err != nil {
		return errors.Wrap(err, "failed to list the content of home directory in container")
	}
	if fileList != "" {
		return errors.Errorf("Home directory unexpectedly contains some files %s", fileList)
	}
	return nil
}
