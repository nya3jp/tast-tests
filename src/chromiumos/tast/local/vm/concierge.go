// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"

	vmpb "chromiumos/system_api/vm_concierge_proto"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
)

const (
	componentName         = "cros-termina"         // The name of the Chrome component for the VM kernel and rootfs.
	conciergeJob          = "vm_concierge"         // The name of the upstart job for vmpb.
	ciceroneJob           = "vm_cicerone"          // The name of the upstart job for cicerone
	testDiskSize          = 4 * 1024 * 1024 * 1024 // 4 GiB default disk size.
	testVMName            = "testVM"               // The default VM name during testing (must be a valid hostname).
	testContainerName     = "testContainer"        // The default container name during testing (must be a valid hostname).
	testContainerUsername = "testuser"             // The default container username during testing
)

func getDBusObject() (obj dbus.BusObject, err error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	return bus.Object(dbusutil.ConciergeName, dbus.ObjectPath(dbusutil.ConciergePath)), nil
}

// Concierge interacts with the vm_concierge daemon, which starts, stops, and
// monitors VMs and containers.
type Concierge struct {
	cryptohomeHash string // cryptohome hash for the logged-in user
}

// New restarts the vm_concierge service, which stops all running VMs. New will
// also mount the cros-termina component with the Termina VM image.
func New(ctx context.Context, user string) (*Concierge, error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	h, err := cryptohome.UserHash(user)
	if err != nil {
		return nil, err
	}

	updater := bus.Object(dbusutil.ComponentUpdaterName, dbus.ObjectPath(dbusutil.ComponentUpdaterPath))

	var resp string
	testing.ContextLogf(ctx, "Mounting %q component", componentName)
	err = updater.Call(dbusutil.ComponentUpdaterInterface+".LoadComponent", 0, componentName).Store(&resp)
	if err != nil {
		return nil, fmt.Errorf("mounting %q component failed: %v", componentName, err)
	}
	testing.ContextLog(ctx, "Mounted component at path ", resp)

	testing.ContextLogf(ctx, "Restarting %v job", conciergeJob)
	if err = upstart.RestartJob(conciergeJob); err != nil {
		return nil, fmt.Errorf("%v Upstart job failed: %v", conciergeJob, err)
	}

	if err = dbusutil.WaitForService(ctx, bus, dbusutil.ConciergeName); err != nil {
		return nil, fmt.Errorf("%v D-Bus service unavailable: %v", dbusutil.ConciergeName, err)
	}

	if err = upstart.RestartJob(ciceroneJob); err != nil {
		return nil, fmt.Errorf("%v Upstart job failed: %v", ciceroneJob, err)
	}

	if err = dbusutil.WaitForService(ctx, bus, dbusutil.CiceroneName); err != nil {
		return nil, fmt.Errorf("%v D-Bus service unavailable: %v", dbusutil.CiceroneName, err)
	}

	return &Concierge{h}, nil
}

func (c *Concierge) createDiskImage() (diskPath string, err error) {
	obj, err := getDBusObject()
	if err != nil {
		return "", err
	}
	resp := &vmpb.CreateDiskImageResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.ConciergeInterface+".CreateDiskImage",
		&vmpb.CreateDiskImageRequest{
			CryptohomeId:    c.cryptohomeHash,
			DiskPath:        testVMName,
			DiskSize:        testDiskSize,
			ImageType:       vmpb.DiskImageType_DISK_IMAGE_QCOW2,
			StorageLocation: vmpb.StorageLocation_STORAGE_CRYPTOHOME_ROOT,
		}, resp); err != nil {
		return "", err
	}

	diskStatus := resp.GetStatus()
	if diskStatus != vmpb.DiskImageStatus_DISK_STATUS_CREATED &&
		diskStatus != vmpb.DiskImageStatus_DISK_STATUS_EXISTS {
		return "", fmt.Errorf("could not create disk image: %v", resp.GetFailureReason())
	}

	return resp.GetDiskPath(), nil
}

// StartTerminaVM will create a stateful disk and start a Termina VM.
func (c *Concierge) StartTerminaVM(ctx context.Context) error {
	// Create the new disk first.
	diskPath, err := c.createDiskImage()
	if err != nil {
		return err
	}

	obj, err := getDBusObject()
	if err != nil {
		return err
	}
	resp := &vmpb.StartVmResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.ConciergeInterface+".StartVm",
		&vmpb.StartVmRequest{
			Name:         testVMName,
			StartTermina: true,
			Disks: []*vmpb.DiskImage{
				&vmpb.DiskImage{
					Path:      diskPath,
					ImageType: vmpb.DiskImageType_DISK_IMAGE_QCOW2,
					Writable:  true,
					DoMount:   false,
				},
			},
		}, resp); err != nil {
		return err
	}
	if !resp.GetSuccess() {
		return fmt.Errorf("failed to start VM: %s", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Started VM %q with CID %d and PID %d", testVMName, resp.VmInfo.Cid, resp.VmInfo.Pid)
	return nil
}

func (c *Concierge) StartContainer(ctx context.Context) error {
	obj, err := getDBusObject()
	if err != nil {
		return err
	}
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	started, err := dbusutil.NewSignalWatcher(ctx, conn, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusutil.CiceronePath,
		Interface: dbusutil.CiceroneInterface,
		Member:    "ContainerStarted",
	})
	defer started.Close()

	failed, err := dbusutil.NewSignalWatcher(ctx, conn, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusutil.ConciergePath,
		Interface: dbusutil.ConciergeInterface,
		Member:    "ContainerStartupFailed",
	})
	defer failed.Close()

	resp := &vmpb.StartContainerResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.ConciergeInterface+".StartContainer",
		&vmpb.StartContainerRequest{
			VmName:            testVMName,
			ContainerName:     testContainerName,
			ContainerUsername: testContainerUsername,
			CryptohomeId:      c.cryptohomeHash,
		}, resp); err != nil {
		return err
	}
	switch resp.GetStatus() {
	case vmpb.ContainerStatus_CONTAINER_STATUS_UNKNOWN:
		return fmt.Errorf("failed to start Container: %v", resp.GetFailureReason())
	case vmpb.ContainerStatus_CONTAINER_STATUS_FAILURE:
		return fmt.Errorf("failed to start Container: %v", resp.GetFailureReason())
	case vmpb.ContainerStatus_CONTAINER_STATUS_RUNNING:
		testing.ContextLogf(ctx, "Container %q already runnning in VM %q.", testContainerName, testVMName)
		return nil
	case vmpb.ContainerStatus_CONTAINER_STATUS_STARTING:
		testing.ContextLogf(ctx, "Now waiting for ContainerStartedSignal for container %q, VM %q.", testContainerName, testVMName)
	}

	// container is starting, wait for signal.
	select {
	case <-started.Signals:
		testing.ContextLogf(ctx, "Container %q runnning in VM %q.", testContainerName, testVMName)
	case <-failed.Signals:
		return fmt.Errorf("Failed to start container %q in VM %q.", testContainerName, testVMName)
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
