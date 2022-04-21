// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shimlessrma contains integration tests for Shimless RMA SWA.
package shimlessrma

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/remote/bundles/cros/shimlessrma/web"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/shimlessrma"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const timeInSecondToWaitForReboot = 10

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisableHWWPWithBatteryDisconnection,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Can complete Shimless RMA successfully. Disable HWWP with Battery Disconnection",
		Contacts: []string{
			"yanghenry@google.com",
			"chromeos-engprod-syd@google.com",
		},
		// TODO: Please check http://shortn/_a81SVAkZE7 to find proper attrs.
		Attr: []string{"group:firmware", "firmware_experimental"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		// Find proper deps from go/tast-deps
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.shimlessrma.AppService"},
		Fixture:      fixture.DevModeGBB,
		// TODO: Figure out how to reboot to Normal Mode
		// Fixture: fixture.NormalMode,
		Timeout: 10 * time.Minute,
	})
}

func DisableHWWPWithBatteryDisconnection(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	d := h.DUT

	cl, client := createShimlessClient(ctx, s, d)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})

	// Set WP as enabled as starting point.
	changeWriteProtectStatus(ctx, s, h, servo.FWWPStateOn)

	web.WelcomePageOperation(ctx, s, client)
	web.ComponentsPageOperation(ctx, s, client)
	web.OwnerPageOperation(ctx, s, client)
	web.WipeDevicePageOperation(ctx, s, client)
	web.WriteProtectPageOperation(ctx, s, client)

	// Disables WP by CCD.
	changeWriteProtectStatus(ctx, s, h, servo.FWWPStateOff)

	// Wait for reboot completed.
	testing.Sleep(ctx, timeInSecondToWaitForReboot*time.Second)

	cl, client = createShimlessClient(ctx, s, d)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})

	web.WriteProtectDisabledPageOperation(ctx, s, client)

	// Bypass firmware installation for now.
	bypassFirmwareInstallation(ctx, s, d)

	// Wait for reboot completed.
	testing.Sleep(ctx, timeInSecondToWaitForReboot*time.Second)

	cl, client = createShimlessClient(ctx, s, d)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})

	web.FirmwareInstallationPageOperation(ctx, s, client)
	web.DeviceInformationPageOperation(ctx, s, client)
	web.DeviceProvisionPageOperation(ctx, s, client)

	// Another reboot after provisioning
	testing.Sleep(ctx, timeInSecondToWaitForReboot*time.Second)

	cl, client = createShimlessClient(ctx, s, d)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})

	web.CalibratePageOperation(ctx, s, client)

	// Enables WP by CCD.
	changeWriteProtectStatus(ctx, s, h, servo.FWWPStateOn)
	web.WriteProtectEnabledPageOperation(ctx, s, client)

	// Wait for reboot completed.
	testing.Sleep(ctx, timeInSecondToWaitForReboot*time.Second)

	cl, client = createShimlessClient(ctx, s, d)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})

	web.FinalizingRepairPageOperation(ctx, s, client)
	web.RepairCompeletedPageOperation(ctx, s, client)
}

func createShimlessClient(ctx context.Context, s *testing.State, d *dut.DUT) (*rpc.Client, pb.AppServiceClient) {
	if err := d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed connect to DUT: ", err)
	}

	// Setup rpc.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	s.Log("Setup rpc successfully")

	request := &pb.NewShimlessRMARequest{
		ManifestKey: s.RequiredVar("ui.signinProfileTestExtensionManifestKey"),
		Reconnect:   false,
	}
	client := pb.NewAppServiceClient(cl.Conn)
	if _, err := client.NewShimlessRMA(ctx, request, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	return cl, client
}

func changeWriteProtectStatus(ctx context.Context, s *testing.State, h *firmware.Helper, status servo.FWWPStateValue) {
	if err := h.Servo.SetFWWPState(ctx, status); err != nil {
		s.Fatal("Failed to disable write protect: ", err)
	}
}

func bypassFirmwareInstallation(ctx context.Context, s *testing.State, d *dut.DUT) {
	// Add "firmware_updated":true to state file.
	_, err := d.Conn().CommandContext(ctx, "sed", "-i", fmt.Sprintf("s/%s/%s/g", "}$", ",\"firmware_updated\":true}"), "/mnt/stateful_partition/unencrypted/rma-data/state").Output()
	if err != nil {
		s.Fatal("Failed to update state file: ", err)
	}
	// Wait for state file updated.
	s.Log("Sleep for 3 seconds after bypass firmware installation")
	testing.Sleep(ctx, timeInSecondToWaitForReboot*time.Second)

	s.Log("Restart dut after firmware installed")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
}
