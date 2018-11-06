// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// VMCmd contains a list of vshArgs to use in the container command
type VMCmd []string

// VMTask implements MemoryTask to run commands in a VM
type VMTask struct {
	// VMCommands is a list of VMCmds
	VMCommands []VMCmd
}

// Run executes the list of VMCommands defined in VMTask in the existing VM from the TestEnvironment
func (vt *VMTask) Run(ctx context.Context, testEnv *TestEnv) error {
	testing.ContextLog(ctx, "Running vm commands")
	for i := 0; i < len(vt.VMCommands); i++ {
		cmd := testEnv.VM.Command(ctx, vt.VMCommands[i]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			cmd.DumpLog(ctx)
			output := string(out[:])
			testing.ContextLog(ctx, output)
			return errors.Wrap(err, "failed to run vm command")
		}
	}
	return nil
}

// Close does nothing since VMTask does not initialize anything in Run
func (vt *VMTask) Close(ctx context.Context, testEnv *TestEnv) {
}
