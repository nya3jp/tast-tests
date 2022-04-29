// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/common/hps/hpsutil"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/remote/bundles/cros/hps/utils"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/hps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxLoLOff,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that the HPS can dim the screen within the right time range when LoL is off",
		Data: []string{hpsutil.PersonPresentPageArchiveFilename,
			hpsutil.P2PowerCycleFilename},
		Contacts: []string{
			"eunicesun@google.com",
			"mblsha@google.com",
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:camerabox", "group:hps", "hps_perbuild"},
		Timeout:      6 * time.Minute,
		SoftwareDeps: []string{"hps", "chrome", caps.BuiltinCamera},
		Vars:         []string{"tablet", "grpcServerPort"},
	})
}

func CameraboxLoLOff(ctx context.Context, s *testing.State) {

	d := s.DUT()

	archive := s.DataPath(hpsutil.PersonPresentPageArchiveFilename)
	filePaths, err := utils.UntarImages(ctx, archive)
	if err != nil {
		s.Fatal("Tmp dir creation failed on DUT")
	}
	// Creating hps context.
	hctx, err := hpsutil.NewHpsContext(ctx, "", hpsutil.DeviceTypeBuiltin, s.OutDir(), d.Conn())
	if err != nil {
		s.Fatal("Error creating HpsContext: ", err)
	}

	// Connecting to the other tablet that will render the picture.
	var chartAddr string
	if altAddr, ok := s.Var("tablet"); ok {
		chartAddr = altAddr
	}
	c, hostPaths, err := chart.New(ctx, d, chartAddr, s.OutDir(), filePaths)

	// Scenario 1: display no person page.
	c.Display(ctx, hostPaths[0])

	// Connecting to Taeko.
	cl, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the DUT: ", err)
	}
	defer cl.Close(ctx)

	req := &pb.ChangeSettingsRequest{
		Setting: "Lock on Leave",
		Enable:  false,
	}
	client := pb.NewHpsServiceClient(cl.Conn)
	if _, err := client.StartUIWithCustomScreenPrivacySetting(hctx.Ctx, req, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to change setting: ", err)
	}

	brightness, err := utils.GetBrightness(hctx.Ctx, d.Conn())
	if err != nil {
		s.Fatal("Error failed to get brightness: ", err)
	}
	testing.ContextLog(ctx, "Brightness: ", brightness)

	// It should not dim after 6s.
	testing.Sleep(ctx, utils.QuickDimTime)
	newBrightness, err := utils.GetBrightness(hctx.Ctx, d.Conn())
	if err != nil {
		s.Fatal("Error when getting brightness: ", err)
	}
	if newBrightness != brightness {
		s.Fatal("Unexpected brightness change")
	}

	// It should not lock after 2mins.
	testing.Sleep(ctx, utils.QuickLockTime)
	newBrightness, err = utils.GetBrightness(hctx.Ctx, d.Conn())
	if err != nil {
		s.Fatal("Error when getting brightness: ", err)
	}
	if newBrightness != brightness {
		s.Fatal("Unexpected brightness change")
	}

}
