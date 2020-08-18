// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Files,
		Desc:         "Check crostini FilesApp file watchers",
		Contacts:     []string{"joelhockey@chromium.org", "jkardatzke@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniStable,
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "artifact_unstable",
				Pre:               crostini.StartedByArtifact(),
				Timeout:           7 * time.Minute,
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:      "download_stretch",
				Pre:       crostini.StartedByDownloadStretch(),
				Timeout:   10 * time.Minute,
				ExtraAttr: []string{"informational"},
			},
			{
				Name:      "download_buster",
				Pre:       crostini.StartedByDownloadBuster(),
				Timeout:   10 * time.Minute,
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func Files(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	tconn := pre.TestAPIConn

	ownerID, err := cryptohome.UserHash(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}

	s.Log("Testing filesapp watch")
	testFilesAppWatch(ctx, s, tconn, ownerID)
}

func createFileInContainer(ctx context.Context, s *testing.State, ownerID, fileName, fileContent string) {
	cmd := vm.DefaultContainerCommand(ctx, ownerID, "sh", "-c", fmt.Sprintf("echo -n %s > %s", shutil.Escape(fileContent), fileName))
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatalf("Failed to write file %v in container: %v", fileName, err)
	}
}

func testFilesAppWatch(ctx context.Context, s *testing.State, tconn *chrome.TestConn, ownerID string) {
	const (
		testFileName1   = "file1.txt"
		testFileName2   = "file2.txt"
		testFileContent = "content"
	)

	createFileInContainer(ctx, s, ownerID, testFileName1, testFileContent)

	// Launch the files application
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Launching the Files App failed: ", err)
	}
	defer files.Root.Release(ctx)

	// Validate file1.txt is shown in 'Linux files'.
	if err := files.OpenDir(ctx, "Linux files", "Files - Linux files"); err != nil {
		s.Fatal("Opening Linux files folder failed: ", err)
	}
	if err := files.WaitForFile(ctx, testFileName1, 10*time.Second); err != nil {
		s.Fatal("Waiting for file1.txt failed: ", err)
	}

	// Create file2.txt in container and check that FilesApp refreshes.
	createFileInContainer(ctx, s, ownerID, testFileName2, testFileContent)
	if err := files.WaitForFile(ctx, testFileName2, 10*time.Second); err != nil {
		s.Fatal("Waiting for file2.txt failed: ", err)
	}
}
