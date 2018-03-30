// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package virtual_machine interacts with the vm_concierge service.
package vm

import (
	"context"
	"fmt"
	//"os/exec"
	//"regexp"
	//"strings"

	"chromiumos/system_api/vm_concierge"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
)

const (
	conciergeName      = "org.chromium.VmConcierge"
	conciergePath      = "/org/chromium/VmConcierge"
	conciergeInterface = "org.chromium.VmConcierge"

	componentUpdaterName      = "org.chromium.ComponentUpdaterService"
	componentUpdaterPath      = "/org/chromium/ComponentUpdaterService"
	componentUpdaterInterface = "org.chromium.ComponentUpdaterService"
)

const (
	testDiskSize = 4 * 1024 * 1024 * 1024 // 4 GiB.
	testName     = "test_vm"
)

type Concierge struct {
	cryptohomeHash string
}

func New(ctx context.Context, cr *chrome.Chrome) (*Concierge, error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	h, err := chrome.CryptohomeHash(cr)
	if err != nil {
		return nil, fmt.Errorf("Failed to get hash: %v", err)
	}

	comp_updater := bus.Object(componentUpdaterName, dbus.ObjectPath(componentUpdaterPath))

	var resp string
	err = comp_updater.Call(componentUpdaterInterface+".LoadComponent", 0, "cros-termina").Store(&resp)
	if err != nil {
		return nil, fmt.Errorf("Failed to mount component: %v", err)
	}
	testing.ContextLog(ctx, "Mounted component at path ", resp)

	err = upstart.RestartJob("vm_concierge")
	if err != nil {
		return nil, fmt.Errorf("Failed to start concierge: %v", err)
	}
	err = dbusutil.WaitForService(ctx, bus, conciergeName)
	if err != nil {
		return nil, fmt.Errorf("Concierge did not start: %v", err)
	}

	vm := &Concierge{
		cryptohomeHash: h,
	}

	return vm, nil
}

func (c *Concierge) createDiskImage(bus *dbus.Conn) (string, error) {
	obj := bus.Object(conciergeName, dbus.ObjectPath(conciergePath))

	req := &vm_concierge.CreateDiskImageRequest{
		CryptohomeId:    c.cryptohomeHash,
		DiskPath:        testName,
		DiskSize:        testDiskSize,
		ImageType:       vm_concierge.DiskImageType_DISK_IMAGE_QCOW2,
		StorageLocation: vm_concierge.StorageLocation_STORAGE_CRYPTOHOME_ROOT,
	}

	req_bytes, err := proto.Marshal(req)
	if err != nil {
		return "", err
	}

	var resp_bytes []byte

	err = obj.Call(conciergeInterface+".CreateDiskImage", 0, req_bytes).Store(&resp_bytes)
	if err != nil {
		return "", err
	}

	resp := &vm_concierge.CreateDiskImageResponse{}
	err = proto.Unmarshal(resp_bytes, resp)
	if err != nil {
		return "", err
	}

	disk_status := resp.GetStatus()
	if disk_status != vm_concierge.DiskImageStatus_DISK_STATUS_CREATED &&
		disk_status != vm_concierge.DiskImageStatus_DISK_STATUS_EXISTS {
		return "", fmt.Errorf("Could not create disk image: %v", resp.GetFailureReason())
	}

	return resp.GetDiskPath(), nil
}

// Start a VM.
func (c *Concierge) StartTerminaVM(ctx context.Context, name string) error {
	bus, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	obj := bus.Object(conciergeName, dbus.ObjectPath(conciergePath))

	// Create the disk first.
	disk_path, err := c.createDiskImage(bus)
	if err != nil {
		return err
	}

	req := &vm_concierge.StartVmRequest{}
	req.Name = testName
	req.StartTermina = true

	stateful_disk := &vm_concierge.DiskImage{
		Path:      disk_path,
		ImageType: vm_concierge.DiskImageType_DISK_IMAGE_QCOW2,
		Writable:  false,
		DoMount:   false,
	}

	req.Disks = append(req.Disks, stateful_disk)

	req_bytes, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	var resp_bytes []byte

	err = obj.Call(conciergeInterface+".StartVm", 0, req_bytes).Store(&resp_bytes)
	if err != nil {
		return err
	}

	resp := &vm_concierge.StartVmResponse{}
	err = proto.Unmarshal(resp_bytes, resp)
	if err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return fmt.Errorf("Failed to start VM: %s", resp.GetFailureReason())
	}

	testing.ContextLog(ctx, "Started VM '%s'. cid: %d pid: %d", req.Name, resp.VmInfo.Cid, resp.VmInfo.Pid)

	return nil
}
