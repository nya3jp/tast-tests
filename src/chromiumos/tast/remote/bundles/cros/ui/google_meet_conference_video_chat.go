// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/ui/conference"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleMeetConferenceVideoChat,
		Desc:         "Using Google Meet host a conference and presentation with participants",
		Contacts:     []string{"jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		ServiceDeps: []string{
			"tast.cros.ui.GoogleMeetService",
			"tast.cros.ui.ZoomService",
		},
		Vars: []string{"ui.cuj_username", "ui.cuj_password", "ui.meet_url"},
		Params: []testing.Param{
			{
				Name:    "basic_clamshell_one_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "clamshell",
					Size:       conference.OneRoomSize,
				},
			},
			{
				Name:    "basic_clamshell_small_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "clamshell",
					Size:       conference.SmallRoomSize,
				},
			},
			{
				Name:    "basic_clamshell_medium_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "clamshell",
					Size:       conference.MediumRoomSize,
				},
			},
			{
				Name:    "basic_clamshell_large_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "clamshell",
					Size:       conference.LargeRoomSize,
				},
			},
			{
				Name:    "basic_tablet_one_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "tablet",
					Size:       conference.OneRoomSize,
				},
			},
			{
				Name:    "basic_tablet_small_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "tablet",
					Size:       conference.SmallRoomSize,
				},
			},
			{
				Name:    "basic_tablet_medium_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "tablet",
					Size:       conference.MediumRoomSize,
				},
			},
			{
				Name:    "basic_tablet_large_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "basic",
					ScreenMode: "tablet",
					Size:       conference.LargeRoomSize,
				},
			},
			{
				Name:    "plus_clamshell_small_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "plus",
					ScreenMode: "clamshell",
					Size:       conference.SmallRoomSize,
				},
			},
			{
				Name:    "plus_clamshell_large_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "plus",
					ScreenMode: "clamshell",
					Size:       conference.LargeRoomSize,
				},
			},
			{
				Name:    "plus_tablet_small_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "plus",
					ScreenMode: "tablet",
					Size:       conference.SmallRoomSize,
				},
			},
			{
				Name:    "plus_tablet_large_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "plus",
					ScreenMode: "tablet",
					Size:       conference.LargeRoomSize,
				},
			},
			{
				Name:    "premium_clamshell_medium_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "premium",
					ScreenMode: "clamshell",
					Size:       conference.MediumRoomSize,
				},
			},
			{
				Name:    "premium_tablet_medium_conference",
				Timeout: time.Minute * 10,
				Val: conference.TestParameters{
					Tier:       "premium",
					ScreenMode: "tablet",
					Size:       conference.MediumRoomSize,
				},
			},
		},
	})
}

func GoogleMeetConferenceVideoChat(ctx context.Context, s *testing.State) {
	param := s.Param().(conference.TestParameters)

	tabletMode := param.ScreenMode == "tablet"
	conference.Run(ctx, s, conference.GoogleMeet, param.Tier, param.Size, tabletMode)
}
