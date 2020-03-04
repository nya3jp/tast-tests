// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus"

	spb "chromiumos/system_api/seneschal_proto" // protobufs for seneschal
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// DefaultVMName is the default crostini VM name.
	DefaultVMName = "termina"
	// DefaultContainerName is the default crostini container name.
	DefaultContainerName = "penguin"

	seneschalName      = "org.chromium.Seneschal"
	seneschalPath      = dbus.ObjectPath("/org/chromium/Seneschal")
	seneschalInterface = "org.chromium.Seneschal"
)

// VM encapsulates a virtual machine managed by the concierge/cicerone daemons.
type VM struct {
	// Concierge is the Concierge instance managing this VM.
	Concierge       *Concierge
	name            string // name of the VM
	ContextID       int64  // cid for the crosvm process
	seneschalHandle uint32 // seneschal handle for the VM
	EnableGPU       bool   // hardware GPU support
	DiskPath        string // the location of the stateful disk
}

// NewDefaultVM gets a default VM instance.
func NewDefaultVM(c *Concierge) *VM {
	return &VM{
		Concierge:       c,
		name:            DefaultVMName,
		ContextID:       -1,    // not populated until VM is started.
		seneschalHandle: 0,     // not populated until VM is started.
		EnableGPU:       false, // disable hardware GPU by default.
	}
}

// CreateDefaultVM prepares a VM with default settings either the live or
// staging container versions. The directory dir may be used to store
// logs on failure. If the container type is Tarball, then artifactPath
// must be specified with the path to the tarball containing the termina VM.
// Otherwise, artifactPath is ignored. If enableGPU is set, VM will try to use
// hardware gpu if possible.
func CreateDefaultVM(ctx context.Context, dir, user string, t ContainerType, artifactPath string, enableGPU bool, diskSize uint64) (*VM, error) {
	userPath, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user Downloads dir")
	}

	if t.Image == Tarball {
		// Put the container rootfs and metadata tarballs in a subdirectory of
		// Downloads for 9P sharing with the guest.
		containerPath := filepath.Join(userPath, "Downloads/crostini")
		if err := os.MkdirAll(containerPath, 0755); err != nil {
			return nil, errors.Wrap(err, "failed to mkdir for container image")
		}

		testing.ContextLog(ctx, "Extracting container tarballs")
		if err := testexec.CommandContext(ctx, "tar", "xvf", artifactPath, "-C", containerPath, "container_metadata.tar.xz", "container_rootfs.tar.xz").Run(testexec.DumpLogOnError); err != nil {
			return nil, errors.Wrap(err, "failed to untar container image")
		}
	}

	concierge, err := NewConcierge(ctx, user)
	concierge.diskSize = diskSize
	if err != nil {
		if stopErr := StopConcierge(ctx); stopErr != nil {
			testing.ContextLog(ctx, "Failed to stop concierge: ", stopErr)
		}
		return nil, err
	}

	vmInstance := NewDefaultVM(concierge)
	vmInstance.EnableGPU = enableGPU
	if err := vmInstance.Start(ctx); err != nil {
		if stopErr := StopConcierge(ctx); stopErr != nil {
			testing.ContextLog(ctx, "Failed to stop concierge: ", stopErr)
		}
		return nil, err
	}
	if t.Image == Tarball {
		if _, err := vmInstance.ShareDownloadsPath(ctx, "crostini", false); err != nil {
			if stopErr := StopConcierge(ctx); stopErr != nil {
				testing.ContextLog(ctx, "Failed to stop concierge: ", stopErr)
			}
			return nil, errors.Wrap(err, "failed to share container image with VM")
		}
	}
	return vmInstance, nil
}

func (vm *VM) vmCommand(ctx context.Context, args ...string) *testexec.Cmd {
	args = append([]string{
		"--vm_name=" + vm.name,
		"--owner_id=" + vm.Concierge.ownerID,
		"--"},
		args...)
	cmd := testexec.CommandContext(ctx, "vsh", args...)

	cmd.Stdin = &bytes.Buffer{}
	return cmd
}

// Start launches the VM.
func (vm *VM) Start(ctx context.Context) error {
	diskPath, err := vm.Concierge.startTerminaVM(ctx, vm)
	if err != nil {
		return err
	}

	vm.DiskPath = diskPath

	cmd := vm.vmCommand(ctx, "grep", "CHROMEOS_RELEASE_VERSION=", "/etc/lsb-release")
	if output, err := cmd.Output(testexec.DumpLogOnError); err != nil {
		testing.ContextLog(ctx, "Failed to get VM image version")
	} else {
		version := strings.Split(string(output), "=")[1]
		testing.ContextLog(ctx, "VM image version is ", version)
	}
	return nil
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

// ShareDownloadsPath shares a path relative to Downloads with the VM.
func (vm *VM) ShareDownloadsPath(ctx context.Context, path string, writable bool) (string, error) {
	_, seneschalObj, err := dbusutil.Connect(ctx, seneschalName, seneschalPath)
	if err != nil {
		return "", err
	}

	resp := &spb.SharePathResponse{}
	if err := dbusutil.CallProtoMethod(ctx, seneschalObj, seneschalInterface+".SharePath",
		&spb.SharePathRequest{
			Handle: vm.seneschalHandle,
			SharedPath: &spb.SharedPath{
				Path:     path,
				Writable: writable,
			},
			StorageLocation: spb.SharePathRequest_DOWNLOADS,
			OwnerId:         vm.Concierge.ownerID,
		}, resp); err != nil {
		return "", err
	}

	if !resp.Success {
		return "", errors.New(resp.FailureReason)
	}

	return resp.Path, nil
}

// UnshareDownloadsPath un-shares a path that was previously shared by calling ShareDownloadsPath.
func (vm *VM) UnshareDownloadsPath(ctx context.Context, path string) error {
	_, seneschalObj, err := dbusutil.Connect(ctx, seneschalName, seneschalPath)
	if err != nil {
		return err
	}

	resp := &spb.UnsharePathResponse{}
	if err := dbusutil.CallProtoMethod(ctx, seneschalObj, seneschalInterface+".UnsharePath",
		&spb.UnsharePathRequest{
			Handle: vm.seneschalHandle,
			Path:   path,
		}, resp); err != nil {
		return err
	}

	if !resp.Success {
		return errors.New(resp.FailureReason)
	}

	return nil
}
