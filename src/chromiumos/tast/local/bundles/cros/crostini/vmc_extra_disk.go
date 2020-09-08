// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/bundles/cros/crostini/vmc"
	"chromiumos/tast/local/chrome"
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
		Params: []testing.Param{
			{
				Name:              "artifact",
				Pre:               chrome.LoggedIn(),
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniStable,
			},
			{
				Name:              "artifact_unstable",
				Pre:               chrome.LoggedIn(),
				ExtraData:         []string{crostini.ImageArtifact},
				ExtraHardwareDeps: crostini.CrostiniUnstable,
			},
		},
	})
}

func VmcExtraDisk(ctx context.Context, s *testing.State) {
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
	if err := vmc.Command(ctx, hash, "create-extra-disk", "--size", "256000000", extraDisk).
		Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Faild to create an extra disk image: ", err)
	}

	const vmName = "tast_vmc_extra_disk_vm"

	// Run `vmc create $vmName`
	if err := vmc.Command(ctx, hash, "create", vmName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Faild to create a VM: ", err)
	}
	defer vmc.Command(ctx, hash, "destroy", vmName).Run(testexec.DumpLogOnError)

	// Runs commands which equals to `cat $testScript | vmc start $vmName --extra-disk $extraDisk`
	cmd := vmc.Command(ctx, hash, "start", vmName, "--extra-disk", extraDisk)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.Fatal("Failed to open stdin pipe: ", err)
	}
	go func() {
		defer stdin.Close()
		script, err := ioutil.ReadFile(s.DataPath(testScript))
		if err != nil {
			s.Fatalf("Failed to read %s: %v", testScript, err)
		}
		if _, err := stdin.Write([]byte(script)); err != nil {
			s.Error("Failed to write a test script to stdin pipe: ", err)
		}
	}()

	out, err := cmd.CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Faild to start VM: ", err)
	}
	output := string(out)
	// Check if the test script ran successfully.
	// Use HasSuffix instead of "==", as output contains ANSI escape sequences.
	if !strings.HasSuffix(output, "OK\n") {
		s.Fatal("Test script didn't ran successfully: ", output)
	}

	// Run `vmc stop $vmName`
	if err := vmc.Command(ctx, hash, "stop", vmName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Faild to stop a VM: ", err)
	}
}
