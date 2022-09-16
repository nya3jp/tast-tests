// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

type gamingProxyCUJParam struct {
	browserType browser.Type
	videoOption gamingproxycuj.VideoOption
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         GamingProxyCUJ,
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
				Name:    "basic_h264_1080p_60fps",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: gamingProxyCUJParam{
					browserType: browser.TypeAsh,
					videoOption: gamingproxycuj.H264DASH1080P60FPS,
				},
			},
			{
				Name:    "basic_vp9_1080p_60fps",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: gamingProxyCUJParam{
					browserType: browser.TypeAsh,
					videoOption: gamingproxycuj.VP9DASH1080P60FPS,
				},
			},
			{
				Name:    "plus_h264_4k_60fps",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: gamingProxyCUJParam{
					browserType: browser.TypeAsh,
					videoOption: gamingproxycuj.H264DASH4K60FPS,
				},
			},
			{
				Name:    "plus_av1_4k_60fps",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: gamingProxyCUJParam{
					browserType: browser.TypeAsh,
					videoOption: gamingproxycuj.AV1DASH60FPS,
				},
			},
			{
				Name:    "plus_vp9_4k_60fps",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: gamingProxyCUJParam{
					browserType: browser.TypeAsh,
					videoOption: gamingproxycuj.VP9DASH4K60FPS,
				},
			},
			{
				Name:              "basic_lacros_h264_1080p_60fps",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: gamingProxyCUJParam{
					browserType: browser.TypeLacros,
					videoOption: gamingproxycuj.H264DASH1080P60FPS,
				},
			},
			{
				Name:              "basic_lacros_vp9_1080p_60fps",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: gamingProxyCUJParam{
					browserType: browser.TypeLacros,
					videoOption: gamingproxycuj.VP9DASH1080P60FPS,
				},
			},
			{
				Name:              "plus_lacros_h264_4k_60fps",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: gamingProxyCUJParam{
					browserType: browser.TypeLacros,
					videoOption: gamingproxycuj.H264DASH4K60FPS,
				},
			},
			{
				Name:              "plus_lacros_av1_4k_60fps",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: gamingProxyCUJParam{
					browserType: browser.TypeLacros,
					videoOption: gamingproxycuj.AV1DASH60FPS,
				},
			},
			{
				Name:              "plus_lacros_vp9_4k_60fps",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: gamingProxyCUJParam{
					browserType: browser.TypeLacros,
					videoOption: gamingproxycuj.VP9DASH4K60FPS,
				},
			},
		},
	})
}

// GamingProxyCUJ simulates the Online Gaming Platform testing.
func GamingProxyCUJ(ctx context.Context, s *testing.State) {
	p := s.Param().(gamingProxyCUJParam)
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
	if err := gamingproxycuj.Run(ctx, cr, s.OutDir(), traceConfigPath, tabletMode, p.browserType, p.videoOption); err != nil {
		s.Fatal("Failed to run gaming proxy cuj: ", err)
	}
}
