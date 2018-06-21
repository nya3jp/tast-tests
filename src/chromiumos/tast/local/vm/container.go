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

// Container encapsulates a container running in a VM.
type Container struct {
	vm            *VM                     // VM in which this container is running.
	containerName string                  // name of the container
	username      string                  // username of the container's primary user
	started       *dbusutil.SignalWatcher // SignalWatcher for ContainerStarted.
}

// Start will start a Linux container in an existing VM.
func (c *Container) Start(ctx context.Context) error {
	if c.started == nil {
		var err error
		if c.started, err = dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
			Type:      "signal",
			Path:      dbusutil.CiceronePath,
			Interface: dbusutil.CiceroneInterface,
			Member:    "ContainerStarted",
		}); err != nil {
			return err
		}
	}

	obj, err := getCiceroneDBusObject()
	if err != nil {
		return err
	}
	resp := &cpb.StartLxdContainerResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.CiceroneInterface+".StartLxdContainer",
		&cpb.StartLxdContainerRequest{
			VmName:        c.vm.name,
			ContainerName: c.containerName,
			OwnerId:       c.vm.ownerId,
		}, resp); err != nil {
		return err
	}

	switch resp.GetStatus() {
	case cpb.StartLxdContainerResponse_RUNNING:
		return errors.New("container is already running")
	case cpb.StartLxdContainerResponse_STARTED:
	default:
		return fmt.Errorf("failed to start container: %v", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Started container %q in VM %q", c.containerName, c.vm.name)
	return nil
}

// GetUsername will get the default user in a container.
func (c *Container) GetUsername(ctx context.Context) (string, error) {
	obj, err := getCiceroneDBusObject()
	if err != nil {
		return "", err
	}

	resp := &cpb.GetLxdContainerUsernameResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.CiceroneInterface+".GetLxdContainerUsername",
		&cpb.GetLxdContainerUsernameRequest{
			VmName:        c.vm.name,
			ContainerName: c.containerName,
			OwnerId:       c.vm.ownerId,
		}, resp); err != nil {
		return "", err
	}

	switch resp.GetStatus() {
	case cpb.GetLxdContainerUsernameResponse_SUCCESS:
	default:
		return "", fmt.Errorf("failed to get username: %v", resp.GetFailureReason())
	}

	return resp.GetUsername(), nil
}

// SetUpUser will set up the default user in a container.
func (c *Container) SetUpUser(ctx context.Context) error {
	obj, err := getCiceroneDBusObject()
	if err != nil {
		return err
	}

	resp := &cpb.SetUpLxdContainerUserResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.CiceroneInterface+".SetUpLxdContainerUser",
		&cpb.SetUpLxdContainerUserRequest{
			VmName:            c.vm.name,
			ContainerName:     c.containerName,
			OwnerId:           c.vm.ownerId,
			ContainerUsername: c.username,
		}, resp); err != nil {
		return err
	}

	switch resp.GetStatus() {
	case cpb.SetUpLxdContainerUserResponse_SUCCESS, cpb.SetUpLxdContainerUserResponse_EXISTS:
	default:
		return fmt.Errorf("failed to set up user: %v", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Set up user %q in container %q", c.username, c.containerName)
	return nil
}

// Command will return an exec.Command with a vsh command that will run in this
// container.
func (c *Container) Command(ctx context.Context, arg ...string) *testexec.Cmd {
	args := append([]string{fmt.Sprintf("--vm_name=%s", c.vm.name), fmt.Sprintf("--target_container=%s", c.containerName), fmt.Sprintf("--owner_id=%s", c.vm.ownerId), "--"}, arg...)
	cmd := testexec.CommandContext(ctx, "vsh", args...)
	// Add a dummy buffer for stdin to force allocating a pipe. vsh uses
	// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
	cmd.Stdin = &bytes.Buffer{}
	return cmd
}

// WaitForGarcon will block until garcon starts up in this container.
func (c *Container) WaitForGarcon(ctx context.Context) error {
	// Always close the ContainerStarted watcher regardless of success.
	defer c.started.Close()

	sigResult := &cpb.ContainerStartedSignal{}
	for sigResult.VmName != c.vm.name &&
		sigResult.ContainerName != c.containerName &&
		sigResult.OwnerId != c.vm.ownerId {
		select {
		case sig := <-c.started.Signals:
			if len(sig.Body) == 0 {
				return errors.New("ContainerStarted signal lacked a body")
			}
			buf, ok := sig.Body[0].([]byte)
			if !ok {
				return errors.New("ContainerStarted signal body is not a byte slice")
			}
			if err := proto.Unmarshal(buf, sigResult); err != nil {
				return fmt.Errorf("failed unmarshaling ContainerStarted body: %v", err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}
