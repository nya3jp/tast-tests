// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SSHFSMount,
		Desc:         "Checks crostini SSHFS mount",
		Contacts:     []string{"joelhockey@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Vars:         []string{"keepState"},
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

func SSHFSMount(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	cont := s.PreValue().(crostini.PreData).Container
	defer crostini.RunCrostiniPostTest(ctx, cont)

	ownerID, err := cryptohome.UserHash(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	sshfsMountDir := fmt.Sprintf("/media/fuse/crostini_%s_%s_%s", ownerID, vm.DefaultVMName, vm.DefaultContainerName)

	if stat, err := os.Stat(sshfsMountDir); err != nil {
		s.Fatalf("Didn't find sshfs mount %v: %v", sshfsMountDir, err)
	} else if !stat.IsDir() {
		s.Fatal("Didn't get directory for sshfs mount ", sshfsMountDir)
	}

	// Verify mount works for writing a file.
	const (
		testFileName    = "SshfsMount.txt"
		testFileContent = "SshfsMount"
	)
	crosFileName := filepath.Join(sshfsMountDir, testFileName)
	if err := ioutil.WriteFile(crosFileName, []byte(testFileContent), 0644); err != nil {
		s.Fatalf("Failed writing file %v: %v", crosFileName, err)
	}
	defer os.Remove(crosFileName)

	// Verify that test file is in the container.
	if err := cont.CheckFileContent(ctx, testFileName, testFileContent); err != nil {
		s.Fatalf("Wrong file content for %v: %v", testFileName, err)
	}
}
