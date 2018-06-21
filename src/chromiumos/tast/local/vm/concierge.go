// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"errors"
	"fmt"

	cpb "chromiumos/system_api/vm_cicerone_proto"   // protobufs for container management
	vmpb "chromiumos/system_api/vm_concierge_proto" // protobufs for VM management
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"
)

const (
	conciergeJob = "vm_concierge"         // name of the upstart job for concierge
	ciceroneJob  = "vm_cicerone"          // name of the upstart job for cicerone
	testDiskSize = 4 * 1024 * 1024 * 1024 // 4 GiB default disk size
	testVMName   = "termina"              // default VM name during testing (must be a valid hostname)
)

func getConciergeDBusObject() (obj dbus.BusObject, err error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	return bus.Object(dbusutil.ConciergeName, dbus.ObjectPath(dbusutil.ConciergePath)), nil
}

func getCiceroneDBusObject() (obj dbus.BusObject, err error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	return bus.Object(dbusutil.CiceroneName, dbus.ObjectPath(dbusutil.CiceronePath)), nil
}

// Concierge interacts with the vm_concierge daemon, which starts, stops, and
// monitors VMs. It also interacts with the cicerone daemon, which interacts
// with containers inside those VMs.
type Concierge struct {
	ownerID string // cryptohome hash for the logged-in user
}

// NewConcierge restarts the vm_concierge service, which stops all running VMs.
func NewConcierge(ctx context.Context, user string) (*Concierge, error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	h, err := cryptohome.UserHash(user)
	if err != nil {
		return nil, err
	}

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

// StopConcierge stops the vm_concierge service, which stops all running VMs.
func StopConcierge(ctx context.Context) error {
	testing.ContextLogf(ctx, "Stopping %v job", conciergeJob)
	if err := upstart.StopJob(conciergeJob); err != nil {
		return fmt.Errorf("%v Upstart job failed to stop: %v", conciergeJob, err)
	}

	return nil
}

func (c *Concierge) createDiskImage() (diskPath string, err error) {
	obj, err := getConciergeDBusObject()
	if err != nil {
		return "", err
	}
	resp := &vmpb.CreateDiskImageResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.ConciergeInterface+".CreateDiskImage",
		&vmpb.CreateDiskImageRequest{
			CryptohomeId:    c.ownerID,
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
func (c *Concierge) StartTerminaVM(ctx context.Context) (*VM, error) {
	// Create the new disk first.
	diskPath, err := c.createDiskImage()
	if err != nil {
		return nil, err
	}

	obj, err := getConciergeDBusObject()
	if err != nil {
		return nil, err
	}

	tremplin, err := dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusutil.CiceronePath,
		Interface: dbusutil.CiceroneInterface,
		Member:    "TremplinStarted",
	})
	defer tremplin.Close()

	resp := &vmpb.StartVmResponse{}
	if err = dbusutil.CallProtoMethod(obj, dbusutil.ConciergeInterface+".StartVm",
		&vmpb.StartVmRequest{
			Name:         testVMName,
			StartTermina: true,
			OwnerId:      c.ownerID,
			Disks: []*vmpb.DiskImage{
				&vmpb.DiskImage{
					Path:      diskPath,
					ImageType: vmpb.DiskImageType_DISK_IMAGE_QCOW2,
					Writable:  true,
					DoMount:   false,
				},
			},
		}, resp); err != nil {
		return nil, err
	}
	if !resp.GetSuccess() {
		return nil, fmt.Errorf("failed to start VM: %s", resp.GetFailureReason())
	}

	testing.ContextLog(ctx, "Waiting for TremplinStarted D-Bus signal")
	sigResult := &cpb.TremplinStartedSignal{}
	select {
	case sig := <-tremplin.Signals:
		if len(sig.Body) == 0 {
			return nil, errors.New("TremplinStarted signal lacked a body")
		}
		buf, ok := sig.Body[0].([]byte)
		if !ok {
			return nil, errors.New("TremplinStarted signal body is not a byte slice")
		}
		if err := proto.Unmarshal(buf, sigResult); err != nil {
			return nil, fmt.Errorf("failed unmarshaling TremplinStarted body: %v", err)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("didn't get TremplinStarted D-Bus signal: %v", ctx.Err())
	}

	if sigResult.OwnerId != c.ownerID {
		return nil, fmt.Errorf("expected owner id %q, received %q", c.ownerID, sigResult.OwnerId)
	}
	if sigResult.VmName != testVMName {
		return nil, fmt.Errorf("expected VM name %q, received %q", testVMName, sigResult.VmName)
	}

	testing.ContextLogf(ctx, "Started VM %q with CID %d and PID %d", testVMName, resp.VmInfo.Cid, resp.VmInfo.Pid)

	return &VM{Concierge: c, name: testVMName}, nil
}
