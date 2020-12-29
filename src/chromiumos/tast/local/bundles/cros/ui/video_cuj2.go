// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type videoCUJParam struct {
	tier cuj.Tier
	app  string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoCUJ2,
		Desc:         "Measures the smoothess of switch between full screen YouTube video and another browser window",
		Contacts:     []string{"xiyuan@chromium.org", "tim.chang@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "loggedInAndKeepState",
		Vars: []string{
			"mode", // Optional. Expecting "tablet" or "clamshell".
			"ui.netflix_username",
			"ui.netflix_password",
		},
		Params: []testing.Param{
			{
				Name:    "basic_youtube_web",
				Timeout: 10 * time.Minute,
				Val: videoCUJParam{
					tier: cuj.Basic,
					app:  videocuj.YoutubeWeb,
				},
			}, {
				Name:    "plus_youtube_web",
				Timeout: 10 * time.Minute,
				Val: videoCUJParam{
					tier: cuj.Plus,
					app:  videocuj.YoutubeWeb,
				},
			}, {
				Name:    "basic_netflix_web",
				Timeout: 10 * time.Minute,
				Val: videoCUJParam{
					tier: cuj.Basic,
					app:  videocuj.NetflixWeb,
				},
			}, {
				Name:    "plus_netflix_web",
				Timeout: 10 * time.Minute,
				Val: videoCUJParam{
					tier: cuj.Plus,
					app:  videocuj.NetflixWeb,
				},
			}, {
				Name:    "basic_youtube_app",
				Timeout: 10 * time.Minute,
				Val: videoCUJParam{
					tier: cuj.Basic,
					app:  videocuj.YoutubeApp,
				},
			}, {
				Name:    "plus_youtube_app",
				Timeout: 10 * time.Minute,
				Val: videoCUJParam{
					tier: cuj.Plus,
					app:  videocuj.YoutubeApp,
				},
			},
		},
	})
}

// VideoCUJ2 ...
func VideoCUJ2(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cuj.FixtureData).Chrome
	a := s.FixtValue().(cuj.FixtureData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	var tabletMode bool
	if mode, ok := s.Var("mode"); ok {
		tabletMode = mode == "tablet"
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
		if err != nil {
			s.Fatalf("Failed to enable tablet mode to %v: %v", tabletMode, err)
		}
		defer cleanup(ctx)
	} else {
		// Use default screen mode of the DUT.
		tabletMode, err = ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get DUT default screen mode: ", err)
		}
	}
	s.Log("Running test with tablet mode: ", tabletMode)

	param := s.Param().(videoCUJParam)
	tier := param.tier
	app := param.app

	videocuj.Run(ctx, s, cr, a, app, tabletMode, tier)
}
