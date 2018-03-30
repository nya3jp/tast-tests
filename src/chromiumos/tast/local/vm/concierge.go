// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"

	"chromiumos/systemapi/concierge"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
)

const (
	componentName = "cros-termina"         // The name of the Chrome component for the VM kernel and rootfs.
	conciergeJob  = "vm_concierge"         // The name of the upstart job for concierge.
	testDiskSize  = 4 * 1024 * 1024 * 1024 // 4 GiB default disk size.
	testName      = "test_vm"              // The default VM name during testing.
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
		return nil, fmt.Errorf("failed to mount component: %v", err)
	}
	testing.ContextLog(ctx, "Mounted component at path ", resp)

	testing.ContextLogf(ctx, "Restarting %v job", conciergeJob)
	if err = upstart.RestartJob(conciergeJob); err != nil {
		return nil, fmt.Errorf("failed to start concierge: %v", err)
	}

	if err = dbusutil.WaitForService(ctx, bus, dbusutil.ConciergeName); err != nil {
		return nil, fmt.Errorf("concierge did not start: %v", err)
	}

	return &Concierge{h}, nil
}

func (c *Concierge) createDiskImage() (diskPath string, err error) {
	req, err := proto.Marshal(&concierge.CreateDiskImageRequest{
		CryptohomeId:    c.cryptohomeHash,
		DiskPath:        testName,
		DiskSize:        testDiskSize,
		ImageType:       concierge.DiskImageType_DISK_IMAGE_QCOW2,
		StorageLocation: concierge.StorageLocation_STORAGE_CRYPTOHOME_ROOT,
	})
	if err != nil {
		return "", err
	}

	obj, err := getDBusObject()
	if err != nil {
		return "", err
	}

	var marshaledResp []byte
	if err = obj.Call(dbusutil.ConciergeInterface+".CreateDiskImage", 0, req).Store(&marshaledResp); err != nil {
		return "", err
	}

	resp := &concierge.CreateDiskImageResponse{}
	if err = proto.Unmarshal(marshaledResp, resp); err != nil {
		return "", err
	}

	diskStatus := resp.GetStatus()
	if diskStatus != concierge.DiskImageStatus_DISK_STATUS_CREATED &&
		diskStatus != concierge.DiskImageStatus_DISK_STATUS_EXISTS {
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

	req, err := proto.Marshal(&concierge.StartVmRequest{
		Name:         testName,
		StartTermina: true,
		Disks: []*concierge.DiskImage{
			&concierge.DiskImage{
				Path:      diskPath,
				ImageType: concierge.DiskImageType_DISK_IMAGE_QCOW2,
				Writable:  true,
				DoMount:   false,
			},
		},
	})
	if err != nil {
		return err
	}

	obj, err := getDBusObject()
	if err != nil {
		return err
	}

	var marshaledResp []byte
	if err = obj.Call(dbusutil.ConciergeInterface+".StartVm", 0, req).Store(&marshaledResp); err != nil {
		return err
	}

	resp := &concierge.StartVmResponse{}
	if err = proto.Unmarshal(marshaledResp, resp); err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("failed to start VM: %s", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Started VM %q with CID %d and PID %d", testName, resp.VmInfo.Cid, resp.VmInfo.Pid)

	return nil
}
