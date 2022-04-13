// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/setup"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type videoCUJParam struct {
	tier        cuj.Tier
	app         string
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoCUJ2,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the smoothess of switch between full screen YouTube video and another browser window",
		Contacts:     []string{"tim.chang@cienet.com", "cienet-development@googlegroups.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars: []string{
			"ui.cuj_mode", // Optional. Expecting "tablet" or "clamshell". Other values will be be taken as "clamshell".
		},
		Params: []testing.Param{
			{
				Name:    "basic_youtube_web",
				Fixture: "loggedInAndKeepState",
				Timeout: 12 * time.Minute,
				Val: videoCUJParam{
					tier: cuj.Basic,
					app:  videocuj.YoutubeWeb,
				},
			}, {
				Name:              "basic_lacros_youtube_web",
				Fixture:           "loggedInAndKeepStateLacrosWithARC",
				Timeout:           12 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoCUJParam{
					tier:        cuj.Basic,
					app:         videocuj.YoutubeWeb,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:              "basic_youtube_web_crosbolt",
				Fixture:           "loggedInAndKeepState",
				Timeout:           10 * time.Minute,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJBasicDevices()),
				Val: videoCUJParam{
					tier: cuj.Basic,
					app:  videocuj.YoutubeWeb,
				},
			}, {
				Name:    "premium_youtube_web",
				Fixture: "loggedInAndKeepState",
				Timeout: 12 * time.Minute,
				Val: videoCUJParam{
					tier: cuj.Premium,
					app:  videocuj.YoutubeWeb,
				},
			}, {
				Name:              "premium_lacros_youtube_web",
				Fixture:           "loggedInAndKeepStateLacrosWithARC",
				Timeout:           12 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoCUJParam{
					tier:        cuj.Premium,
					app:         videocuj.YoutubeWeb,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:              "plus_youtube_web_crosbolt",
				Fixture:           "loggedInAndKeepState",
				Timeout:           10 * time.Minute,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJPlusDevices()),
				Val: videoCUJParam{
					tier: cuj.Plus,
					app:  videocuj.YoutubeWeb,
				},
			}, {
				Name:    "basic_youtube_app",
				Fixture: "loggedInAndKeepState",
				Timeout: 10 * time.Minute,
				Val: videoCUJParam{
					tier: cuj.Basic,
					app:  videocuj.YoutubeApp,
				},
			}, {
				Name:              "basic_lacros_youtube_app",
				Fixture:           "loggedInAndKeepStateLacrosWithARC",
				Timeout:           10 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoCUJParam{
					tier:        cuj.Basic,
					app:         videocuj.YoutubeApp,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:              "basic_youtube_app_crosbolt",
				Fixture:           "loggedInAndKeepState",
				Timeout:           10 * time.Minute,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJBasicDevices()),
				Val: videoCUJParam{
					tier: cuj.Basic,
					app:  videocuj.YoutubeApp,
				},
			}, {
				Name:    "premium_youtube_app",
				Fixture: "loggedInAndKeepState",
				Timeout: 10 * time.Minute,
				Val: videoCUJParam{
					tier: cuj.Premium,
					app:  videocuj.YoutubeApp,
				},
			}, {
				Name:              "premium_lacros_youtube_app",
				Fixture:           "loggedInAndKeepStateLacrosWithARC",
				Timeout:           10 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: videoCUJParam{
					tier:        cuj.Premium,
					app:         videocuj.YoutubeApp,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:              "plus_youtube_app_crosbolt",
				Fixture:           "loggedInAndKeepState",
				Timeout:           10 * time.Minute,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJPlusDevices()),
				Val: videoCUJParam{
					tier: cuj.Plus,
					app:  videocuj.YoutubeApp,
				},
			},
		},
	})
}

// VideoCUJ2 performs the video cases including youtube web, and youtube app.
func VideoCUJ2(ctx context.Context, s *testing.State) {
	p := s.Param().(videoCUJParam)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	a := s.FixtValue().(cuj.FixtureData).ARC
	var lacrosFixtValue lacrosfixt.FixtValue
	if p.browserType == browser.TypeLacros {
		lacrosFixtValue = s.FixtValue().(cuj.FixtureData).LacrosFixt
	}
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
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
	var uiHandler cuj.UIActionHandler
	if tabletMode {
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to rotate display to landscape: ", err)
		}
		defer cleanup(cleanupCtx)
		if uiHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create tablet action handler: ", err)
		}
	} else {
		if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create clamshell action handler: ", err)
		}
	}
	defer uiHandler.Close()

	testResources := videocuj.TestResources{
		Cr:        cr,
		LFixtVal:  lacrosFixtValue,
		Tconn:     tconn,
		A:         a,
		Kb:        kb,
		UIHandler: uiHandler,
	}

	videoCUJParams := s.Param().(videoCUJParam)
	testParams := videocuj.TestParams{
		Tier:            videoCUJParams.tier,
		App:             videoCUJParams.app,
		OutDir:          s.OutDir(),
		TabletMode:      tabletMode,
		ExtendedDisplay: false,
	}

	if err := videocuj.Run(ctx, testResources, testParams); err != nil {
		s.Fatal("Failed to run video cuj test: ", err)
	}
}
