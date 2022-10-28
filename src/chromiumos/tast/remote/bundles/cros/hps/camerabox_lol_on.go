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
	"chromiumos/tast/remote/bundles/cros/hps/utils"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/hps"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParamForLoLOn struct {
	numOfPerson         string
	usingLatestFirmware bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxLoLOn,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the HPS can dim/lock as expected when LOL enabled",
		Data:         []string{hpsutil.PersonPresentPageArchiveFilename},
		Contacts: []string{
			"eunicesun@google.com",
			"mblsha@google.com",
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:camerabox", "group:hps", "hps_perbuild"},
		Timeout:      50 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.HPS()),
		SoftwareDeps: []string{"hps", "chrome", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.hps.HpsService"},
		Vars:         []string{"tablet"},
		Params: []testing.Param{
			{
				Name: "no_presence",
				Val: testParamForLoLOn{
					numOfPerson:         utils.ZeroPresence,
					usingLatestFirmware: false,
				},
			},
			{
				Name: "one_presence",
				Val: testParamForLoLOn{
					numOfPerson:         utils.OnePresence,
					usingLatestFirmware: false,
				},
			},
			{
				Name: "no_presence_latestfw",
				Val: testParamForLoLOn{
					numOfPerson:         utils.ZeroPresence,
					usingLatestFirmware: true,
				},
				Fixture: "hpsdUsingLatestFirmware",
			},
			{
				Name: "one_presence_latestfw",
				Val: testParamForLoLOn{
					numOfPerson:         utils.OnePresence,
					usingLatestFirmware: true,
				},
				Fixture: "hpsdUsingLatestFirmware",
			},
		},
	})
}

func CameraboxLoLOn(ctx context.Context, s *testing.State) {
	param := s.Param().(testParamForLoLOn)

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

	displayChart.Display(ctx, hostPaths[param.numOfPerson])

	// Connecting to Taeko.
	cleanupCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, time.Minute)
	defer cancel()
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to setup grpc: ", err)
	}
	defer cl.Close(cleanupCtx)

	startTime := time.Now()

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

	// Check that HPS is running the expected firmware version.
	runningVersion, err := hpsutil.FetchRunningFirmwareVersion(hctx)
	if err != nil {
		s.Error("Error reading running firmware version: ", err)
	}
	firmwarePath := hpsutil.FirmwarePath
	if param.usingLatestFirmware {
		firmwarePath = hpsutil.LatestFirmwarePath
	}
	expectedVersion, err := hpsutil.FetchFirmwareVersionFromImage(hctx, firmwarePath)
	if err != nil {
		s.Error("Error reading firmware version from image: ", err)
	}
	if runningVersion != expectedVersion {
		s.Errorf("HPS reports running firmware version %v but expected %v", runningVersion, expectedVersion)
	}

	// When showing ZeroPresence expect that quick dim will happen.
	quickDimExpectedReq := &wrappers.BoolValue{
		Value: param.numOfPerson == utils.ZeroPresence,
	}
	quickDimMetrics, err := client.RetrieveDimMetrics(hctx.Ctx, quickDimExpectedReq)
	dimDelay, screenOffDelay := delayForPresence(param.numOfPerson, quickDimMetrics)
	if err != nil {
		s.Fatal("Error getting delay settings: ", err)
	}

	// If we're expecting quick dim because no user is present, we need to
	// start counting the time from here because this is when powerd has
	// begun receiving a filtered presence signal from hpsd.
	if param.numOfPerson == utils.ZeroPresence {
		startTime = time.Now()
	}

	initialBrightness, err := utils.GetBrightness(hctx.Ctx, dut.Conn())
	if err != nil {
		s.Fatal("Error failed to get brightness: ", err)
	}

	// Render hps-internal page for debugging before waiting for dim.
	if _, err := client.OpenHPSInternalsPage(hctx.Ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error open hps-internals")
	}

	testing.ContextLog(ctx, "Waiting for screen to dim")
	if err := utils.PollForDim(ctx, initialBrightness, dimDelay, false, dut.Conn()); err != nil {
		s.Error("Error when polling for brightness: ", err)
	}

	dimTime := time.Now()
	err = utils.EnsureHpsSenseSignal(ctx, client, param.numOfPerson != utils.ZeroPresence)
	if err != nil {
		s.Error("Unexpected HPS Signal after dimming: ", err)
	}

	testing.ContextLog(ctx, "Waiting for screen to turn off")
	if err := utils.PollForDim(ctx, initialBrightness, screenOffDelay, true, dut.Conn()); err != nil {
		s.Error("Error when polling for brightness: ", err)
	}

	screenOffTime := time.Now()
	err = utils.EnsureHpsSenseSignal(ctx, client, param.numOfPerson != utils.ZeroPresence)
	if err != nil {
		s.Error("Unexpected HPS Signal after turning off screen: ", err)
	}

	// Not waiting for the lock screen, since it's controlled by "Show lock screen after waking from sleep" setting.

	dimDuration := dimTime.Sub(startTime)
	screenOffDuration := screenOffTime.Sub(dimTime)
	testing.ContextLog(ctx, "(expected) dimDelay: ", dimDelay.Seconds())
	testing.ContextLog(ctx, "(actual)   dimDuration: ", dimDuration.Seconds())
	testing.ContextLog(ctx, "(expected) screenOffDelay: ", screenOffDelay.Seconds())
	testing.ContextLog(ctx, "(actual)   screenOffDuration: ", screenOffDuration.Seconds())

	dimDelta := math.Abs(dimDuration.Seconds() - dimDelay.Seconds())
	if dimDelta > utils.BrightnessChangeTimeoutSlackDuration.Seconds() {
		s.Errorf("Dim duration delta (%f) is greater than allowed slack: %f", dimDelta, utils.BrightnessChangeTimeoutSlackDuration.Seconds())
	}
	screenOffDelta := math.Abs(screenOffDuration.Seconds() - screenOffDelay.Seconds())
	if screenOffDelta > utils.BrightnessChangeTimeoutSlackDuration.Seconds() {
		s.Errorf("Screen off duration delta (%f) is greater than allowed slack: %f", screenOffDelta, utils.BrightnessChangeTimeoutSlackDuration.Seconds())
	}
}

// delayForPresence returns the expected dim time depending on the human presence.
func delayForPresence(numPresence string, dimSettings *pb.RetrieveDimMetricsResponse) (time.Duration, time.Duration) {
	if numPresence == utils.ZeroPresence {
		return dimSettings.DimDelay.AsDuration(), dimSettings.ScreenOffDelay.AsDuration()
	}
	// http://cs/chromeos_public/src/platform2/power_manager/powerd/policy/state_controller.cc
	// StateController::UpdateState() will prevent dimming the screen the first time if the HPS is positive,
	// effectively extending the delay by 2x.
	return 2 * dimSettings.DimDelay.AsDuration(), dimSettings.ScreenOffDelay.AsDuration()
}
