// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"strings"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	spb "chromiumos/system_api/seneschal_proto"   // protobufs for seneschal
	cpb "chromiumos/system_api/vm_cicerone_proto" // protobufs for container management
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	// DefaultVMName is the default crostini VM name.
	DefaultVMName = "termina"
	// DefaultContainerName is the default crostini container name.
	DefaultContainerName = "penguin"
	// DefaultDiskSize is the default disk size for VM. 2.5 GB by default.
	DefaultDiskSize = 5 * 512 * 1024 * 1024 // 2.5 GiB default disk size

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
	diskSize        uint64 // actual disk size in bytes
	targetDiskSize  uint64 // targeted disk size during creation time
}

// NewDefaultVM gets a default VM instance. enableGPU enabled the hardware gpu support for the VM. diskSize set the targeted disk size of the VM.
func NewDefaultVM(c *Concierge, enableGPU bool, diskSize uint64) *VM {
	return &VM{
		Concierge:       c,
		name:            DefaultVMName,
		ContextID:       -1,        // not populated until VM is started.
		seneschalHandle: 0,         // not populated until VM is started.
		EnableGPU:       enableGPU, // enable the gpu if set.
		diskSize:        0,         // not populated until VM is started.
		targetDiskSize:  diskSize,
	}
}

// GetRunningVM creates a VM struct for the VM that is currently running.
func GetRunningVM(ctx context.Context, user string) (*VM, error) {
	c, err := GetRunningConcierge(ctx, user)
	if err != nil {
		return nil, err
	}
	vm := NewDefaultVM(c, false, 0)
	if err := c.getVMInfo(ctx, vm); err != nil {
		return nil, errors.Wrapf(err, "failed to get info for %q VM", vm.name)
	}
	return vm, nil
}

// Name returns the human-readable name of this VM (as opposed to the
// encoded one from vm.GetEncodedName()).
func (vm *VM) Name() string {
	return vm.name
}

// Start launches the VM.
func (vm *VM) Start(ctx context.Context) error {
	diskPath, err := vm.Concierge.startTerminaVM(ctx, vm)
	if err != nil {
		return err
	}
	vm.DiskPath = diskPath

	diskSize, err := vm.Concierge.listVMDisksSize(ctx, vm.name)
	if err != nil {
		return err
	}
	vm.diskSize = diskSize

	cmd := vm.Command(ctx, "grep", "CHROMEOS_RELEASE_VERSION=", "/etc/lsb-release")
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

// StartLxd starts the LXD daemon inside the VM. This is a required
// step before performing any container operations.
func (vm *VM) StartLxd(ctx context.Context) error {
	lxd, err := dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      ciceronePath,
		Interface: ciceroneInterface,
		Member:    "StartLxdProgress",
	})
	defer lxd.Close(ctx)

	resp := &cpb.StartLxdResponse{}
	if err = dbusutil.CallProtoMethod(ctx, vm.Concierge.ciceroneObj, ciceroneInterface+".StartLxd",
		&cpb.StartLxdRequest{
			VmName:  vm.name,
			OwnerId: vm.Concierge.GetOwnerID(),
		}, resp); err != nil {
		return err
	}

	status := resp.GetStatus()
	if status == cpb.StartLxdResponse_ALREADY_RUNNING {
		testing.ContextLog(ctx, "LXD is already running")
		return nil
	}

	testing.ContextLog(ctx, "Waiting for LXD to start")
	sigResult := &cpb.StartLxdProgressSignal{}
	for {
		select {
		case sig := <-lxd.Signals:
			if len(sig.Body) != 1 {
				return errors.Errorf("StartLxdProgressSignal signal has %d elements, expected 1", len(sig.Body))
			}
			buf, ok := sig.Body[0].([]byte)
			if !ok {
				return errors.New("StartLxdProgressSignal signal body is not a byte slice")
			}
			if err := proto.Unmarshal(buf, sigResult); err != nil {
				return errors.Wrap(err, "failed unmarshaling StartLxdProgressSignal body")
			}
			if sigResult.GetStatus() == cpb.StartLxdProgressSignal_STARTED {
				testing.ContextLog(ctx, "LXD started successfully")
				return nil
			}
			if sigResult.GetStatus() == cpb.StartLxdProgressSignal_FAILED {
				return errors.Errorf("failed to start LXD: %s", sigResult.GetFailureReason())
			}
			testing.ContextLogf(ctx, "Got StartLxdProgressSignal signal: %s", cpb.StartLxdProgressSignal_Status_name[int32(sigResult.GetStatus())])
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "didn't get StartLxdProgressSignal D-Bus signal")
		}
	}
}

// Command returns a testexec.Cmd with a vsh command that will run in this VM.
func (vm *VM) Command(ctx context.Context, vshArgs ...string) *testexec.Cmd {
	args := append([]string{"--vm_name=" + vm.name,
		"--owner_id=" + vm.Concierge.ownerID,
		"--"},
		vshArgs...)
	cmd := testexec.CommandContext(ctx, "vsh", args...)
	// Add an empty buffer for stdin to force allocating a pipe. vsh uses
	// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
	cmd.Stdin = &bytes.Buffer{}
	return cmd
}

// LXCCommand runs lxc inside the VM with the specified args.
func (vm *VM) LXCCommand(ctx context.Context, lxcArgs ...string) error {
	envLXC := []string{"env", "LXD_DIR=/mnt/stateful/lxd", "LXD_CONF=/mnt/stateful/lxd_conf", "lxc"}
	cmd := vm.Command(ctx, append(envLXC, lxcArgs...)...)
	err := cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to run %q", strings.Join(cmd.Args, " "))
	}
	return nil
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
