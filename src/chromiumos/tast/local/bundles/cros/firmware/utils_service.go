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

// CheckBootMode wraps a call to the local firmware support package.
func (*UtilsService) CheckBootMode(ctx context.Context, req *fwpb.CheckBootModeRequest) (*fwpb.CheckBootModeResponse, error) {
	var mode firmware.BootMode
	switch req.BootMode {
	case fwpb.BootMode_BOOT_MODE_UNSPECIFIED:
		return nil, errors.New("cannot check unspecified boot mode")
	case fwpb.BootMode_BOOT_MODE_NORMAL:
		mode = firmware.BootModeNormal
	case fwpb.BootMode_BOOT_MODE_DEV:
		mode = firmware.BootModeDev
	case fwpb.BootMode_BOOT_MODE_RECOVERY:
		mode = firmware.BootModeRecovery
	default:
		return nil, errors.Errorf("did not recognize boot mode %v", req.BootMode)
	}
	verified, err := firmware.CheckBootMode(ctx, mode)
	if err != nil {
		return nil, err
	}
	return &fwpb.CheckBootModeResponse{Verified: verified}, nil
}

// BlockingSync syncs the root device and internal device.
func (*UtilsService) BlockingSync(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	var err error
	var cmd *testexec.Cmd
	var devices []string

	// The double calls to sync fakes a blocking call
	// since the first call returns before the flush is complete,
	// but the second will wait for the first to finish.
	for i := 0; i < 2; i++ {
		cmd = testexec.CommandContext(ctx, "sync")
		err = cmd.Run(testexec.DumpLogOnError)
		if err != nil {
			return nil, errors.Wrapf(err, "sending sync command #%d to DUT", i+1)
		}
	}

	// Find the root device.
	rootDevice, err := firmware.GetRootDevice(ctx)
	if err != nil {
		return nil, err
	}
	devices = append(devices, rootDevice)

	// If booted from removable media, sync the internal device too.
	isRemovable, err := firmware.IsBootDeviceRemovable(ctx)
	if err != nil {
		return nil, err
	} else if isRemovable {
		internalDevice, err := firmware.GetInternalDevice(ctx)
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
			cmd = testexec.CommandContext(ctx, "mmc", "status", "get", device)
			err = cmd.Run(testexec.DumpLogOnError)
			if err != nil {
				return nil, errors.Wrapf(err, "sending mmc command to device %s", device)
			}
		} else if strings.Contains(device, "nvme") {
			// For NVMe devices, use `nvme flush` command to commit data
			// and metadata to non-volatile media.
			// Get a list of NVMe namespaces, and flush them individually.
			// The output is assumed to be in the following format:
			// [ 0]:0x1
			// [ 1]:0x2
			cmd = testexec.CommandContext(ctx, "nvme", "list-ns", device)
			namespaces, err := cmd.Output(testexec.DumpLogOnError)
			if err != nil {
				return nil, errors.Wrapf(err, "listing namespaces for device %s", device)
			} else if len(namespaces) == 0 {
				return nil, errors.Errorf("Listing namespaces for device %s returned no output", device)
			}
			for _, namespace := range namespaces {
				ns := strings.Split(string(namespace), ":")[1]
				cmd = testexec.CommandContext(ctx, "nvme", "flush", device, "-n", ns)
				if err = cmd.Run(testexec.DumpLogOnError); err != nil {
					return nil, errors.Wrapf(err, "flushing namespace %s on device %s", ns, device)
				}
			}
		} else {
			// For other devices, hdparm sends TUR to check if
			// a device is ready for transfer operation.
			cmd = testexec.CommandContext(ctx, "hdparm", "-f", device)
			if err = cmd.Run(testexec.DumpLogOnError); err != nil {
				return nil, errors.Wrapf(err, "sending hdparm command to device %s", device)
			}
		}
	}
	return &empty.Empty{}, nil
}
