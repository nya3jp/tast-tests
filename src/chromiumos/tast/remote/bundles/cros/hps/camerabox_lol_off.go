// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxLoLOff,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the HPS can dim the screen within the right time range when LoL is off",
		Data:         []string{hpsutil.PersonPresentPageArchiveFilename},
		Contacts: []string{
			"eunicesun@google.com",
			"mblsha@google.com",
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:camerabox", "group:hps", "hps_perbuild"},
		Timeout:      10 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.HPS()),
		SoftwareDeps: []string{"hps", "chrome", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.hps.HpsService"},
		Vars:         []string{"tablet", "grpcServerPort"},
	})
}

func CameraboxLoLOff(ctx context.Context, s *testing.State) {

	dut := s.DUT()

	// Creating hps context.
	hctx, err := hpsutil.NewHpsContext(ctx, "", hpsutil.DeviceTypeBuiltin, s.OutDir(), dut.Conn())
	if err != nil {
		s.Fatal("Error creating HpsContext: ", err)
	}

	// Connecting to the other tablet that will render the picture.
	ctxForCleanupDisplayChart := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	hostPaths, displayChart, err := utils.SetupDisplay(ctx, s)
	if err != nil {
		s.Fatal("Error setting up display: ", err)
	}
	defer displayChart.Close(ctxForCleanupDisplayChart, s.OutDir())

	displayChart.Display(ctx, hostPaths[utils.ZeroPresence])

	// Connecting to Taeko.
	cleanupCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, time.Minute)
	defer cancel()
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to setup grpc: ", err)
	}
	defer cl.Close(cleanupCtx)

	client := pb.NewHpsServiceClient(cl.Conn)
	req := &pb.StartUIWithCustomScreenPrivacySettingRequest{
		Setting: utils.LockOnLeave,
		Enable:  true,
	}
	// Change the setting to true so that we can get the quickdim delay time.
	if _, err := client.StartUIWithCustomScreenPrivacySetting(hctx.Ctx, req, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to change setting: ", err)
	}

	// Get the delays for the quick dim.
	delayReq := &wrappers.BoolValue{
		Value: true,
	}
	quickDimMetrics, err := client.RetrieveDimMetrics(hctx.Ctx, delayReq)
	if err != nil {
		s.Fatal("Error getting delay settings: ", err)
	}

	req.Enable = false
	if _, err := client.StartUIWithCustomScreenPrivacySetting(hctx.Ctx, req, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to change setting: ", err)
	}

	brightness, err := utils.GetBrightness(hctx.Ctx, dut.Conn())
	if err != nil {
		s.Fatal("Error failed to get brightness: ", err)
	}

	// Render hps-internal page for debugging before waiting for dim.
	if _, err := client.OpenHPSInternalsPage(hctx.Ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error open hps-internals")
	}
	// It should not dim after DimDelay.
	// Poll instead of Sleep in order to fail fast in case screen dims before the delay.
	err = utils.PollForBrightnessChange(ctx, brightness, quickDimMetrics.DimDelay.AsDuration(), dut.Conn())
	if err != nil {
		// If brightness is not changed it's not an error in this case, as we'll check the brightness after polling anyway.
		testing.ContextLog(ctx, err.Error())
	}
	newBrightness, err := utils.GetBrightness(hctx.Ctx, dut.Conn())
	if err != nil {
		s.Fatal("Error when getting brightness: ", err)
	}
	if newBrightness != brightness {
		s.Fatal("Unexpected brightness change: was: ", brightness, ", new: ", newBrightness)
	}

	// It should not lock after quick dim delay.
	err = utils.PollForBrightnessChange(ctx, brightness, quickDimMetrics.ScreenOffDelay.AsDuration(), dut.Conn())
	if err != nil {
		// If brightness is not changed it's not an error in this case, as we'll check the brightness after polling anyway.
		testing.ContextLog(ctx, err.Error())
	}
	newBrightness, err = utils.GetBrightness(hctx.Ctx, dut.Conn())
	if err != nil {
		s.Fatal("Error when getting brightness: ", err)
	}
	if newBrightness != brightness {
		s.Fatal("Unexpected brightness change: was: ", brightness, ", new: ", newBrightness)
	}
}
