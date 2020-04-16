// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/firmware"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/lsbrelease"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			fwpb.RegisterUtilsServiceServer(srv, &UtilsService{s: s})
		},
	})
}

// UtilsService implements tast.cros.firmware.UtilsService.
type UtilsService struct {
	s *testing.ServiceState
}

// Platform gets the name of the DUT platform (coral, samus, drallion, etc).
func (*UtilsService) Platform(ctx context.Context, req *empty.Empty) (*fwpb.PlatformResponse, error) {
	lsbContents, err := lsbrelease.Load()
	if err != nil {
		return nil, errors.Wrap(err, "loading lsbrelease contents")
	}
	p, ok := lsbContents[lsbrelease.Board]
	if !ok {
		return nil, errors.Errorf("failed to find %s in lsbrelease contents", lsbrelease.Board)
	}
	return &fwpb.PlatformResponse{Platform: p}, nil
}

// CurrentBootMode determines the DUT's current firmware boot mode.
func (*UtilsService) CurrentBootMode(ctx context.Context, req *empty.Empty) (*fwpb.CurrentBootModeResponse, error) {
	csValsByMode := map[fwpb.BootMode](map[string]string){
		fwpb.BootMode_BOOT_MODE_NORMAL:   {"devsw_boot": "0", "mainfw_type": "normal"},
		fwpb.BootMode_BOOT_MODE_DEV:      {"devsw_boot": "1", "mainfw_type": "developer"},
		fwpb.BootMode_BOOT_MODE_RECOVERY: {"mainfw_type": "recovery"},
	}
	for bootMode, csVals := range csValsByMode {
		if firmware.CheckCrossystemValues(ctx, csVals) {
			return &fwpb.CurrentBootModeResponse{BootMode: bootMode}, nil
		}
	}
	return nil, errors.New("did not match any known boot mode")
}

// BlockingSync syncs the root device and internal device.
func (*UtilsService) BlockingSync(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	// The double calls to sync fakes a blocking call
	// since the first call returns before the flush is complete,
	// but the second will wait for the first to finish.
	for i := 0; i < 2; i++ {
		if err := testexec.CommandContext(ctx, "sync").Run(testexec.DumpLogOnError); err != nil {
			return nil, errors.Wrapf(err, "sending sync command #%d to DUT", i+1)
		}
	}

	// Find the root device.
	rootDevice, err := firmware.RootDevice(ctx)
	if err != nil {
		return nil, err
	}
	devices := []string{rootDevice}

	// If booted from removable media, sync the internal device too.
	isRemovable, err := firmware.BootDeviceRemovable(ctx)
	if err != nil {
		return nil, err
	}
	if isRemovable {
		internalDevice, err := firmware.InternalDevice(ctx)
		if err != nil {
			return nil, err
		}
		devices = append(devices, internalDevice)
	}

	// sync only sends SYNCHRONIZE_CACHE but doesn't check the status.
	// This function will perform a device-specific sync command.
	for _, device := range devices {
		if strings.Contains(device, "mmcblk") {
			// For mmc devices, use `mmc status get` command to send an
			// empty command to wait for the disk to be available again.
			if err := testexec.CommandContext(ctx, "mmc", "status", "get", device).Run(testexec.DumpLogOnError); err != nil {
				return nil, errors.Wrapf(err, "sending mmc command to device %s", device)
			}
		} else if strings.Contains(device, "nvme") {
			// For NVMe devices, use `nvme flush` command to commit data
			// and metadata to non-volatile media.
			// Get a list of NVMe namespaces, and flush them individually.
			// The output is assumed to be in the following format:
			// [ 0]:0x1
			// [ 1]:0x2
			namespaces, err := testexec.CommandContext(ctx, "nvme", "list-ns", device).Output(testexec.DumpLogOnError)
			if err != nil {
				return nil, errors.Wrapf(err, "listing namespaces for device %s", device)
			}
			if len(namespaces) == 0 {
				return nil, errors.Errorf("Listing namespaces for device %s returned no output", device)
			}
			for _, namespace := range namespaces {
				ns := strings.Split(string(namespace), ":")[1]
				if err := testexec.CommandContext(ctx, "nvme", "flush", device, "-n", ns).Run(testexec.DumpLogOnError); err != nil {
					return nil, errors.Wrapf(err, "flushing namespace %s on device %s", ns, device)
				}
			}
		} else {
			// For other devices, hdparm sends TUR to check if
			// a device is ready for transfer operation.
			if err := testexec.CommandContext(ctx, "hdparm", "-f", device).Run(testexec.DumpLogOnError); err != nil {
				return nil, errors.Wrapf(err, "sending hdparm command to device %s", device)
			}
		}
	}
	return &empty.Empty{}, nil
}
