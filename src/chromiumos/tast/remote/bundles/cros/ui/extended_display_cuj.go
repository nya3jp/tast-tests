// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/remote/bundles/cros/ui/conference"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtendedDisplayCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test video entertainment with extended display",
		Contacts:     []string{"vlin@cienet.com", "cienet-development@googlegroups.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		ServiceDeps: []string{
			"tast.cros.ui.ConferenceService",
		},
		Vars: []string{
			"ui.chameleon_addr",         // Only needed when using chameleon board as extended display.
			"ui.chameleon_display_port", // The port connected as extended display. Default is 3.
			"ui.collectTrace",           // Optional. Expecting "enable" or "disable", default is "disable".
		},
		Data: []string{conference.CameraVideo, conference.TraceConfigFile},
		Params: []testing.Param{
			{
				Name:    "premium_meet_large",
				Timeout: 50*time.Minute + conference.CPUIdleTimeout,
				Val: conference.TestParameters{
					// This is a premium test case for extended display CUJ.
					// But this case just calls Google Meet "plus" case, so the given tier
					// is "plus" instead of "premium".
					Tier:     "plus",
					RoomType: conference.LargeRoomSize,
				},
			},
			{
				Name:              "premium_lacros_meet_large",
				Timeout:           50*time.Minute + conference.CPUIdleTimeout,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: conference.TestParameters{
					// This is a premium test case for extended display CUJ.
					// But this case just calls Google Meet "plus" case, so the given tier
					// is "plus" instead of "premium".
					Tier:     "plus",
					RoomType: conference.LargeRoomSize,
					IsLacros: true,
				},
			},
		},
	})
}

// ExtendedDisplayCUJ performs the video chat cuj (google meet) test on extended display.
// Known issues: b:187165216 describes an issue that click event cannot be executed
// on extended display on certain models.
func ExtendedDisplayCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(conference.TestParameters)

	dut := s.DUT()
	c, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to dial to remote dut: ", err)
	}
	defer c.Close(ctx)

	remoteCameraVideoPath, err := conference.PushFileToTmpDir(ctx, s, dut, conference.CameraVideo)
	if err != nil {
		s.Fatal("Failed to push file to DUT's tmp directory: ", err)
	}
	defer dut.Conn().CommandContext(ctx, "rm", remoteCameraVideoPath).Run()

	if chameleonAddr, ok := s.Var("ui.chameleon_addr"); ok {
		// Use chameleon board as extended display. Make sure chameleon is connected.
		che, err := chameleon.New(ctx, chameleonAddr)
		if err != nil {
			s.Fatal("Failed to connect to chameleon board: ", err)
		}
		defer che.Close(ctx)

		portID := 3 // Use default port 3 for display.
		if port, ok := s.Var("ui.chameleon_display_port"); ok {
			portID, err = strconv.Atoi(port)
			if err != nil {
				s.Fatalf("Failed to parse chameleon display port %q: %v", port, err)
			}
		}

		dp, err := che.NewPort(ctx, portID)
		if err != nil {
			s.Fatalf("Failed to create chameleon port %d: %v", portID, err)
		}
		if err := dp.Plug(ctx); err != nil {
			s.Fatal("Failed to plug chameleon port: ", err)
		}
		defer dp.Unplug(ctx)
		// Wait for DUT to detect external display.
		if err := dp.WaitVideoInputStable(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for video input on chameleon board: ", err)
		}
	}

	if collect, ok := s.Var("ui.collectTrace"); ok && collect == "enable" {
		remoteTraceConfigFilePath, err := conference.PushFileToTmpDir(ctx, s, dut, conference.TraceConfigFile)
		if err != nil {
			s.Fatal("Failed to push file to DUT's tmp directory: ", err)
		}
		defer dut.Conn().CommandContext(ctx, "rm", remoteTraceConfigFilePath).Run()
	}
	client := pb.NewConferenceServiceClient(c.Conn)
	if _, err := client.RunGoogleMeetScenario(ctx, &pb.MeetScenarioRequest{
		Tier:            param.Tier,
		RoomType:        int64(param.RoomType),
		ExtendedDisplay: true,
		CameraVideoPath: remoteCameraVideoPath,
		IsLacros:        param.IsLacros,
	}); err != nil {
		s.Fatal("Failed to run Meet Scenario: ", err)
	}
}
