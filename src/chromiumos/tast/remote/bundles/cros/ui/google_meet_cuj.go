// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/remote/bundles/cros/ui/conference"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleMeetCUJ,
		Desc:         "Host a Google Meet video conference and do presentation to participants",
		Contacts:     []string{"jane.yang@cienet.com", "xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		ServiceDeps: []string{
			"tast.cros.ui.ConferenceService",
		},
		Params: []testing.Param{
			{
				Name:    "basic_two",
				Timeout: time.Minute * 50,
				Val: conference.TestParameters{
					Tier: "basic",
					Size: conference.TwoRoomSize,
				},
			}, {
				Name:    "basic_small",
				Timeout: time.Minute * 50,
				Val: conference.TestParameters{
					Tier: "basic",
					Size: conference.SmallRoomSize,
				},
			}, {
				Name:    "basic_large",
				Timeout: time.Minute * 50,
				Val: conference.TestParameters{
					Tier: "basic",
					Size: conference.LargeRoomSize,
				},
			}, {
				Name:    "basic_class",
				Timeout: time.Minute * 50,
				Val: conference.TestParameters{
					Tier: "basic",
					Size: conference.ClassRoomSize,
				},
			}, {
				Name:    "plus_large",
				Timeout: time.Minute * 50,
				Val: conference.TestParameters{
					Tier: "plus",
					Size: conference.LargeRoomSize,
				},
			}, {
				Name:    "plus_class",
				Timeout: time.Minute * 50,
				Val: conference.TestParameters{
					Tier: "plus",
					Size: conference.ClassRoomSize,
				},
			}, {
				Name:    "premium_large",
				Timeout: time.Minute * 50,
				Val: conference.TestParameters{
					Tier: "premium",
					Size: conference.LargeRoomSize,
				},
			},
		},
	})
}

func GoogleMeetCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(conference.TestParameters)

	dut := s.DUT()
	c, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to dial to remote dut: ", err)
	}
	defer c.Close(ctx)

	client := pb.NewConferenceServiceClient(c.Conn)
	if _, err := client.RunGoogleMeetScenario(ctx, &pb.MeetScenarioRequest{
		Tier:            param.Tier,
		RoomSize:        int64(param.Size),
		ExtendedDisplay: false,
	}); err != nil {
		s.Fatal("Failed to run Meet Scenario: ", err)
	}
}
