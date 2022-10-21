// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"
	"math"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc"

	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hps/utils"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/hps"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
		HardwareDeps: hwdep.D(hwdep.HPS()),
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

	// Enable LoL in setting.
	client := pb.NewHpsServiceClient(cl.Conn)
	req := &pb.StartUIWithCustomScreenPrivacySettingRequest{
		Setting: utils.LockOnLeave,
		Enable:  true,
	}
	if _, err := client.StartUIWithCustomScreenPrivacySetting(hctx.Ctx, req, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to change setting: ", err)
	}

	// Wait for hpsd to finish starting the HPS peripheral and enabling the feature we requested.
	waitReq := &pb.WaitForHpsRequest{
		WaitForSense:  true,
		WaitForNotify: false,
	}
	if _, err := client.WaitForHps(ctx, waitReq); err != nil {
		s.Fatal("Failed to wait for HPS to be ready: ", err)
	}

	// Get the delays for the quick dim
	delayReq := &wrappers.BoolValue{
		Value: true,
	}
	quickDimMetrics, err := client.RetrieveDimMetrics(hctx.Ctx, delayReq)
	if err != nil {
		s.Fatal("Error getting delay settings: ", err)
	}

	initialBrightness, err := utils.GetBrightness(hctx.Ctx, dut.Conn())
	if err != nil {
		s.Fatal("Error failed to get brightness: ", err)
	}

	// Render hps-internals page for debugging before waiting for dim.
	if _, err := client.OpenHPSInternalsPage(hctx.Ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error open hps-internals")
	}

	// Expect screen will dim quickly with ZeroPresence.
	if err := utils.PollForDim(ctx, initialBrightness, quickDimMetrics.DimDelay.AsDuration(), false, dut.Conn()); err != nil {
		s.Fatal("Error when polling for brightness: ", err)
	}

	dimmedBrightness, err := utils.GetBrightness(hctx.Ctx, dut.Conn())

	// Expect screen will undim quickly after showing a face.
	startTime := time.Now()
	displayChart.Display(ctx, hostPaths[utils.OnePresence])
	testing.ContextLog(ctx, "Expect screen to undim with one presence")
	if err := expectBrightnessChange(ctx, dimmedBrightness, initialBrightness, startTime, time.Second, dut.Conn()); err != nil {
		s.Error("Expected screen to undim: ", err)
	}

	// Simulate user leaving again
	undimTime := time.Now()
	displayChart.Display(ctx, hostPaths[utils.ZeroPresence])
	testing.ContextLog(ctx, "Expect screen to dim with zero presence")
	if err := expectBrightnessChange(ctx, initialBrightness, dimmedBrightness, undimTime, quickDimMetrics.DimDelay.AsDuration(), dut.Conn()); err != nil {
		s.Error("Expected screen to dim: ", err)
	}

	if err := utils.PollForDim(ctx, initialBrightness, quickDimMetrics.ScreenOffDelay.AsDuration(), true, dut.Conn()); err != nil {
		s.Fatal("Error when polling for brightness: ", err)
	}

	// Not waiting for the lock screen, since it's controlled by "Show lock screen after waking from sleep" setting.
}

func expectBrightnessChange(ctx context.Context, initialBrightness, expectedBrightness float64, startTime time.Time, maxDuration time.Duration, conn *ssh.Conn) error {
	if err := utils.PollForBrightnessChange(ctx, initialBrightness, maxDuration, conn); err != nil {
		return err
	}
	endTime := time.Now()
	newBrightness, err := utils.GetBrightness(ctx, conn)
	if err != nil {
		return err
	}
	duration := endTime.Sub(startTime)
	testing.ContextLog(ctx, "Brightness changed to ", newBrightness, " in ", duration.Seconds(), "s")
	if newBrightness != expectedBrightness {
		return errors.Errorf("Brightness is not what we expected: expected %f, got %f", expectedBrightness, newBrightness)
	}
	if delta := math.Abs(duration.Seconds() - maxDuration.Seconds()); delta > utils.BrightnessChangeTimeoutSlackDuration.Seconds() {
		return errors.Errorf("Duration delta (%f) is greater than allowed slack: %f", delta, duration.Seconds())
	}
	return nil
}
