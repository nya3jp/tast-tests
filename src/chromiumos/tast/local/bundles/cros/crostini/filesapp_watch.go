// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesappWatch,
		Desc:         "Checks crostini Filesapp watch",
		Contacts:     []string{"joelhockey@chromium.org", "cros-containers-dev@google.com"},
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

func FilesappWatch(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	tconn := pre.TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container

	const (
		testFileName1   = "file1.txt"
		testFileName2   = "file2.txt"
		testFileContent = "content"
	)

	if err := crostini.CreateFileInContainer(ctx, cont, testFileName1, testFileContent); err != nil {
		s.Fatal("Create file failed: ", err)
	}

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
	if err := crostini.CreateFileInContainer(ctx, cont, testFileName2, testFileContent); err != nil {
		s.Fatal("Create file failed: ", err)
	}
	if err := files.WaitForFile(ctx, testFileName2, 10*time.Second); err != nil {
		s.Fatal("Waiting for file2.txt failed: ", err)
	}
}
