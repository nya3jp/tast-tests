// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"

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
	conciergeJob = "vm_concierge" // name of the upstart job for concierge
	ciceroneJob  = "vm_cicerone"  // name of the upstart job for cicerone

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
	ciceroneObj  dbus.BusObject
}

// GetRunningConcierge returns a concierge instance without restarting concierge service.
// Returns an error if concierge is not available.
func GetRunningConcierge(ctx context.Context, user string) (*Concierge, error) {
	h, err := cryptohome.UserHash(ctx, user)
	if err != nil {
		return nil, err
	}

	// Try to get a connection to a running concierge instance. If it's not available,
	// returns with an error immediately.
	conn, err := dbusutil.SystemBus()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to system bus")
	}
	if !dbusutil.ServiceOwned(ctx, conn, conciergeName) {
		return nil, errors.Wrapf(err, "%s is not owned", conciergeName)
	}

	concierge := conn.Object(conciergeName, conciergePath)
	cicerone := conn.Object(ciceroneName, ciceronePath)
	return &Concierge{h, concierge, cicerone}, nil
}

// NewConcierge restarts the vm_concierge service, which stops all running VMs.
func NewConcierge(ctx context.Context, user string) (*Concierge, error) {
	h, err := cryptohome.UserHash(ctx, user)
	if err != nil {
		return nil, err
	}

	testing.ContextLogf(ctx, "Restarting %v job", conciergeJob)
	if err = upstart.RestartJob(ctx, conciergeJob); err != nil {
		return nil, errors.Wrapf(err, "%v Upstart job failed", conciergeJob)
	}
	bus, concierge, err := dbusutil.Connect(ctx, conciergeName, conciergePath)
	if err != nil {
		return nil, err
	}
	cicerone := bus.Object(ciceroneName, ciceronePath)

	testing.ContextLogf(ctx, "Restarting %v job", ciceroneJob)
	if err = upstart.RestartJob(ctx, ciceroneJob); err != nil {
		return nil, errors.Wrapf(err, "%v Upstart job failed", ciceroneJob)
	}
	if err = dbusutil.WaitForService(ctx, bus, ciceroneName); err != nil {
		return nil, errors.Wrapf(err, "%v D-Bus service unavailable", ciceroneName)
	}

	return &Concierge{h, concierge, cicerone}, nil
}

// StopConcierge stops the vm_concierge service, which stops all running VMs.
func StopConcierge(ctx context.Context) error {
	testing.ContextLogf(ctx, "Stopping %v job", conciergeJob)
	if err := upstart.StopJob(ctx, conciergeJob); err != nil {
		return errors.Wrapf(err, "%v Upstart job failed to stop", conciergeJob)
	}

	return nil
}

// GetEncodedName returns the encoded version of the user-chosen name which
// concierge uses to identify several pieces of the VM (its stateful image, log
// files, and ssh keys).
func GetEncodedName(name string) string {
	return base64.URLEncoding.WithPadding(base64.StdPadding).EncodeToString([]byte(name))
}

// GetVMDiskInfo returns a VmDiskInfo proto for the given VM via ListVmDisks
func (c *Concierge) GetVMDiskInfo(ctx context.Context, vmName string) (*vmpb.VmDiskInfo, error) {
	resp := &vmpb.ListVmDisksResponse{}
	if err := dbusutil.CallProtoMethod(ctx, c.conciergeObj, conciergeInterface+".ListVmDisks",
		&vmpb.ListVmDisksRequest{
			CryptohomeId: c.ownerID,
			AllLocations: true,
			VmName:       DefaultVMName,
		}, resp); err != nil {
		return nil, err
	}

	if !resp.GetSuccess() {
		return nil, errors.Errorf("could not fetch VM disks info: %v", resp.GetFailureReason())
	}

	for _, diskInfo := range resp.GetImages() {
		if diskInfo.GetName() == vmName {
			return diskInfo, nil
		}
	}
	return nil, errors.Errorf("could not find vm named %v", vmName)
}

// listVMDisksSize returns the size of the named VM through ListVmDisks.
func (c *Concierge) listVMDisksSize(ctx context.Context, vmName string) (size uint64, err error) {
	disk, err := c.GetVMDiskInfo(ctx, vmName)
	if err != nil {
		return 0, err
	}
	return disk.Size, nil
}

