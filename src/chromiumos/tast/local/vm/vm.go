// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"

	"chromiumos/tast/local/testexec"
)

const (
	testVMName = "termina" // default VM name during testing (must be a valid hostname)
)

// VM encapsulates a virtual machine managed by the concierge/cicerone daemons.
type VM struct {
	// Concierge is the Concierge instance managing this VM.
	Concierge *Concierge
	name      string // name of the VM
	Cid       int64  // cid for the crosvm process
}

// NewDefaultVM gets a default VM instance.
func NewDefaultVM(c *Concierge) *VM {
	return &VM{
		Concierge: c,
		name:      testVMName,
		Cid:       -1, // not populated until VM is started.
	}
}

// Start launches the VM.
func (vm *VM) Start(ctx context.Context) error {
	return vm.Concierge.startTerminaVM(ctx, vm)
}

// Stop shuts down VM. It can be restarted again later.
func (vm *VM) Stop(ctx context.Context) error {
	return vm.Concierge.stopVM(ctx, vm)
}

// Command will return an testexec.Cmd with a vsh command that will run in this
// VM.
func (vm *VM) Command(ctx context.Context, vshArgs ...string) *testexec.Cmd {
	args := append([]string{"--vm_name=" + vm.name,
		"--owner_id=" + vm.Concierge.ownerID,
		"--"},
		vshArgs...)
	cmd := testexec.CommandContext(ctx, "vsh", args...)
	// Add a dummy buffer for stdin to force allocating a pipe. vsh uses
	// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
	cmd.Stdin = &bytes.Buffer{}
	return cmd
}
