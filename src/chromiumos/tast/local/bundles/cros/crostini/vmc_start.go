// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/vmc"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VmcStart,
		Desc:         "Starts Crostini via vmc commands",
		Contacts:     []string{"keiichiw@chromium.org", "cros-containers-dev@google.com"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"keepState"},
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

func VmcStart(ctx context.Context, s *testing.State) {
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	hash, err := vmc.UserIDHash(ctx)
	if err != nil {
		s.Fatal("Failed to get CROS_USER_ID_HASH: ", err)
	}

	const vmName = "tast_vmc_start_vm"

	// Run `vmc create $vmName`
	if err := vmc.Command(ctx, hash, "create", vmName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to create %s VM: %v", vmName, err)
	}
	defer func() {
		if err := vmc.Command(ctx, hash, "destroy", vmName).Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to destroy %s VM: %v", vmName, err)
		}
	}()

	// Run `echo exit | vmc start $vmName`
	// First, start the vmc command.
	cmd := vmc.Command(ctx, hash, "start", vmName)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.Fatal("Failed to open stdin pipe: ", err)
	}
	if err := cmd.Start(); err != nil {
		s.Fatalf("Failed to start %s VM: %v", vmName, err)
	}
	// Then, send "exit" to the VM shell.
	if _, err := stdin.Write([]byte("exit\n")); err != nil {
		s.Fatal("Failed to write 'exit' to stdin pipe: ", err)
	}
	if err := cmd.Wait(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to wait for exiting from the VM shell: ", err)
	}

	// Run `vmc stop $vmName`
	if err := vmc.Command(ctx, hash, "stop", vmName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to stop %s VM: %v", vmName, err)
	}
}
