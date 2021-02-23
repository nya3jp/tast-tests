// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
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
		Contacts:     []string{"vlin@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		ServiceDeps: []string{
			"tast.cros.ui.ConferenceService",
		},
		Vars: []string{
			"mode", // Not required. Use "tablet" or "clamshell" to force the mode.
			"ui.cuj_username",
			"ui.cuj_password",
			"ui.meet_url",
			"chameleon",
		},
		Params: []testing.Param{
			{
				Name:    "premium_meet_large",
				Timeout: 10 * time.Minute,
				Val: conference.TestParameters{
					Tier: "plus",
					Size: conference.LargeRoomSize,
				},
			},
		},
	})
}

func ExtendedDisplayCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(conference.TestParameters)

	dut := s.DUT()
	c, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to dial to remote dut: ", err)
	}
	defer c.Close(ctx)

	chameleonAddr := s.RequiredVar("chameleon")
	che, err := chameleon.New(ctx, chameleonAddr)
	if err != nil {
		s.Fatal("Failed to connect to chameleon board: ", err)
	}
	che.Plug(ctx, 3)
	defer che.Unplug(ctx, 3)
	// Wait DUT detect external display
	if err := che.WaitVideoInputStable(ctx, 3, 10*time.Second); err != nil {
		s.Fatal("Failed to plug external display: ", err)
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
