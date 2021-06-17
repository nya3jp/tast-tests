// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/arc"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ADBOverUSB,
		Desc:     "Checks that arc(vm)-adbd job is up and running when adb-over-usb feature available",
		Contacts: []string{"shuanghu@chromium.org", "arc-eng@google.com"},
		HardwareDeps: hwdep.D(
			// Available boards info, please refer to doc https://www.chromium.org/chromium-os/chrome-os-systems-supporting-adb-debugging-over-usb
			hwdep.Model("eve", "atlas", "nocturne", "soraka"),
		),
		SoftwareDeps: []string{"reboot", "chrome", "crossystem", "flashrom"},
		ServiceDeps:  []string{"tast.cros.arc.ADBOverUSBService", "tast.cros.firmware.UtilsService", "tast.cros.firmware.BiosService"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Data:    []string{firmware.ConfigFile},
		Pre:     pre.DevMode(),
		Vars:    []string{"servo"},
		Timeout: 15 * time.Minute,
	})
}

func ADBOverUSB(ctx context.Context, s *testing.State) {
	d := s.DUT()
	// Connect to the gRPC server on the DUT
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	service := arc.NewADBOverUSBServiceClient(cl.Conn)
	enableUDCRequest := arcpb.EnableUDCRequest{
		Enable: true,
	}
	rsp, err := service.SetUDCEnabled(ctx, &enableUDCRequest)
	if err != nil {
		s.Fatal("Failed to enable USB Device Controller on the DUT: ", err)
	}

	if rsp.UDCValueUpdated {
		s.Log("Rebooting")
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot DUT: ", err)
		}

		// Reconnect to the gRPC server after rebooting DUT.
		cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(ctx)

		service = arc.NewADBOverUSBServiceClient(cl.Conn)

		defer func() {
			disableUDCRequest := arcpb.EnableUDCRequest{
				Enable: false,
			}
			if _, err := service.SetUDCEnabled(ctx, &disableUDCRequest); err != nil {
				s.Fatal("Failed to disable USB Device Controller on the DUT: ", err)
			}
			s.Log("Rebooting")
			if err := s.DUT().Reboot(ctx); err != nil {
				s.Fatal("Failed to reboot DUT: ", err)
			}
		}()
	}

	s.Log("Checking arc(vm)-adbd job on DUT")
	if _, err = service.CheckADBDJobStatus(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to check arc(vm)-adbd job on DUT: ", err)
	}
}
