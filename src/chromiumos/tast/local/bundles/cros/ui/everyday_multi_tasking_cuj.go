// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
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
			"ui.cuj_username",  // Used to select the account to do login to spotify.
			"ui.bt_devicename", // Required for Bluetooth subtests.
		},
		Fixture: "loggedInAndKeepState",
		Data:    []string{"cca_ui.js"},
		Params: []testing.Param{
			{
				Name:    "basic_ytmusic",
				Timeout: 20 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Basic,
					appName:  et.YoutubeMusicAppName,
					enableBT: false,
				},
			}, {
				Name:    "basic_ytmusic_bluetooth",
				Timeout: 20 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Basic,
					appName:  et.YoutubeMusicAppName,
					enableBT: true,
				},
			}, {
				Name:    "basic_spotify_bluetooth",
				Timeout: 20 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Basic,
					appName:  et.SpotifyAppName,
					enableBT: true,
				},
			}, {
				Name:    "plus_ytmusic",
				Timeout: 30 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.YoutubeMusicAppName,
					enableBT: false,
				},
			}, {
				Name:    "plus_ytmusic_bluetooth",
				Timeout: 30 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.YoutubeMusicAppName,
					enableBT: true,
				},
			}, {
				Name:    "plus_spotify_bluetooth",
				Timeout: 30 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.SpotifyAppName,
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

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if _, ok := s.Var("ui.cuj_mute"); ok {
		if err := crastestclient.Mute(ctx); err != nil {
			s.Fatal("Failed to mute audio: ", err)
		}
		defer crastestclient.Unmute(cleanupCtx)
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
			}(cleanupCtx)
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
		}(cleanupCtx)
	}

	// The cras active node will change if the bluetooth has been connected or disconnected.
	// Wait until cras node change completes before doing output volume testing.
	condition := func(cn *audio.CrasNode) bool {
		if !cn.Active {
			return false
		}
		testing.ContextLog(ctx, "Cras active node is: ", cn)

		// According to src/third_party/adhd/cras/README.dbus-api,
		// the audio.CrasNode.Type should be "BLUETOOTH" when a BlueTooth device
		// is used as input or output.
		if enableBT {
			return cn.Type == "BLUETOOTH"
		}
		return cn.Type != "BLUETOOTH"
	}
	if err := audio.WaitForDeviceUntil(ctx, condition); err != nil {
		s.Fatal("Failed to wait for cras node to be in expected status: ", err)
	}

	// Spotify login account.
	var account string
	if app == et.SpotifyAppName {
		account = s.RequiredVar("ui.cuj_username")
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
		defer cleanup(cleanupCtx)
	} else {
		// Use default screen mode of the DUT.
		tabletMode, err = ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get DUT default screen mode: ", err)
		}
	}
	s.Log("Running test with tablet mode: ", tabletMode)

	ccaScriptPaths := []string{s.DataPath("cca_ui.js")}
	if err := et.Run(ctx, cr, a, tier, ccaScriptPaths, s.OutDir(), app, account, tabletMode); err != nil {
		s.Fatal("Failed to run everyday multi-tasking cuj test: ", err)
	}
}
