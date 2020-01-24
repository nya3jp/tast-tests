// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// VMCmd contains a list of vshArgs to use in the container command.
type VMCmd []string

// VMTask implements MemoryTask to run commands in a VM.
type VMTask struct {
	// VMCommands is a list of VMCmds.
	Cmds  []VMCmd
	Files []string
}

// Run executes the list of VMCommands defined in VMTask in the existing VM from the TestEnvironment.
func (vt *VMTask) Run(ctx context.Context, s *testing.State, testEnv *TestEnv) error {
	ownerID, err := cryptohome.UserHash(ctx, testEnv.cr.User())
	if err != nil {
		return errors.Wrap(err, "failed to get user hash")
	}
	vmMountDir := fmt.Sprintf("/media/fuse/crostini_%s_%s_%s", ownerID, vm.DefaultVMName, vm.DefaultContainerName)
	for _, f := range vt.Files {
		name := filepath.Base(f)
		input, err := ioutil.ReadFile(f)
		if err != nil {
			return errors.Wrap(err, "cannot read data file")
		}
		if err = ioutil.WriteFile(filepath.Join(vmMountDir, name), input, 0644); err != nil {
			return errors.Wrap(err, "cannot copy data file into VM directory")
		}
	}
	testing.ContextLog(ctx, "Running VM commands")
	for i := 0; i < len(vt.Cmds); i++ {
		cmd := vm.DefaultContainerCommand(ctx, ownerID, vt.Cmds[i]...)
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			return errors.Wrap(err, "failed to run vm command")
		}
	}
	return nil
}

// Close does nothing since VMTask does not initialize anything in Run.
func (vt *VMTask) Close(ctx context.Context, testEnv *TestEnv) {}

// String returns a string describing the VMTask.
func (vt *VMTask) String() string {
	return fmt.Sprintf("VMTask with commands: %s", vt.Cmds)
}

// NeedVM returns true to indicate that a VM is required for a VMTask.
func (vt *VMTask) NeedVM() bool {
	return true
}
