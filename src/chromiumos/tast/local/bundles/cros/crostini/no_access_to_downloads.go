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
		Func:         NoAccessToDownloads,
		Desc:         "Run a test to make sure Linux does not have access to downloads on Chrome using a pre-built crostini image",
		Contacts:     []string{"jinrong@google.com", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Params:       crostini.MakeTestParams(crostini.TestInformational),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func NoAccessToDownloads(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, cont)

	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Test home directory in container is empty by default")
	if err := checkHomeDirInContainerEmpty(ctx, cont); err != nil {
		s.Fatal("Home directory in container is not empty by default: ", err)
	}
	s.Log("Test MyFiles are not in the container by default")
	const (
		myFiles = "MyFiles"
		mntPath = "/mnt/chromeos"
	)
	if err := cont.CheckFileDoesNotExistInDir(ctx, mntPath, myFiles); err != nil {
		s.Fatal("MyFiles is unexpectedly listed in /mnt/chromeos in container by default: ", err)
	}

	// Create a file in Downloads.
	const filename = "test.txt"
	if err := ioutil.WriteFile(filepath.Join(filesapp.DownloadPath, filename), []byte("teststring"), 0644); err != nil {
		s.Fatal("Failed to create a file in Downloads: ", err)
	}
	defer os.Remove(filepath.Join(filesapp.DownloadPath, filename))

	s.Log("Test home directory in container is empty after creating a file in Downloads in Chrome")
	if err := checkHomeDirInContainerEmpty(ctx, cont); err != nil {
		s.Fatal("Home directory in container is not empty after creating a file in Downloads in Chrome: ", err)
	}
	s.Log("Test MyFiles are not in the container after creating a file in Downloads in Chrome")
	if err := cont.CheckFileDoesNotExistInDir(ctx, mntPath, myFiles); err != nil {
		s.Fatal("MyFiles is unexpectedly listed in /mnt/chromeos in container: ", err)
	}
}

func checkHomeDirInContainerEmpty(ctx context.Context, cont *vm.Container) error {
	// Get file list in home directory in container.
	fileList, err := cont.GetFileList(ctx, ".")
	if err != nil {
		return errors.Wrap(err, "failed to list the content of home directory in container")
	}
	if len(fileList) != 0 {
		return errors.Errorf("home directory unexpectedly contains some files %s", fileList)
	}
	return nil
}
