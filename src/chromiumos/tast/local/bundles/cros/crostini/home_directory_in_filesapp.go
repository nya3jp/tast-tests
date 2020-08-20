// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/linuxfiles"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const filename = "test.txt"

func init() {
	testing.AddTest(&testing.Test{
		Func:     HomeDirectoryInFilesapp,
		Desc:     "Runs a basic test on the default share folder (through UI) using a pre-built crostini image",
		Contacts: []string{"jinrongwu@chromium.org"},
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

func HomeDirectoryInFilesapp(ctx context.Context, s *testing.State) {
	tconn := s.PreValue().(crostini.PreData).TestAPIConn
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, cont)

	// Clean up the home directory in the end.
	defer func() {
		if err := cont.Cleanup(ctx, "."); err != nil {
			s.Error("Failed to remove all files in home directory in the container: ", err)
		}
	}()

	// Open Files app.
	fa, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to open Files app: ", err)
	}
	defer fa.Root.Release(ctx)
	// Check whether "Linux files" is listed through opening it.
	if err = fa.OpenDir(ctx, linuxfiles.DirName, linuxfiles.Title); err != nil {
		s.Fatal("Failed to open Linux files: ", err)
	}

	// Create a file inside container.
	if err := cont.Command(ctx, "touch", filename).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to create file in the container: ", err)
	}
	// Open "Linux files" to refresh.
	if err = fa.OpenDir(ctx, linuxfiles.DirName, linuxfiles.Title); err != nil {
		s.Fatal("Failed to open Linux files after creating files inside container: ", err)
	}

	// Click Refresh.
	if err := fa.LeftClickItem(ctx, "Refresh", ui.RoleTypeButton); err != nil {
		s.Fatal("Failed to click button Refresh on Files app: ", err)
	}
	// Check the newly created file is listed in Linux files.
	if err = fa.WaitForFile(ctx, filename, 10*time.Second); err != nil {
		s.Error("Failed to list the file created from crostini in Files app: ", err)
	}
}
