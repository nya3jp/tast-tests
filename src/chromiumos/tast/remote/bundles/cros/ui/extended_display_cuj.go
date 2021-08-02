// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Desc:         "Test video entertainment with extended display",
		Contacts:     []string{"vlin@cienet.com", "cienet-development@googlegroups.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		ServiceDeps: []string{
			"tast.cros.ui.ConferenceService",
		},
		Vars: []string{
			"ui.chameleon_addr",         // Only needed when using chameleon board as extended display.
			"ui.chameleon_display_port", // The port connected as extended display. Default is 3.

		},
		Params: []testing.Param{
			{
				Name:    "premium_meet_large",
				Timeout: 50 * time.Minute,
				Val: conference.TestParameters{
					// This is a premium test case for extended display CUJ.
					// But this case just calls Google Meet "plus" case, so the given tier
					// is "plus" instead of "premium".
					Tier: "plus",
					Size: conference.LargeRoomSize,
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
	c, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to dial to remote dut: ", err)
	}
	defer c.Close(ctx)

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

	client := pb.NewConferenceServiceClient(c.Conn)

	if _, err := client.RunGoogleMeetScenario(ctx, &pb.MeetScenarioRequest{
		Tier:            param.Tier,
		RoomSize:        int64(param.Size),
		ExtendedDisplay: true,
	}); err != nil {
		s.Fatal("Failed to run Meet Scenario: ", err)
	}
}