func (c *Concierge) createDiskImage(ctx context.Context, diskSize uint64) (diskPath string, err error) {
	resp := &vmpb.CreateDiskImageResponse{}
	if err = dbusutil.CallProtoMethod(ctx, c.conciergeObj, conciergeInterface+".CreateDiskImage",
		&vmpb.CreateDiskImageRequest{
			CryptohomeId:    c.ownerID,
			VmName:          DefaultVMName,
			DiskSize:        diskSize,
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

func (c *Concierge) startTerminaVM(ctx context.Context, vm *VM) (string, error) {
	// Create the new disk first.
	diskPath, err := c.createDiskImage(ctx, vm.targetDiskSize)
	if err != nil {
		return diskPath, err
	}

	tremplin, err := dbusutil.NewSignalWatcherForSystemBus(ctx, dbusutil.MatchSpec{
		Type:      "signal",
		Path:      ciceronePath,
		Interface: ciceroneInterface,
		Member:    "TremplinStarted",
	})
	defer tremplin.Close(ctx)

	// Get the number of online cpus.
	buf, err := ioutil.ReadFile("/sys/devices/system/cpu/online")
	if err != nil {
		return diskPath, errors.Wrap(err, "failed to read number of online cpus")
	}
	cpus := uint32(0)
	for _, s := range strings.Split(string(buf), ",") {
		// First try to see if it is a range of values.
		var first, last uint32
		if _, err := fmt.Sscanf(s, "%v-%v", &first, &last); err == nil {
			cpus += (last - first) + 1
			continue
		}

		// It's not a range of values so try to parse it like a single value.
		var val uint32
		if _, err := fmt.Sscanf(s, "%v", &val); err != nil {
			return diskPath, errors.Wrap(err, "failed to parse online cpu value")
		}

		cpus++
	}

	resp := &vmpb.StartVmResponse{}
	if err = dbusutil.CallProtoMethod(ctx, c.conciergeObj, conciergeInterface+".StartVm",
		&vmpb.StartVmRequest{
			Name:         vm.name,
			StartTermina: true,
			OwnerId:      c.ownerID,
			Disks: []*vmpb.DiskImage{
				{
					Path:      diskPath,
					ImageType: vmpb.DiskImageType_DISK_IMAGE_AUTO,
					Writable:  true,
					DoMount:   false,
				},
			},
			EnableGpu: vm.EnableGPU,
			Cpus:      cpus,
		}, resp); err != nil {
		return diskPath, err
	}
	if !resp.GetSuccess() {
		return diskPath, errors.Errorf("failed to start VM: %s", resp.GetFailureReason())
	}

	testing.ContextLog(ctx, "Waiting for TremplinStarted D-Bus signal")
	sigResult := &cpb.TremplinStartedSignal{}
	select {
	case sig := <-tremplin.Signals:
		if len(sig.Body) == 0 {
			return diskPath, errors.New("TremplinStarted signal lacked a body")
		}
		buf, ok := sig.Body[0].([]byte)
		if !ok {
			return diskPath, errors.New("TremplinStarted signal body is not a byte slice")
		}
		if err := proto.Unmarshal(buf, sigResult); err != nil {
			return diskPath, errors.Wrap(err, "failed unmarshaling TremplinStarted body")
		}
	case <-ctx.Done():
		return diskPath, errors.Wrap(ctx.Err(), "didn't get TremplinStarted D-Bus signal")
	}

	if sigResult.OwnerId != c.ownerID {
		return diskPath, errors.Errorf("expected owner id %q, received %q", c.ownerID, sigResult.OwnerId)
	}
	if sigResult.VmName != vm.name {
		return diskPath, errors.Errorf("expected VM name %q, received %q", vm.name, sigResult.VmName)
	}

	vm.ContextID = resp.VmInfo.Cid
	vm.seneschalHandle = resp.VmInfo.SeneschalServerHandle

	testing.ContextLogf(ctx, "Started VM %q with CID %d and PID %d", vm.name, resp.VmInfo.Cid, resp.VmInfo.Pid)

	return diskPath, nil
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

func (c *Concierge) getVMInfo(ctx context.Context, vm *VM) error {
	resp := &vmpb.GetVmInfoResponse{}
	if err := dbusutil.CallProtoMethod(ctx, vm.Concierge.conciergeObj, conciergeInterface+".GetVmInfo",
		&vmpb.GetVmInfoRequest{
			Name:    vm.name,
			OwnerId: vm.Concierge.ownerID,
		}, resp); err != nil {
		return err
	}

	if !resp.GetSuccess() {
		return errors.New("failed to get VM info")
	}

	vm.ContextID = resp.VmInfo.Cid
	vm.seneschalHandle = resp.VmInfo.SeneschalServerHandle

	return nil
}

// GetOwnerID returns the cryptohome hash for the logged-in user.
func (c *Concierge) GetOwnerID() string {
	return c.ownerID
}
