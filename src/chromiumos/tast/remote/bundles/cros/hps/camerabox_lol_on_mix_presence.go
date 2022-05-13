// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc"

	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/hps/utils"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/hps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxLoLOnMixPresence,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the HPS can dim/lock as expected when LOL enabled",
		Data:         []string{hpsutil.PersonPresentPageArchiveFilename},
		Contacts: []string{
			"eunicesun@google.com",
			"mblsha@google.com",
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:camerabox", "group:hps", "hps_perbuild"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"hps", "chrome", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.hps.HpsService"},
		Vars:         []string{"tablet"},
	})
}

func CameraboxLoLOnMixPresence(ctx context.Context, s *testing.State) {
	dut := s.DUT()

	// Creating hps context.
	hctx, err := hpsutil.NewHpsContext(ctx, "", hpsutil.DeviceTypeBuiltin, s.OutDir(), dut.Conn())
	if err != nil {
		s.Fatal("Error creating HpsContext: ", err)
	}

	hostPaths, displayChart, err := utils.SetupDisplay(ctx, s)
	if err != nil {
		s.Fatal("Error setting up display: ", err)
	}

	displayChart.Display(ctx, hostPaths[utils.ZeroPresence])

	// Connecting to Taeko.
	cleanupCtx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to setup grpc: ", err)
	}
	defer cl.Close(cleanupCtx)

	// Wait for Dbus to be available.
	client := pb.NewHpsServiceClient(cl.Conn)
	if _, err := client.WaitForDbus(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to wait for dbus command to be available: ", err)
	}

	// Enable LoL in setting.
	req := &pb.StartUIWithCustomScreenPrivacySettingRequest{
		Setting: utils.LockOnLeave,
		Enable:  true,
	}
	if _, err := client.StartUIWithCustomScreenPrivacySetting(hctx.Ctx, req, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to change setting: ", err)
	}

	// Get the delays for the quick dim
	delayReq := &wrappers.BoolValue{
		Value: true,
	}
	quickDimMetrics, err := client.RetrieveDimMetrics(hctx.Ctx, delayReq)
	if err != nil {
		s.Fatal("Error getting delay settings: ", err)
	}

	brightness, err := utils.GetBrightness(hctx.Ctx, dut.Conn())
	if err != nil {
		s.Fatal("Error failed to get brightness: ", err)
	}

	// Render hps-internals page for debugging before waiting for dim.
	if _, err := client.OpenHPSInternalsPage(hctx.Ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error open hps-internals")
	}

	if err := utils.PollForDim(ctx, brightness, quickDimMetrics.DimDelay.AsDuration(), false, dut.Conn()); err != nil {
		s.Fatal("Error when polling for brightness: ", err)
	}

	displayChart.Display(ctx, hostPaths[utils.OnePresence])
	newBrightness, err := utils.GetBrightness(hctx.Ctx, dut.Conn())
	if err != nil {
		s.Fatal("Error failed to get brightness: ", err)
	}
	testing.Sleep(ctx, time.Second)
	if newBrightness != brightness {
		s.Fatal("Did not undim screen with human presence")
	}

	// Simulate user leaving again
	displayChart.Display(ctx, hostPaths[utils.ZeroPresence])
	if err := utils.PollForDim(ctx, brightness, quickDimMetrics.DimDelay.AsDuration(), false, dut.Conn()); err != nil {
		s.Fatal("Error when polling for brightness: ", err)
	}

	if err := utils.PollForDim(ctx, brightness, quickDimMetrics.ScreenOffDelay.AsDuration(), true, dut.Conn()); err != nil {
		s.Fatal("Error when polling for brightness: ", err)
	}

	utils.WaitWithDelay(ctx, quickDimMetrics.LockDelay.AsDuration())
	if _, err := client.CheckForLockScreen(hctx.Ctx, &empty.Empty{}); err != nil {
		s.Fatal("The system failed to lock: ", err)
	}

}
