// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	cpb "chromiumos/system_api/vm_cicerone_proto" // Protobufs for container management.
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"

	"github.com/golang/protobuf/proto"
)

const (
	testContainerName     = "penguin"                                        // The default container name during testing (must be a valid hostname).
	testImageAlias        = "debian/stretch"                                 // The default container name during testing (must be a valid hostname).
	testImageServer       = "https://storage.googleapis.com/cros-containers" // The default container name during testing (must be a valid hostname).
	testContainerUsername = "testuser"                                       // The default container username during testing
)

// VM encapsulates a virtual machine managed by the concierge/cicerone daemons.
type VM struct {
	ownerId string // cryptohome hash for the logged-in user
	name    string // name of the VM
}

// CreateContainer will start a Linux container in an existing VM.
// TODO(851207): Make a minimal Linux container for testing so this completes
// fast enough to use in bvt.
func (vm *VM) CreateContainer(ctx context.Context) (*Container, error) {
	c := &Container{
		vm:            vm,
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

	resp := &cpb.CreateLxdContainerResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.CiceroneInterface+".CreateLxdContainer",
		&cpb.CreateLxdContainerRequest{
			VmName:        testVMName,
			ContainerName: testContainerName,
			OwnerId:       c.vm.ownerId,
			ImageServer:   "https://storage.googleapis.com/cros-containers",
			ImageAlias:    "debian/stretch",
		}, resp); err != nil {
		return nil, err
	}

	switch resp.GetStatus() {
	case cpb.CreateLxdContainerResponse_UNKNOWN, cpb.CreateLxdContainerResponse_FAILED:
		return nil, fmt.Errorf("failed to create container: %v", resp.GetFailureReason())
	case cpb.CreateLxdContainerResponse_EXISTS:
		return nil, errors.New("container already exists")
	case cpb.CreateLxdContainerResponse_CREATING:
		testing.ContextLogf(ctx, "Waiting for LxdContainerCreated signal for container %q, VM %q", testContainerName, testVMName)
	}

	// Container is being created, wait for signal.
	sigResult := &cpb.LxdContainerCreatedSignal{}
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
		return nil, ctx.Err()
	}

	if sigResult.GetVmName() != testVMName {
		return nil, fmt.Errorf("unexpected container creation signal for VM %q", sigResult.GetVmName())
	} else if sigResult.GetContainerName() != testContainerName {
		return nil, fmt.Errorf("unexpected container creation signal for container %q", sigResult.GetContainerName())
	}
	switch sigResult.GetStatus() {
	case cpb.LxdContainerCreatedSignal_CREATED:
	default:
		return nil, fmt.Errorf("failed to create container: status: %d reason: %v", sigResult.GetStatus(), sigResult.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Created container %q in VM %q", testContainerName, testVMName)
	return c, nil
}

// Command will return an exec.Command with a vsh command that will run in this
// VM.
func (vm *VM) Command(ctx context.Context, arg ...string) *testexec.Cmd {
	args := append([]string{fmt.Sprintf("--vm_name=%s", vm.name), fmt.Sprintf("--owner_id=%s", vm.ownerId), "--"}, arg...)
	cmd := testexec.CommandContext(ctx, "vsh", args...)
	// Add a dummy buffer for stdin to force allocating a pipe. vsh uses
	// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
	cmd.Stdin = &bytes.Buffer{}
	return cmd
}
