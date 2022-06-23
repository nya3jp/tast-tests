// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/edu3dmodelingcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

type edu3DModelingParam struct {
	tier cuj.Tier
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EDU3DModelingCUJ,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Measures the performance of editing the 3D models on TinkerCAD website",
		Contacts:     []string{"xliu@cienet.com", "jeff.lin@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"rotate.png"},
		Fixture:      "enrolledLoggedInToCUJUser",
		Vars: []string{
			// Required. The URL of initial 3D model design. It will be copied when test starts.
			"ui.sampleDesignURL",
			// Optional. Expecting "tablet" or "clamshell". Other values will be be taken as "clamshell".
			"ui.cuj_mode",
		},
		Params: []testing.Param{
			{
				Name:    "plus_tinkercad",
				Timeout: 5 * time.Minute,
				Val: edu3DModelingParam{
					tier: cuj.Plus,
				},
			},
		},
	})
}

func EDU3DModelingCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	sampleDesignURL := s.RequiredVar("ui.sampleDesignURL")

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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
	if tabletMode {
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to rotate display to landscape: ", err)
		}
		defer cleanup(cleanupCtx)
	}

	rotateIconPath := s.DataPath("rotate.png")
	if err := edu3dmodelingcuj.Run(ctx, cr, tabletMode, s.OutDir(), sampleDesignURL, rotateIconPath); err != nil {
		s.Fatal("Failed to run tinkercad cuj: ", err)
	}
}
