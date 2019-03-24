// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	cpb "chromiumos/system_api/vm_cicerone_proto"   // protobufs for container management
	vmpb "chromiumos/system_api/vm_concierge_proto" // protobufs for VM management
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	conciergeJob = "vm_concierge"         // name of the upstart job for concierge
	ciceroneJob  = "vm_cicerone"          // name of the upstart job for cicerone
	testDiskSize = 4 * 1024 * 1024 * 1024 // 4 GiB default disk size

	conciergeName      = "org.chromium.VmConcierge"
	conciergePath      = dbus.ObjectPath("/org/chromium/VmConcierge")
	conciergeInterface = "org.chromium.VmConcierge"
)

// Concierge interacts with the vm_concierge daemon, which starts, stops, and
// monitors VMs. It also interacts with the cicerone daemon, which interacts
// with containers inside those VMs.
type Concierge struct {
	ownerID      string // cryptohome hash for the logged-in user
	conciergeObj dbus.BusObject
}

// GetRunningConcierge returns a concierge instance without restarting concierge service.
// Returns an error if concierge is not available.
func GetRunningConcierge(ctx context.Context, user string) (*Concierge, error) {
	h, err := cryptohome.UserHash(user)
	if err != nil {
		return nil, err
	}

	// Try to get a connection to a running concierge instance, if it's not available,
	// returns with an error immediately.
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to system bus")
	}
	if !dbusutil.ServiceOwned(ctx, conn, conciergeName) {
		return nil, errors.Wrapf(err, "%s is not owned", conciergeName)
	}

	obj := conn.Object(conciergeName, conciergePath)
	return &Concierge{h, obj}, nil
}

// NewConcierge restarts the vm_concierge service, which stops all running VMs.
func NewConcierge(ctx context.Context, user string) (*Concierge, error) {
	h, err := cryptohome.UserHash(user)
	if err != nil {
		return nil, err
	}

	testing.ContextLogf(ctx, "Restarting %v job", conciergeJob)
	if err = upstart.RestartJob(ctx, conciergeJob); err != nil {
		return nil, errors.Wrapf(err, "%v Upstart job failed", conciergeJob)
	}
	bus, obj, err := dbusutil.Connect(ctx, conciergeName, conciergePath)
	if err != nil {
		return nil, err
	}

	testing.ContextLogf(ctx, "Restarting %v job", ciceroneJob)
	if err = upstart.RestartJob(ctx, ciceroneJob); err != nil {
		return nil, errors.Wrapf(err, "%v Upstart job failed", ciceroneJob)
	}
	if err = dbusutil.WaitForService(ctx, bus, ciceroneName); err != nil {
		return nil, errors.Wrapf(err, "%v D-Bus service unavailable", ciceroneName)
	}

	return &Concierge{h, obj}, nil
}

// StopConcierge stops the vm_concierge service, which stops all running VMs.
func StopConcierge(ctx context.Context) error {
	testing.ContextLogf(ctx, "Stopping %v job", conciergeJob)
	if err := upstart.StopJob(ctx, conciergeJob); err != nil {
		return errors.Wrapf(err, "%v Upstart job failed to stop", conciergeJob)
	}

	return nil
}

func (c *Concierge) createDiskImage(ctx context.Context) (diskPath string, err error) {
	resp := &vmpb.CreateDiskImageResponse{}
	if err = dbusutil.CallProtoMethod(ctx, c.conciergeObj, conciergeInterface+".CreateDiskImage",
		&vmpb.CreateDiskImageRequest{
			CryptohomeId:    c.ownerID,
			DiskPath:        DefaultVMName,
			DiskSize:        testDiskSize,
			ImageType:       vmpb.DiskImageType_DISK_IMAGE_AUTO,
			StorageLocation: vmpb.StorageLocation_STORAGE_CRYPTOHOME_ROOT,
		}, resp); err != nil {
		return "", err
	}

	diskStatus := resp.GetStatus()
	if diskStatus != vmpb.DiskImageStatus_DISK_STATUS_CREATED &&
		diskStatus != vmpb.DiskImageStatus_DISK_STATUS_EXISTS {
		return "", errors.Errorf("could not create disk image: %v", resp.GetFailureReason())
	}

	return resp.GetDiskPath(), nil
}

