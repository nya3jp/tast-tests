// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	cpb "chromiumos/system_api/vm_cicerone_proto" // protobufs for container management
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"

	"github.com/golang/protobuf/proto"
)

// Container encapsulates a container running in a VM.
type Container struct {
	// VM is the VM in which this container is running.
	VM            *VM
	containerName string // name of the container
	username      string // username of the container's primary user
}

// Start starts a Linux container in an existing VM.
func (c *Container) Start(ctx context.Context) error {
	obj, err := getCiceroneDBusObject()
	if err != nil {
		return err
	}
	resp := &cpb.StartLxdContainerResponse{}
	if err = dbusutil.CallProtoMethod(ctx, obj, dbusutil.CiceroneInterface+".StartLxdContainer",
		&cpb.StartLxdContainerRequest{
			VmName:        c.VM.name,
			ContainerName: c.containerName,
			OwnerId:       c.VM.Concierge.ownerID,
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

	testing.ContextLogf(ctx, "Started container %q in VM %q", c.containerName, c.VM.name)
	return nil
}

// GetUsername returns the default user in a container.
func (c *Container) GetUsername(ctx context.Context) (string, error) {
	obj, err := getCiceroneDBusObject()
	if err != nil {
		return "", err
	}

	resp := &cpb.GetLxdContainerUsernameResponse{}
	if err = dbusutil.CallProtoMethod(ctx, obj, dbusutil.CiceroneInterface+".GetLxdContainerUsername",
		&cpb.GetLxdContainerUsernameRequest{
			VmName:        c.VM.name,
			ContainerName: c.containerName,
			OwnerId:       c.VM.Concierge.ownerID,
		}, resp); err != nil {
		return "", err
	}

	if resp.GetStatus() != cpb.GetLxdContainerUsernameResponse_SUCCESS {
		return "", fmt.Errorf("failed to get username: %v", resp.GetFailureReason())
	}

	return resp.GetUsername(), nil
}

// SetUpUser sets up the default user in a container.
func (c *Container) SetUpUser(ctx context.Context) error {
	obj, err := getCiceroneDBusObject()
	if err != nil {
		return err
	}

	resp := &cpb.SetUpLxdContainerUserResponse{}
	if err = dbusutil.CallProtoMethod(ctx, obj, dbusutil.CiceroneInterface+".SetUpLxdContainerUser",
		&cpb.SetUpLxdContainerUserRequest{
			VmName:            c.VM.name,
			ContainerName:     c.containerName,
			OwnerId:           c.VM.Concierge.ownerID,
			ContainerUsername: c.username,
		}, resp); err != nil {
		return err
	}

	if resp.GetStatus() != cpb.SetUpLxdContainerUserResponse_SUCCESS &&
		resp.GetStatus() != cpb.SetUpLxdContainerUserResponse_EXISTS {
		return fmt.Errorf("failed to set up user: %v", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Set up user %q in container %q", c.username, c.containerName)
	return nil
}

// PushFile copies a local file to the container's filesystem.
func (c *Container) PushFile(ctx context.Context, localPath, containerPath string) error {
	testing.ContextLogf(ctx, "Copying local file %v to container %v", localPath, containerPath)
	// We base64 encode this and write it through terminal commands. We need to
	// base64 encode it since we are using the vsh command underneath which is a
	// terminal and binary control characters may interfere with its operation.
	fileData, err := ioutil.ReadFile(localPath)
	if err != nil {
		return err
	}
	base64Data := base64.StdEncoding.EncodeToString(fileData)
	// TODO(jkardatzke): Switch this to using stdin to pipe the data once
	// https://crbug.com/885255 is fixed.
	cmd := c.Command(ctx, "sh", "-c", "echo '"+base64Data+"' | base64 --decode >"+containerPath)
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}

// LinuxPackageInfo queries the container for information about a Linux package
// file. The packageId returned corresponds to the package ID for an installed
// package based on the PackageKit specification which is of the form
// 'package_id;version;arch;repository'.
func (c *Container) LinuxPackageInfo(ctx context.Context, path string) (err error, packageId string) {
	obj, err := getCiceroneDBusObject()
	if err != nil {
		return err, ""
	}

	resp := &cpb.LinuxPackageInfoResponse{}
	if err = dbusutil.CallProtoMethod(ctx, obj, dbusutil.CiceroneInterface+".GetLinuxPackageInfo",
		&cpb.LinuxPackageInfoRequest{
			VmName:        c.VM.name,
			ContainerName: c.containerName,
			OwnerId:       c.VM.Concierge.ownerID,
			FilePath:      path,
		}, resp); err != nil {
		return err, ""
	}

	if !resp.GetSuccess() {
		err = fmt.Errorf("failed to get Linux package info: %v", resp.GetFailureReason())
		return err, ""
	}

	return err, resp.GetPackageId()
}

// InstallPackage installs a Linux package file into the container.
func (c *Container) InstallPackage(ctx context.Context, path string) error {
	obj, err := getCiceroneDBusObject()
	if err != nil {
		return err
	}

	progress, err := dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusutil.CiceronePath,
		Interface: dbusutil.CiceroneInterface,
		Member:    "InstallLinuxPackageProgress",
	})
	if err != nil {
		return err
	}
	// Always close the InstallLinuxPackageProgress watcher regardless of success.
	defer progress.Close(ctx)

	resp := &cpb.InstallLinuxPackageResponse{}
	if err = dbusutil.CallProtoMethod(ctx, obj, dbusutil.CiceroneInterface+".InstallLinuxPackage",
		&cpb.LinuxPackageInfoRequest{
			VmName:        c.VM.name,
			ContainerName: c.containerName,
			OwnerId:       c.VM.Concierge.ownerID,
			FilePath:      path,
		}, resp); err != nil {
		return err
	}

	if resp.Status != cpb.InstallLinuxPackageResponse_STARTED {
		return fmt.Errorf("failed to start Linux package install: %v", resp.FailureReason)
	}

	// Wait for the signal for install completion which will signify success or
	// failure.
	testing.ContextLog(ctx, "Waiting for InstallLinuxPackageProgress D-Bus signal")
	sigResult := &cpb.InstallLinuxPackageProgressSignal{}
	for {
		select {
		case sig := <-progress.Signals:
			if len(sig.Body) == 0 {
				return errors.New("InstallLinuxPackageProgress signal lacked a body")
			}
			buf, ok := sig.Body[0].([]byte)
			if !ok {
				return errors.New("InstallLinuxPackageProgress signal body is not a byte slice")
			}
			if err := proto.Unmarshal(buf, sigResult); err != nil {
				return fmt.Errorf("failed unmarshaling InstallLinuxPackageProgress body: %v", err)
			}
			if sigResult.VmName == c.VM.name && sigResult.ContainerName == c.containerName &&
				sigResult.OwnerId == c.VM.Concierge.ownerID {
				if sigResult.Status == cpb.InstallLinuxPackageProgressSignal_SUCCEEDED {
					return nil
				}
				if sigResult.Status == cpb.InstallLinuxPackageProgressSignal_FAILED {
					return fmt.Errorf("failure with Linux package install: %v", sigResult.FailureDetails)
				}
			}
		case <-ctx.Done():
			return fmt.Errorf("didn't get InstallLinuxPackageProgress D-Bus signal: %v", ctx.Err())
		}
	}
}

// Command returns a testexec.Cmd with a vsh command that will run in this
// container.
func (c *Container) Command(ctx context.Context, vshArgs ...string) *testexec.Cmd {
	args := append([]string{"--vm_name=" + c.VM.name,
		"--target_container=" + c.containerName,
		"--owner_id=" + c.VM.Concierge.ownerID,
		"--"},
		vshArgs...)
	cmd := testexec.CommandContext(ctx, "vsh", args...)
	// Add a dummy buffer for stdin to force allocating a pipe. vsh uses
	// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
	cmd.Stdin = &bytes.Buffer{}
	return cmd
}

// DumpLog dumps the logs from the container to a local output file named
// container_log.txt. It does this by executing journalctl in the container
// and grabbing the output.
func (c *Container) DumpLog(s *testing.State) error {
	f, err := os.Create(filepath.Join(s.OutDir(), "container_log.txt"))
	if err != nil {
		return err
	}
	defer f.Close()

	// TODO(jkardatzke): Remove stripping off the color codes that show up in
	// journalctl once crbug.com/888102 is fixed.
	cmd := c.Command(s.Context(), "sh", "-c",
		"sudo journalctl --no-pager | tr -cd '[:space:][:print:]'")
	cmd.Stdout = f
	return cmd.Run()
}
