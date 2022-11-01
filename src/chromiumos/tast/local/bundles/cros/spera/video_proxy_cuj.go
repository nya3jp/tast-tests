// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/spera/videoproxycuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

type gamingProxyCUJParam struct {
	browserType browser.Type
	videoOption gamingproxycuj.VideoOption
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoProxyCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "A test case that simulates the Online Gaming Platform testing",
		Contacts:     []string{"xliu@cienet.com", "jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"spera.cuj_mode",     // Optional. Expecting "tablet" or "clamshell".
			"spera.collectTrace", // Optional. Expecting "enable" or "disable", default is "disable".
		},
		Data: []string{cujrecorder.SystemTraceConfigFile},
		Params: []testing.Param{
			{
				Name:    "essential_h264",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeAsh,
					VideoOption: videoproxycuj.H264DASH1080P60FPS,
				},
			},
			{
				Name:    "essential_vp9",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeAsh,
					VideoOption: videoproxycuj.VP9DASH1080P60FPS,
				},
			},
			{
				Name:    "essential_hevc",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeAsh,
					VideoOption: videoproxycuj.HEVC1080P60FPS,
				},
			},
			{
				Name:    "advanced_h264",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeAsh,
					VideoOption: videoproxycuj.H264DASH4K60FPS,
				},
			},
			{
				Name:    "advanced_av1",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeAsh,
					VideoOption: videoproxycuj.AV1DASH60FPS,
				},
			},
			{
				Name:    "advanced_vp9",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeAsh,
					VideoOption: videoproxycuj.VP9DASH4K60FPS,
				},
			},
			{
				Name:    "advanced_hevc",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeAsh,
					VideoOption: videoproxycuj.HEVC4K60FPS,
				},
			},
			{
				Name:              "essential_lacros_h264",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeLacros,
					VideoOption: videoproxycuj.H264DASH1080P60FPS,
				},
			},
			{
				Name:              "essential_lacros_vp9",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeLacros,
					VideoOption: videoproxycuj.VP9DASH1080P60FPS,
				},
			},
			{
				Name:              "essential_lacros_hevc",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeLacros,
					VideoOption: videoproxycuj.HEVC1080P60FPS,
				},
			},
			{
				Name:              "advanced_lacros_h264",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeLacros,
					VideoOption: videoproxycuj.H264DASH4K60FPS,
				},
			},
			{
				Name:              "advanced_lacros_vp9",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeLacros,
					VideoOption: videoproxycuj.VP9DASH4K60FPS,
				},
			},
			{
				Name:              "advanced_lacros_av1",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeLacros,
					VideoOption: videoproxycuj.AV1DASH60FPS,
				},
			},
			{
				Name:              "advanced_lacros_hevc",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoproxycuj.TestParams{
					BrowserType: browser.TypeLacros,
					VideoOption: videoproxycuj.HEVC4K60FPS,
				},
			},
		},
	})
}

func VideoProxyCUJ(ctx context.Context, s *testing.State) {
	p := s.Param().(videoproxycuj.TestParams)
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
	traceConfigPath := ""
	if collect, ok := s.Var("spera.collectTrace"); ok && collect == "enable" {
		traceConfigPath = s.DataPath(cujrecorder.SystemTraceConfigFile)
	}
	if err := videoproxycuj.Run(ctx, cr, s.OutDir(), traceConfigPath, tabletMode, p.BrowserType, p.VideoOption); err != nil {
		s.Fatal("Failed to run video proxy cuj: ", err)
	}
}
