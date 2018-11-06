// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memoryuser contains common code to run multifaceted memory tests
// with Chrome, ARC, and VMs
package memoryuser

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// VMCmd contains a list of vshArgs to use in the container command
type VMCmd []string

// VMTask contains a list of VMCmds
type VMTask struct {
	VMCommands []VMCmd
}

// RunMemoryTask starts a VM and then executes the list of VMCommands defined in VMTask
func (vmTask VMTask) RunMemoryTask(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Restarting Concierge")
	conc, err := vm.NewConcierge(ctx, cr.User())
	if err != nil {
		s.Fatal("Failed to start concierge: ", err)
	}

	testvm := vm.NewDefaultVM(conc)
	err = testvm.Start(ctx)
	if err != nil {
		s.Fatal("Failed to start VM: ", err)
	}
	defer testvm.Stop(ctx)

	s.Log("Running vm commands")
	for i := 0; i < len(vmTask.VMCommands); i++ {
		cmd := testvm.Command(ctx, vmTask.VMCommands[i]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			cmd.DumpLog(ctx)
			output := string(out[:])
			s.Log(output)
			s.Fatal("Failed to run vm command: ", err)
		}
	}

}
