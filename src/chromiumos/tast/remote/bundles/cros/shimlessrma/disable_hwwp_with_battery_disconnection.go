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
	"chromiumos/tast/remote/bundles/cros/shimlessrma/rmaweb"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/shimlessrma"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const waitForRebootStart = 10 * time.Second

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisableHWWPWithBatteryDisconnection,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Can complete Shimless RMA successfully. Disable HWWP with Battery Disconnection",
		Contacts: []string{
			"yanghenry@google.com",
			"chromeos-engprod-syd@google.com",
		},
		Attr: []string{"group:shimless_rma", "shimless_rma_experimental"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		// Find proper deps from go/tast-deps
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.shimlessrma.AppService"},
		Fixture:      fixture.NormalMode,
		Timeout:      10 * time.Minute,
	})
}

func DisableHWWPWithBatteryDisconnection(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	firmwareHelper := s.FixtValue().(*fixture.Value).Helper
	if err := firmwareHelper.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if val, err := firmwareHelper.Servo.GetString(ctx, servo.GSCCCDLevel); err != nil {
		s.Fatal("Failed to get gsc_ccd_level: ", err)
	} else if val != servo.Open {
		s.Logf("CCD is not open, got %q. Attempting to unlock", val)
		if err := firmwareHelper.Servo.SetString(ctx, servo.CR50Testlab, servo.Open); err != nil {
			s.Fatal("Failed to unlock CCD: ", err)
		}
	}

	dut := firmwareHelper.DUT

	cl, client := createShimlessClient(ctx, s, dut, firmwareHelper, false)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})

	// Set WP as enabled as starting point.
	changeWriteProtectStatus(ctx, s, firmwareHelper, servo.FWWPStateOn)
	uiHelper := rmaweb.NewUIHelper(client)

	if err := uiHelper.WelcomePageOperation(ctx); err != nil {
		s.Fatal("Fail to operate Welcome page: ", err)
	}
	if err := uiHelper.ComponentsPageOperation(ctx); err != nil {
		s.Fatal("Fail to operate Components page: ", err)
	}
	if err := uiHelper.OwnerPageOperation(ctx); err != nil {
		s.Fatal("Fail to operate Owner page: ", err)
	}
	if err := uiHelper.WipeDevicePageOperation(ctx); err != nil {
		s.Fatal("Fail to operate Wipe Device page: ", err)
	}
	if err := uiHelper.WriteProtectPageOperation(ctx); err != nil {
		s.Fatal("Fail to operate Choosing Write Protect page: ", err)
	}

	if err := firmwareHelper.Servo.RunCR50Command(ctx, "bpforce disconnect atboot"); err != nil {
		s.Fatal("Fail to disconnect battery: ", err)
	}
	// Disables WP by CCD.
	changeWriteProtectStatus(ctx, s, firmwareHelper, servo.FWWPStateOff)
	// Wait for reboot start.
	testing.Sleep(ctx, waitForRebootStart)

	cl, client = createShimlessClient(ctx, s, dut, firmwareHelper, true)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})

	uiHelper = rmaweb.NewUIHelper(client)
	if err := uiHelper.WriteProtectDisabledPageOperation(ctx); err != nil {
		s.Fatal("Fail to operate write protect disabled page: ", err)
	}
	// Bypass firmware installation for now.
	bypassFirmwareInstallation(ctx, s, dut, client)

	// Wait for reboot start.
	testing.Sleep(ctx, waitForRebootStart)

	cl, client = createShimlessClient(ctx, s, dut, firmwareHelper, true)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})

	uiHelper = rmaweb.NewUIHelper(client)
	if err := uiHelper.FirmwareInstallationPageOperation(ctx); err != nil {
		s.Fatal("Fail to operate Firmware Installation page: ", err)
	}
	if err := uiHelper.DeviceInformationPageOperation(ctx); err != nil {
		s.Fatal("Fail to operate Device Information page: ", err)
	}
	if err := uiHelper.DeviceProvisionPageOperation(ctx); err != nil {
		s.Fatal("Fail to operate Device Provision page: ", err)
	}

	// Another reboot after provisioning
	testing.Sleep(ctx, waitForRebootStart)

	cl, client = createShimlessClient(ctx, s, dut, firmwareHelper, true)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})

	uiHelper = rmaweb.NewUIHelper(client)

	if err := firmwareHelper.Servo.RunCR50Command(ctx, "bpforce follow_batt_pres atboot"); err != nil {
		s.Fatal("Failed to disconnect battery: ", err)
	}
	changeWriteProtectStatus(ctx, s, firmwareHelper, servo.FWWPStateOn)

	// TODO: I comment out the following code due to a bug in Shimless RMA.
	// I will add it back after Shimless RMA fix it.
	// Bug link: b:231906070
	/**
	web.WriteProtectEnabledPageOperation(ctx, s, client)
	s.Log("WriteProtectEnabledPageOperation")

	// Wait for reboot start.
	testing.Sleep(ctx, waitForRebootStart)

	cl, client = createShimlessClient(ctx, s, dut, firmwareHelper, true)
	defer cl.Close(cleanupCtx)
	defer client.CloseShimlessRMA(cleanupCtx, &empty.Empty{})
	s.Log("Init Shimless RMA successfully after enable CCD")
	*/

	if err := uiHelper.FinalizingRepairPageOperation(ctx); err != nil {
		s.Fatal("Fail to operate Finalizing Repair page: ", err)
	}
	if err := uiHelper.RepairCompeletedPageOperation(ctx); err != nil {
		s.Fatal("Fail to operate Repair Compeleted page: ", err)
	}
}

func createShimlessClient(ctx context.Context, s *testing.State, dut *dut.DUT, firmwareHelper *firmware.Helper, reconnect bool) (*rpc.Client, pb.AppServiceClient) {
	if err := firmwareHelper.WaitConnect(ctx); err != nil {
		s.Fatal("Failed connect to DUT: ", err)
	}

	// Setup rpc.
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	request := &pb.NewShimlessRMARequest{
		ManifestKey: s.RequiredVar("ui.signinProfileTestExtensionManifestKey"),
		Reconnect:   reconnect,
	}
	client := pb.NewAppServiceClient(cl.Conn)
	if _, err := client.NewShimlessRMA(ctx, request, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	return cl, client
}

func changeWriteProtectStatus(ctx context.Context, s *testing.State, firmwareHelper *firmware.Helper, status servo.FWWPStateValue) {
	if err := firmwareHelper.Servo.SetFWWPState(ctx, status); err != nil {
		s.Fatal("Failed to change write protect: ", err)
	}
}

func bypassFirmwareInstallation(ctx context.Context, s *testing.State, dut *dut.DUT, client pb.AppServiceClient) {
	// This sleep is important since we need to wait for RMAD to update state file completed.
	testing.Sleep(ctx, 3*time.Second)
	// Add "firmware_updated":true to state file.
	_, err := dut.Conn().CommandContext(ctx, "sed", "-i", fmt.Sprintf("s/%s/%s/g", ".$", ",\"firmware_updated\":true}"), "/mnt/stateful_partition/unencrypted/rma-data/state").Output()
	if err != nil {
		s.Fatal("Failed to update state file to skip firmware installtion: ", err)
	}

	s.Log("Restart dut after firmware installed")
	if err := dut.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
}
