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

type numPresenceParams struct {
	numOfPerson string
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
		SoftwareDeps: []string{"hps", "chrome", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.hps.HpsService"},
		Vars:         []string{"tablet"},
		Params: []testing.Param{{
			Name: "no_presence",
			Val: numPresenceParams{
				numOfPerson: utils.ZeroPresence,
			},
		}, {
			Name: "one_presence",
			Val: numPresenceParams{
				numOfPerson: utils.OnePresence,
			},
		}},
	})
}

func CameraboxLoLOn(ctx context.Context, s *testing.State) {
	presenceNo := s.Param().(numPresenceParams)

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

	displayChart.Display(ctx, hostPaths[presenceNo.numOfPerson])

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

	// Get the delays for the quick dim.
	delayReq := &wrappers.BoolValue{
		Value: presenceNo.numOfPerson == utils.ZeroPresence,
	}
	quickDimMetrics, err := client.RetrieveDimMetrics(hctx.Ctx, delayReq)
	dimDelay, screenOffDelay, lockDelay := delayForPresence(presenceNo.numOfPerson, quickDimMetrics)
	if err != nil {
		s.Fatal("Error getting delay settings: ", err)
	}

	brightness, err := utils.GetBrightness(hctx.Ctx, dut.Conn())
	if err != nil {
		s.Fatal("Error failed to get brightness: ", err)
	}

	// Render hps-internal page for debugging before waiting for dim.
	if _, err := client.OpenHPSInternalsPage(hctx.Ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error open hps-internals")
	}

	// It takes around 10s for hps sensor to start having proper results.
	testing.Sleep(ctx, time.Second*10)

	if err := utils.PollForDim(ctx, brightness, dimDelay, false, dut.Conn()); err != nil {
		s.Fatal("Error when polling for brightness: ", err)
	}

	if err := utils.PollForDim(ctx, brightness, screenOffDelay, true, dut.Conn()); err != nil {
		s.Fatal("Error when polling for brightness: ", err)
	}

	utils.WaitWithDelay(ctx, lockDelay)
	if _, err := client.CheckForLockScreen(hctx.Ctx, &empty.Empty{}); err != nil {
		s.Fatal("The system failed to lock: ", err)
	}

}

// delayForPresence returns the expected dim time depending on the human presence.
func delayForPresence(numPresence string, dimSettings *pb.RetrieveDimMetricsResponse) (time.Duration, time.Duration, time.Duration) {
	if numPresence == utils.ZeroPresence {
		return dimSettings.DimDelay.AsDuration(), dimSettings.ScreenOffDelay.AsDuration(), dimSettings.LockDelay.AsDuration()
	}
	// When a human is present, the screen off delay is 30s from dim, and screen lock is 1min from screen off.
	return dimSettings.DimDelay.AsDuration() * 2, 30 * time.Second, time.Minute
}
