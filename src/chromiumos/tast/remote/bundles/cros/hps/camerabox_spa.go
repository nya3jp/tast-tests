// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
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

type spaOnParams struct {
	spaOn bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         CameraboxSPA,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that HPS does not respond when SPA is off",
		Data:         []string{hpsutil.PersonPresentPageArchiveFilename},
		Contacts: []string{
			"eunicesun@google.com",
			"mblsha@google.com",
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:camerabox", "group:hps", "hps_perbuild"},
		Timeout:      6 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.HPS()),
		SoftwareDeps: []string{"hps", "chrome", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.hps.HpsService"},
		Vars:         []string{"tablet", "grpcServerPort"},
		Params: []testing.Param{{
			Name: "off",
			Val: spaOnParams{
				spaOn: false,
			},
		}, {
			Name: "on",
			Val: spaOnParams{
				spaOn: true,
			},
		}},
	})
}

func CameraboxSPA(ctx context.Context, s *testing.State) {
	spaEnabled := s.Param().(spaOnParams)

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
		Setting: utils.SecondPersonAlert,
		Enable:  spaEnabled.spaOn,
	}
	// Change the setting to true so that we can get the quickdim delay time.
	if _, err := client.StartUIWithCustomScreenPrivacySetting(hctx.Ctx, req, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to change setting: ", err)
	}

	// Wait for hpsd to finish starting the HPS peripheral and enabling the feature we requested.
	waitReq := &pb.WaitForHpsRequest{
		WaitForSense:  false,
		WaitForNotify: spaEnabled.spaOn,
	}
	if _, err := client.WaitForHps(ctx, waitReq); err != nil {
		s.Fatal("Failed to wait for HPS to be ready: ", err)
	}

	// Check that HPS is running the expected firmware version.
	if spaEnabled.spaOn {
		runningVersion, err := hpsutil.FetchRunningFirmwareVersion(hctx)
		if err != nil {
			s.Error("Error reading running firmware version: ", err)
		}
		expectedVersion, err := hpsutil.FetchFirmwareVersionFromImage(hctx)
		if err != nil {
			s.Error("Error reading firmware version from image: ", err)
		}
		if runningVersion != expectedVersion {
			s.Errorf("HPS reports running firmware version %v but expected %v", runningVersion, expectedVersion)
		}
	}

	// Render hps-internal page for debugging before waiting for dim.
	if _, err := client.OpenHPSInternalsPage(hctx.Ctx, &empty.Empty{}); err != nil {
		s.Fatal("Error open hps-internals: ", err)
	}

	// Index i is representing the number of people in an image too.
	for key, val := range hostPaths {
		displayChart.Display(ctx, val)
		testing.Sleep(ctx, time.Second*5)
		result, err := client.CheckSPAEyeIcon(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Unexpected error occured: ", err)
		}
		if key == utils.OnePresence || key == utils.ZeroPresence {
			if result.Value {
				s.Fatal("Unexpected snooping alert")
			}
		}
		if key == utils.TwoPresence {
			if !result.Value && spaEnabled.spaOn {
				s.Fatal("No snooping alert")
			}
			if result.Value && !spaEnabled.spaOn {
				s.Fatal("Unexpected snooping alert")
			}
		}
	}
}
