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
	testDiskSize = 4 * 1024 * 1024 * 1024 // 4 GiB default disk size.
	testName     = "test_vm"              // The default VM name during testing.
)

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
	err = updater.Call(dbusutil.ComponentUpdaterInterface+".LoadComponent", 0, "cros-termina").Store(&resp)
	if err != nil {
		return nil, fmt.Errorf("failed to mount component: %v", err)
	}
	testing.ContextLog(ctx, "Mounted component at path ", resp)

	if err = upstart.RestartJob("vm_concierge"); err != nil {
		return nil, fmt.Errorf("failed to start concierge: %v", err)
	}

	if err = dbusutil.WaitForService(ctx, bus, dbusutil.ConciergeName); err != nil {
		return nil, fmt.Errorf("Concierge did not start: %v", err)
	}

	return &Concierge{h}, nil
}

func (c *Concierge) createDiskImage(bus *dbus.Conn) (diskPath string, err error) {
	obj := bus.Object(dbusutil.ConciergeName, dbus.ObjectPath(dbusutil.ConciergePath))

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
		return "", fmt.Errorf("Could not create disk image: %v", resp.GetFailureReason())
	}

	return resp.GetDiskPath(), nil
}

// StartTerminaVM will create a stateful disk and start a Termina VM.
func (c *Concierge) StartTerminaVM(ctx context.Context) error {
	bus, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	obj := bus.Object(dbusutil.ConciergeName, dbus.ObjectPath(dbusutil.ConciergePath))

	// Create the new disk first.
	diskPath, err := c.createDiskImage(bus)
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

	var marshaledResp []byte
	if err = obj.Call(dbusutil.ConciergeInterface+".StartVm", 0, req).Store(&marshaledResp); err != nil {
		return err
	}

	resp := &concierge.StartVmResponse{}
	if err = proto.Unmarshal(marshaledResp, resp); err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("Failed to start VM: %s", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Started VM '%s'. cid: %d pid: %d", testName, resp.VmInfo.Cid, resp.VmInfo.Pid)

	return nil
}
