// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memoryuser contains common code to run multifaceted memory tests
// with Chrome, ARC, and VMs
package memoryuser

import (
	"context"

	"chromiumos/tast/testing"
)

// VMCmd contains a list of vshArgs to use in the container command
type VMCmd []string

// VMTask contains a list of VMCmds
type VMTask struct {
	VMCommands []VMCmd
}

// RunMemoryTask executes the list of VMCommands defined in VMTask in the existing VM from the TestEnvironment
func (vmTask VMTask) RunMemoryTask(ctx context.Context, s *testing.State, testEnv *TestEnvironment) {
	s.Log("Running vm commands")
	for i := 0; i < len(vmTask.VMCommands); i++ {
		cmd := testEnv.VM.Command(ctx, vmTask.VMCommands[i]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			cmd.DumpLog(ctx)
			output := string(out[:])
			s.Log(output)
			s.Fatal("Failed to run vm command: ", err)
		}
	}

}
