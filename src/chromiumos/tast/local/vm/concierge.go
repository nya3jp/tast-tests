// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"

	"github.com/godbus/dbus"

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
			DiskPath:        testVMName,
			DiskSize:        testDiskSize,
			ImageType:       vmpb.DiskImageType_DISK_IMAGE_RAW,
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

func (c *Concierge) GetOwnerID() string {
	return c.ownerID
}
