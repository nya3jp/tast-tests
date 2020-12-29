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
	"chromiumos/tast/local/bundles/cros/ui/cuj/bluetooth"
	et "chromiumos/tast/local/bundles/cros/ui/everydaymultitaskingcuj"
	"chromiumos/tast/local/chrome/ash"
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
					appName:  et.YoutubeMusicAppName,
					enableBT: false,
				},
			}, {
				Name:    "basic_ytmusic_bluetooth",
				Timeout: 10 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Basic,
					appName:  et.YoutubeMusicAppName,
					enableBT: true,
				},
			}, {
				Name:    "plus_ytmusic",
				Timeout: 15 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.YoutubeMusicAppName,
					enableBT: false,
				},
			}, {
				Name:    "plus_ytmusic_bluetooth",
				Timeout: 15 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.YoutubeMusicAppName,
					enableBT: true,
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

	if _, ok := s.Var("ui.cuj_mute"); ok {
		if err := crastestclient.Mute(ctx); err != nil {
			s.Fatal("Failed to mute audio: ", err)
		}
		defer crastestclient.Unmute(ctx)
	}

	isBtEnabled, err := bluetooth.IsEnabled(ctx)
	if err != nil {
		s.Fatal("Failed to get bluetooth status: ", err)
	}

	if enableBT {
		testing.ContextLog(ctx, "Start to connect bluetooth")
		deviceName := s.RequiredVar("ui.bt_devicename")
		if err := bluetooth.ConnectDevice(ctx, deviceName); err != nil {
			s.Fatal("Failed to connect bluetooth: ", err)
		}
		if !isBtEnabled {
			defer func(ctx context.Context) {
				if err := bluetooth.Disable(ctx); err != nil {
					s.Fatal("Failed to disable bluetooth: ", err)
				}
			}(ctx)
		}
	} else if isBtEnabled {
		testing.ContextLog(ctx, "Start to disable bluetooth")
		if err := bluetooth.Disable(ctx); err != nil {
			s.Fatal("Failed to disable bluetooth: ", err)
		}
		defer func(ctx context.Context) {
			if err := bluetooth.Enable(ctx); err != nil {
				s.Fatal("Failed to connect bluetooth: ", err)
			}
		}(ctx)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	var tabletMode bool
	if mode, ok := s.Var("ui.cuj_mode"); ok {
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

	// Shorten context a bit to allow for cleanup if test fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	ccaScriptPaths := []string{s.DataPath("cca_ui.js")}
	if err := et.Run(ctx, cr, a, tier, ccaScriptPaths, s.OutDir(), app, "", tabletMode); err != nil {
		s.Fatal(ctx, "Failed to run everyday multi-tasking cuj test:", err)
	}
}
