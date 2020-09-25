// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/vmc"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const testScript string = "test-extra-disk.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         VmcExtraDisk,
		Desc:         "Starts Crostini with an extra disk image",
		Contacts:     []string{"keiichiw@chromium.org", "cros-containers-dev@google.com"},
		SoftwareDeps: []string{"chrome", "vm_host", "untrusted_vm"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{testScript},
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

func VmcExtraDisk(ctx context.Context, s *testing.State) {
	defer crostini.RunCrostiniPostTest(ctx, s.PreValue().(crostini.PreData))

	hash, err := vmc.UserIDHash(ctx)
	if err != nil {
		s.Fatal("Failed to get CROS_USER_ID_HASH: ", err)
	}

	// Create a file in a temp directory in Chrome OS and push it to the container.
	dir, err := ioutil.TempDir("", "tast.crostini.VmcExtraDisk")
	if err != nil {
		s.Fatal("Failed to create a temp directory: ", err)
	}
	defer os.RemoveAll(dir)

	extraDisk := filepath.Join(dir, "extra.img")

	// Run `vmc extra-disk-create`
	if err := vmc.Command(ctx, hash, "create-extra-disk", "--size", "256M", extraDisk).
		Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to create an extra disk image: ", err)
	}

	const vmName = "tast_vmc_extra_disk_vm"

	// Run `vmc create $vmName`
	if err := vmc.Command(ctx, hash, "create", vmName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to create %s VM: %v", vmName, err)
	}
	defer func() {
		if err := vmc.Command(ctx, hash, "destroy", vmName).Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Failed to destroy %s VM: %v", vmName, err)
		}
	}()

	// Run `vmc start $vmName --extra-disk $extraDisk`.
	cmd := vmc.Command(ctx, hash, "start", vmName, "--extra-disk", extraDisk)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.Fatal("Failed to open stdin pipe: ", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Fatal("Failed to open stdout pipe: ", err)
	}
	if err := cmd.Start(); err != nil {
		s.Fatalf("Failed to start %s VM with %s: %v", vmName, extraDisk, err)
	}

	// Write the content of testScript to the VM's stdin.
	f, err := os.Open(s.DataPath(testScript))
	if err != nil {
		s.Fatalf("Failed to open %s: %v", testScript, err)
	}
	defer f.Close()
	if _, err := io.Copy(stdin, f); err != nil {
		s.Fatal("Failed to write a test script to stdin pipe: ", err)
	}
	// Close stdin explicitly to get outputs.
	if err := stdin.Close(); err != nil {
		s.Fatal("Failed to close stdin pipe: ", err)
	}

	buf := new(strings.Builder)
	if _, err := io.Copy(buf, stdout); err != nil {
		s.Fatal("Failed to get the VM output: ", err)
	}
	output := string(buf.String())

	if err := cmd.Wait(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to wait for exiting from the VM shell: ", err)
	}

	// Check if the test script ran successfully.
	// Use strings.Contains instead of "==", as output contains ANSI escape sequences.
	if !strings.Contains(output, "TAST_OK") {
		s.Error("Test script didn't run successfully: ", output)
	}

	// Run `vmc stop $vmName`
	if err := vmc.Command(ctx, hash, "stop", vmName).Run(testexec.DumpLogOnError); err != nil {
		s.Errorf("Failed to stop %s VM: %v", vmName, err)
	}
}
