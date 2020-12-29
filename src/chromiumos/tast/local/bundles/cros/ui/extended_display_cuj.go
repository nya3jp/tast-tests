// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/chameleon"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type extendedDisplayCUJParam struct {
	tier cuj.Tier
	app  string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ExtendedDisplayCUJ,
		Desc:         "Test video entertainment with extended display",
		Contacts:     []string{"vlin@cienet.com", "cienet-development@googlegroups.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "loggedInAndKeepState",
		Vars: []string{
			"ui.cuj_mode", // Optional. Use "tablet" to force the tablet mode. Other values will be be taken as "clamshell".

			"ui.chameleon_addr",         // Only needed when using chameleon board as extended display.
			"ui.chameleon_display_port", // The port connected as extended display. Default is 3.
		},
		Params: []testing.Param{
			{
				Name:    "plus_video_youtube_web",
				Timeout: 10 * time.Minute,
				Val: extendedDisplayCUJParam{
					tier: cuj.Plus,
					app:  videocuj.YoutubeWeb,
				},
			},
		},
	})
}

// ExtendedDisplayCUJ performs the video cuj (youtube web) test on extended display.
// Known issues: b:187165216 describes an issue that click event cannot be executed
// on extended display on certain models.
func ExtendedDisplayCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cuj.FixtureData).Chrome
	a := s.FixtValue().(cuj.FixtureData).ARC

	if chameleonAddr, ok := s.Var("ui.chameleon_addr"); ok {
		// Use chameleon board as extended display. Make sure chameleon is connected.
		che, err := chameleon.New(ctx, chameleonAddr)
		if err != nil {
			s.Fatal("Failed to connect to chameleon board: ", err)
		}
		defer che.Close(ctx)

		portID := 3 // Use default port 3 for display.
		if port, ok := s.Var("ui.chameleon_display_port"); ok {
			portID, err = strconv.Atoi(port)
			if err != nil {
				s.Fatalf("Failed to parse chameleon display port %q: %v", port, err)
			}
		}

		dp, err := che.NewPort(ctx, portID)
		if err != nil {
			s.Fatalf("Failed to create chameleon port %d: %v", portID, err)
		}
		if err := dp.Plug(ctx); err != nil {
			s.Fatal("Failed to plug chameleon port: ", err)
		}
		defer dp.Unplug(ctx)
		// Wait for DUT to detect external display.
		if err := dp.WaitVideoInputStable(ctx, 10*time.Second); err != nil {
			s.Fatal("Failed to wait for video input on chameleon board: ", err)
		}
	}

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
		defer cleanup(ctx)
	} else {
		// Use default screen mode of the DUT.
		tabletMode, err = ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get DUT default screen mode: ", err)
		}
	}
	s.Log("Running test with tablet mode: ", tabletMode)

	// The external display is not able to be controlled by touch event,
	// so we'll test extended display with keyboard/mouse even on tablet mode.
	uiHandler, err := cuj.NewClamshellActionHandler(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	testResources := videocuj.TestResources{
		Cr:        cr,
		Tconn:     tconn,
		A:         a,
		Kb:        kb,
		UIHandler: uiHandler,
	}

	param := s.Param().(extendedDisplayCUJParam)
	testParams := videocuj.TestParams{
		Tier:            param.tier,
		App:             param.app,
		OutDir:          s.OutDir(),
		TabletMode:      tabletMode,
		ExtendedDisplay: true,
	}

	// Shorten context a bit to allow for cleanup if Run fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	if err := videocuj.Run(ctx, testResources, testParams); err != nil {
		s.Fatal("Failed to do video cuj testing: ", err)
	}
}