// SyncTimes runs the SyncVmTimes dbus method in concierge.
func (c *Concierge) SyncTimes(ctx context.Context) error {
	resp := &vmpb.SyncVmTimesResponse{}
	if err := dbusutil.CallProtoMethod(ctx, c.conciergeObj, conciergeInterface+".SyncVmTimes",
		nil, resp); err != nil {
		return err
	}

	failures := resp.GetFailures()
	if failures != 0 {
		return errors.Errorf("could not set %d (out of %d) times: %v", resp.GetFailures(), resp.GetRequests(), resp.GetFailureReason())
	}
	return nil
}

func (c *Concierge) startTerminaVM(ctx context.Context, vm *VM) error {
	// Create the new disk first.
	diskPath, err := c.createDiskImage(ctx)
	if err != nil {
		return err
	}

	tremplin, err := dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      ciceronePath,
		Interface: ciceroneInterface,
		Member:    "TremplinStarted",
	})
	defer tremplin.Close(ctx)

	resp := &vmpb.StartVmResponse{}
	if err = dbusutil.CallProtoMethod(ctx, c.conciergeObj, conciergeInterface+".StartVm",
		&vmpb.StartVmRequest{
			Name:         vm.name,
			StartTermina: true,
			OwnerId:      c.ownerID,
			Disks: []*vmpb.DiskImage{
				&vmpb.DiskImage{
					Path:      diskPath,
					ImageType: vmpb.DiskImageType_DISK_IMAGE_AUTO,
					Writable:  true,
					DoMount:   false,
				},
			},
		}, resp); err != nil {
		return err
	}
	if !resp.GetSuccess() {
		return errors.Errorf("failed to start VM: %s", resp.GetFailureReason())
	}

	testing.ContextLog(ctx, "Waiting for TremplinStarted D-Bus signal")
	sigResult := &cpb.TremplinStartedSignal{}
	select {
	case sig := <-tremplin.Signals:
		if len(sig.Body) == 0 {
			return errors.New("TremplinStarted signal lacked a body")
		}
		buf, ok := sig.Body[0].([]byte)
		if !ok {
			return errors.New("TremplinStarted signal body is not a byte slice")
		}
		if err := proto.Unmarshal(buf, sigResult); err != nil {
			return errors.Wrap(err, "failed unmarshaling TremplinStarted body")
		}
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get TremplinStarted D-Bus signal")
	}

	if sigResult.OwnerId != c.ownerID {
		return errors.Errorf("expected owner id %q, received %q", c.ownerID, sigResult.OwnerId)
	}
	if sigResult.VmName != vm.name {
		return errors.Errorf("expected VM name %q, received %q", vm.name, sigResult.VmName)
	}

	vm.ContextID = resp.VmInfo.Cid

	testing.ContextLogf(ctx, "Started VM %q with CID %d and PID %d", vm.name, resp.VmInfo.Cid, resp.VmInfo.Pid)

	return nil
}

func (c *Concierge) stopVM(ctx context.Context, vm *VM) error {
	resp := &vmpb.StopVmResponse{}
	if err := dbusutil.CallProtoMethod(ctx, vm.Concierge.conciergeObj, conciergeInterface+".StopVm",
		&vmpb.StopVmRequest{
			Name:    vm.name,
			OwnerId: vm.Concierge.ownerID,
		}, resp); err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return errors.Errorf("failed to stop VM: %v", resp.GetFailureReason())
	}

	testing.ContextLogf(ctx, "Shut down VM %q", vm.name)
	return nil
}

// GetOwnerID returns the cryptohome hash for the logged-in user.
func (c *Concierge) GetOwnerID() string {
	return c.ownerID
}
