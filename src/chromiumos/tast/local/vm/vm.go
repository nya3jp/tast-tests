// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	cpb "chromiumos/system_api/vm_cicerone_proto"   // Protobufs for container management.
	vmpb "chromiumos/system_api/vm_concierge_proto" // Protobufs for VM management.
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"

	"github.com/golang/protobuf/proto"
)

const (
	liveContainerImageServer    = "https://storage.googleapis.com/cros-containers"         // simplestreams image server being served live
	stagingContainerImageServer = "https://storage.googleapis.com/cros-containers-staging" // simplestreams image server for staging

	testContainerName     = "penguin"        // default container name during testing (must be a valid hostname)
	testContainerUsername = "testuser"       // default container username during testing
	testImageAlias        = "debian/stretch" // default container alias
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

// NewContainer will create a Linux container in an existing VM.
// TODO(851207): Make a minimal Linux container for testing so this completes
// fast enough to use in bvt.
func (vm *VM) NewContainer(ctx context.Context, t ContainerType) (*Container, error) {
	c := &Container{
		Vm:            vm,
		containerName: testContainerName,
		username:      testContainerUsername,
	}

	obj, err := getCiceroneDBusObject()
	if err != nil {
		return nil, err
	}
	created, err := dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusutil.CiceronePath,
		Interface: dbusutil.CiceroneInterface,
		Member:    "LxdContainerCreated",
	})
	defer created.Close()

	var server string
	switch t {
	case LiveImageServer:
		server = liveContainerImageServer
	case StagingImageServer:
		server = stagingContainerImageServer
	}

	resp := &cpb.CreateLxdContainerResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.CiceroneInterface+".CreateLxdContainer",
		&cpb.CreateLxdContainerRequest{
			VmName:        testVMName,
			ContainerName: testContainerName,
			OwnerId:       c.Vm.Concierge.ownerId,
			ImageServer:   server,
			ImageAlias:    testImageAlias,
		}, resp); err != nil {
		return nil, err
	}

	switch resp.GetStatus() {
	case cpb.CreateLxdContainerResponse_UNKNOWN, cpb.CreateLxdContainerResponse_FAILED:
		return nil, fmt.Errorf("failed to create container: %v", resp.GetFailureReason())
	case cpb.CreateLxdContainerResponse_EXISTS:
		return nil, errors.New("container already exists")
	}

	// Container is being created, wait for signal.
	sigResult := &cpb.LxdContainerCreatedSignal{}
	testing.ContextLogf(ctx, "Waiting for LxdContainerCreated signal for container %q, VM %q", testContainerName, testVMName)
	select {
	case sig := <-created.Signals:
		if len(sig.Body) == 0 {
			return nil, errors.New("LxdContainerCreated signal lacked a body")
		}
		buf, ok := sig.Body[0].([]byte)
		if !ok {
			return nil, errors.New("LxdContainerCreated signal body is not a byte slice")
		}
		if err := proto.Unmarshal(buf, sigResult); err != nil {
			return nil, fmt.Errorf("failed unmarshaling LxdContainerCreated body: %v", err)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("didn't get LxdContainerCreated D-Bus signal: %v", ctx.Err())

	}

	if sigResult.GetVmName() != testVMName {
		return nil, fmt.Errorf("unexpected container creation signal for VM %q", sigResult.GetVmName())
	} else if sigResult.GetContainerName() != testContainerName {
		return nil, fmt.Errorf("unexpected container creation signal for container %q", sigResult.GetContainerName())
	}
	if sigResult.GetStatus() != cpb.LxdContainerCreatedSignal_CREATED {
		return nil, fmt.Errorf("failed to create container: status: %d reason: %v", sigResult.GetStatus(), sigResult.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Created container %q in VM %q", testContainerName, testVMName)
	return c, nil
}

func (vm *VM) Shutdown(ctx context.Context) error {
	obj, err := getConciergeDBusObject()
	if err != nil {
		return err
	}

	resp := &vmpb.StopVmResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.ConciergeInterface+".StopVm",
		&vmpb.StopVmRequest{
			Name:    vm.name,
			OwnerId: vm.Concierge.ownerId,
		}, resp); err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("failed to stop VM: %v", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Shut down VM %q", vm.name)
	return nil
}

// Command will return an testexec.Cmd with a vsh command that will run in this
// VM.
func (vm *VM) Command(ctx context.Context, arg ...string) *testexec.Cmd {
	args := append([]string{fmt.Sprintf("--vm_name=%s", vm.name),
		fmt.Sprintf("--owner_id=%s",
			vm.Concierge.ownerId),
		"--"},
		arg...)
	cmd := testexec.CommandContext(ctx, "vsh", args...)
	// Add a dummy buffer for stdin to force allocating a pipe. vsh uses
	// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
	cmd.Stdin = &bytes.Buffer{}
	return cmd
}
