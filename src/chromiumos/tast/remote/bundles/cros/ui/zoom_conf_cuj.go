// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/ui/conference"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ZoomConfCUJ,
		Desc:         "Host a Zoom video conference and do presentation to participants",
		Contacts:     []string{"jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		ServiceDeps: []string{
			"tast.cros.ui.ZoomService",
		},
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			// Zoom bot server proxy address
			"ui.conference_server",
		},
		Params: []testing.Param{
			{
				Name:    "basic_clamshell_two",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "clamshell",
					Size:       conference.TwoRoomSize,
				},
			},
			{
				Name:    "basic_clamshell_small",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "clamshell",
					Size:       conference.SmallRoomSize,
				},
			},
			{
				Name:    "basic_clamshell_large",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "clamshell",
					Size:       conference.LargeRoomSize,
				},
			},
			{
				Name:    "basic_clamshell_class",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "clamshell",
					Size:       conference.ClassRoomSize,
				},
			},
			{
				Name:    "basic_tablet_two",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "tablet",
					Size:       conference.TwoRoomSize,
				},
			},
			{
				Name:    "basic_tablet_small",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "tablet",
					Size:       conference.SmallRoomSize,
				},
			},
			{
				Name:    "basic_tablet_large",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "tablet",
					Size:       conference.LargeRoomSize,
				},
			},
			{
				Name:    "basic_tablet_class",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "tablet",
					Size:       conference.ClassRoomSize,
				},
			},
			{
				Name:    "plus_clamshell_small",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "plus",
					ScreenMode: "clamshell",
					Size:       conference.SmallRoomSize,
				},
			},
			{
				Name:    "plus_clamshell_class",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "plus",
					ScreenMode: "clamshell",
					Size:       conference.ClassRoomSize,
				},
			},
			{
				Name:    "plus_tablet_small",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "plus",
					ScreenMode: "tablet",
					Size:       conference.SmallRoomSize,
				},
			},
			{
				Name:    "plus_tablet_class",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "plus",
					ScreenMode: "tablet",
					Size:       conference.ClassRoomSize,
				},
			},
			{
				Name:    "premium_clamshell_large",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "premium",
					ScreenMode: "clamshell",
					Size:       conference.LargeRoomSize,
				},
			},
			{
				Name:    "premium_tablet_large",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "premium",
					ScreenMode: "tablet",
					Size:       conference.LargeRoomSize,
				},
			},
		},
	})
}

func ZoomConfCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(conference.TestParameters)

	tabletMode := param.ScreenMode == "tablet"

	dut := s.DUT()
	u1, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to dial to remote dut: ", err)
	}
	defer u1.Close(ctx)

	client := pb.NewZoomServiceClient(u1.Conn)
	conference.Run(ctx, s, client, param.Tier, param.Size, tabletMode)
}
