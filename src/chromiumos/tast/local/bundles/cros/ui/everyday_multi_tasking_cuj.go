// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	et "chromiumos/tast/local/bundles/cros/ui/everydaymultitaskingcuj"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type multiTaskingParam struct {
	tier     cuj.Tier
	appName  string
	enableBT bool // enable the bluetooth or not
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EverydayMultiTaskingCUJ,
		Desc:         "Measures the performance of everyday multi-tasking CUJ",
		Contacts:     []string{"xliu@cienet.com", "jane.yang@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars: []string{
			"ui.cuj_mute",      // Optional. Mute the DUT during the test.
			"ui.cuj_mode",      // Optional. Expecting "tablet" or "clamshell".
			"ui.bt_devicename", // Required for Bluetooth subtests.
		},
		Fixture: "loggedInAndKeepState",
		Data:    []string{"cca_ui.js"},
		Params: []testing.Param{
			{
				Name:    "basic_ytmusic",
				Timeout: 10 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Basic,
					appName:  et.YoutubeMusic,
					enableBT: false,
				},
			}, {
				Name:    "plus_ytmusic",
				Timeout: 15 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.YoutubeMusic,
					enableBT: false,
				},
			},
		},
	})
}

func EverydayMultiTaskingCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(multiTaskingParam)
	tier := param.tier
	app := param.appName
	enableBT := param.enableBT

	cr := s.FixtValue().(cuj.FixtureData).Chrome
	a := s.FixtValue().(cuj.FixtureData).ARC

	tabletMode := false

	// Shorten context a bit to allow for cleanup if Run fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	if _, ok := s.Var("ui.cuj_mute"); ok {
		if err := crastestclient.Mute(ctx); err != nil {
			s.Fatal("Failed to mute audio: ", err)
		}
		defer crastestclient.Unmute(ctx)
	}

	et.Run(ctx, s, cr, a, tier, app, tabletMode, enableBT)
}
