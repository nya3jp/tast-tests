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
	"chromiumos/tast/local/bundles/cros/ui/gamecuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type gameParam struct {
	tier cuj.Tier
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GamingOnARCCUJ,
		Desc:         "Measures the performance of ARC++ game",
		Contacts:     []string{"xliu@cienet.com", "jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		Fixture:      "loggedInAndKeepState",
		Vars: []string{
			"ui.cuj_mute",      // Optional. Mute the DUT during the test.
			"ui.cuj_mode",      // Optional. Expecting "tablet" or "clamshell".
			"ui.bt_devicename", // Optional. The name of the Bluetooth device.
		},
		Params: []testing.Param{
			{
				Name:    "plus",
				Timeout: 10 * time.Minute,
				Val: gameParam{
					tier: cuj.Plus,
				},
			},
		},
	})
}

// GamingOnARCCUJ test starts and plays an ARC++ game and gathers the performance.
func GamingOnARCCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cuj.FixtureData).Chrome
	a := s.FixtValue().(cuj.FixtureData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if _, ok := s.Var("ui.cuj_mute"); ok {
		if err := crastestclient.Mute(ctx); err != nil {
			s.Fatal("Failed to mute audio: ", err)
		}
		defer crastestclient.Unmute(cleanupCtx)
	}

	// If ui.bt_devicename is given, then connect to the Bluetooth device.
	if deviceName, ok := s.Var("ui.bt_devicename"); ok {
		isBtEnabled, err := bluetooth.IsEnabled(ctx)
		if err != nil {
			s.Fatal("Failed to get bluetooth status: ", err)
		}
		testing.ContextLog(ctx, "Start to connect bluetooth")
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
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

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
	if tabletMode {
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to rotate display to landscape: ", err)
		}
		defer cleanup(cleanupCtx)
	}

	// Prepare ARC and install android app for the game cuj.
	arcGame := gamecuj.NewArcGame(ctx, kb, tconn, a, d)
	if err := gamecuj.Run(ctx, s, cr, arcGame, tabletMode); err != nil {
		s.Fatal("Failed to run game cuj: ", err)
	}
}
