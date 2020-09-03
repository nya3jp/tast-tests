// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"

	"chromiumos/tast/local/bundles/cros/crostini/vmc"
	"chromiumos/tast/local/chrome"
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

func VmcStart(ctx context.Context, s *testing.State) {
	hash, err := vmc.UserIDHash(ctx)
	if err != nil {
		s.Fatal("Failed to get CROS_USER_ID_HASH: ", err)
	}

	const vmName = "tast_vmc_start_vm"

	// Run `vmc create $vmName`
	if err := vmc.Command(ctx, hash, "create", vmName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Faild to create a VM: ", err)
	}
	defer vmc.Command(ctx, hash, "destroy", vmName).Run(testexec.DumpLogOnError)

	// Run `echo exit | vmc start $vmName`
	cmd := vmc.Command(ctx, hash, "start", vmName)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.Fatal("Failed to open stdin pipe: ", err)
	}
	go func() {
		defer stdin.Close()
		if _, err := stdin.Write([]byte("exit")); err != nil {
			s.Error("Failed to write 'exit' to stdin pipe: ", err)
		}
	}()
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Faild to start a VM: ", err)
	}

	// Run `vmc stop $vmName`
	if err := vmc.Command(ctx, hash, "stop", vmName).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Faild to stop a VM: ", err)
	}
}
