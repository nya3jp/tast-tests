// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/spera/gamingproxycuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

const manualTestTime = 5 * time.Minute

func init() {
	testing.AddTest(&testing.Test{
		Func:         GamingProxyManualCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Semi-automated tests that allow users to manually operate on the browser UI with keyboard/mouse",
		Contacts:     []string{"xliu@cienet.com", "jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"spera.cuj_mode", // Optional. Expecting "tablet" or "clamshell".
		},
		Params: []testing.Param{
			{
				Name:    "basic_h264_1080p_60fps",
				Fixture: "loggedInAndKeepState",
				Timeout: manualTestTime + 2*time.Minute,
				Val: gamingproxycuj.TestParams{
					BrowserType:    browser.TypeAsh,
					VideoOption:    gamingproxycuj.H264DASH1080P60FPS,
					ManualTestTime: manualTestTime,
				},
			},
			{
				Name:    "plus_av1_4k_60fps",
				Fixture: "loggedInAndKeepState",
				Timeout: manualTestTime + 2*time.Minute,
				Val: gamingproxycuj.TestParams{
					BrowserType:    browser.TypeAsh,
					VideoOption:    gamingproxycuj.AV1DASH60FPS,
					ManualTestTime: manualTestTime,
				},
			},
			{
				Name:              "basic_lacros_h264_1080p_60fps",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           manualTestTime + 2*time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: gamingproxycuj.TestParams{
					BrowserType:    browser.TypeLacros,
					VideoOption:    gamingproxycuj.H264DASH1080P60FPS,
					ManualTestTime: manualTestTime,
				},
			},
			{
				Name:              "plus_lacros_av1_4k_60fps",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           manualTestTime + 2*time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: gamingproxycuj.TestParams{
					BrowserType:    browser.TypeLacros,
					VideoOption:    gamingproxycuj.AV1DASH60FPS,
					ManualTestTime: manualTestTime,
				},
			},
		},
	})
}

// GamingProxyManualCUJ simulates the Online Gaming Platform with manual testing.
func GamingProxyManualCUJ(ctx context.Context, s *testing.State) {
	p := s.Param().(gamingproxycuj.TestParams)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	var tabletMode bool
	if mode, ok := s.Var("spera.cuj_mode"); ok {
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

	if err := gamingproxycuj.RunＭanual(ctx, cr, s.OutDir(), tabletMode, p.BrowserType, p.VideoOption, p.ManualTestTime); err != nil {
		s.Fatal("Failed to run gaming proxy manual cuj: ", err)
	}
}
