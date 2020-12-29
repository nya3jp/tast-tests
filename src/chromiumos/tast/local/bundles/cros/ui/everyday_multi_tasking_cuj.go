// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	et "chromiumos/tast/local/bundles/cros/ui/everydaymultitaskingcuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type multiTaskingParam struct {
	tire     cuj.Tier
	appName  string
	enableBT bool // enable the bluetooth or not
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EverydayMultiTaskingCUJ,
		Desc:         "Measures the performance of everyday multi-tasking CUJ",
		Contacts:     []string{"xliu@cienet.com", "jane.yang@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars: []string{
			"mute",             // Optional. Mute the DUT during the test.
			"mode",             // Optional. Expecting "tablet" or "clamshell".
			"ui.cuj_username",  // Used to select the account to do login to spotify.
			"ui.bt_devicename", // Required for Bluetooth subtests.
		},
		Fixture: "loggedInAndKeepState",
		Data:    []string{"cca_ui.js"},
		Params: []testing.Param{
			{
				Name:    "basic_ytmusic",
				Timeout: 10 * time.Minute,
				Val: multiTaskingParam{
					tire:     cuj.Basic,
					appName:  et.YoutubeMusic,
					enableBT: false,
				},
			}, {
				Name:    "basic_ytmusic_bluetooth",
				Timeout: 10 * time.Minute,
				Val: multiTaskingParam{
					tire:     cuj.Basic,
					appName:  et.YoutubeMusic,
					enableBT: true,
				},
			}, {
				Name:    "basic_spotify_bluetooth",
				Timeout: 10 * time.Minute,
				Val: multiTaskingParam{
					tire:     cuj.Basic,
					appName:  et.Spotify,
					enableBT: true,
				},
			}, {
				Name:    "plus_ytmusic",
				Timeout: 15 * time.Minute,
				Val: multiTaskingParam{
					tire:     cuj.Plus,
					appName:  et.YoutubeMusic,
					enableBT: false,
				},
			}, {
				Name:    "plus_ytmusic_bluetooth",
				Timeout: 15 * time.Minute,
				Val: multiTaskingParam{
					tire:     cuj.Plus,
					appName:  et.YoutubeMusic,
					enableBT: true,
				},
			}, {
				Name:    "plus_spotify_bluetooth",
				Timeout: 15 * time.Minute,
				Val: multiTaskingParam{
					tire:     cuj.Plus,
					appName:  et.Spotify,
					enableBT: true,
				},
			},
		},
	})
}

func EverydayMultiTaskingCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(multiTaskingParam)
	tier := param.tire
	app := param.appName
	enableBT := param.enableBT

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

	// Shorten context a bit to allow for cleanup if Run fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	et.Run(ctx, s, cr, a, tier, app, tabletMode, enableBT)
}
