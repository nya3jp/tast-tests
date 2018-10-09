// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"

	vmpb "chromiumos/system_api/vm_concierge_proto" // protobufs for VM management
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	liveContainerImageServerFormat    = "https://storage.googleapis.com/cros-containers/%d"         // simplestreams image server being served live
	stagingContainerImageServerFormat = "https://storage.googleapis.com/cros-containers-staging/%d" // simplestreams image server for staging

	testVMName            = "termina"             // default VM name during testing (must be a valid hostname)
	testContainerName     = "penguin"             // default container name during testing (must be a valid hostname)
	testContainerUsername = "testuser"            // default container username during testing
	testImageAlias        = "debian/stretch/test" // default container alias
)

type ContainerType int

const (
	// LiveImageServer indicates that the current live container image should be downloaded.
	LiveImageServer ContainerType = iota
	// StagingImageServer indicates that the current staging container image should be downloaded.
	StagingImageServer
)

// VM encapsulates a virtual machine managed by the concierge/cicerone daemons.
type VM struct {
	// Concierge is the Concierge instance managing this VM.
	Concierge *Concierge
	name      string // name of the VM
}

// GetDefaultVM gets a default VM instance.
func GetDefaultVM(c *Concierge) *VM {
	return &VM{
		Concierge: c,
		name:      testVMName,
	}
}

// Start launches the VM.
func (vm *VM) Start(ctx context.Context) error {
	return vm.Concierge.StartTerminaVM(ctx, vm)
}

// Stop shuts down VM. It can be restart again later.
func (vm *VM) Stop(ctx context.Context) error {
	resp := &vmpb.StopVmResponse{}
	if err := dbusutil.CallProtoMethod(ctx, vm.Concierge.conciergeObj, conciergeInterface+".StopVm",
		&vmpb.StopVmRequest{
			Name:    vm.name,
			OwnerId: vm.Concierge.ownerID,
		}, resp); err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return errors.Errorf("failed to stop VM: %v", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Shut down VM %q", vm.name)
	return nil
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
