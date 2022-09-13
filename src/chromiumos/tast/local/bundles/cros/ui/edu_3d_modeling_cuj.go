// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/edu3dmodelingcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		// TODO (b/242590511): Deprecated after moving all performance cuj test cases to chromiumos/tast/local/bundles/cros/spera directory.
		Func:         EDU3DModelingCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of editing the 3D models on TinkerCAD website",
		Contacts:     []string{"abergman@google.com", "xliu@cienet.com", "jeff.lin@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"rotate.png"},
		Vars: []string{
			// Required. The URL of initial 3D model design. It will be copied when test starts.
			"ui.sampleDesignURL",
			// Optional. Expecting "tablet" or "clamshell". Other values will be be taken as "clamshell".
			"ui.cuj_mode",
		},
		Params: []testing.Param{
			{
				Name:    "plus_tinkercad",
				Timeout: 6 * time.Minute,
				Fixture: "enrolledLoggedInToCUJUser",
				Val:     browser.TypeAsh,
			},
			{
				Name:              "plus_lacros_tinkercad",
				Timeout:           6 * time.Minute,
				Fixture:           "enrolledLoggedInToCUJUserLacros",
				Val:               browser.TypeLacros,
				ExtraSoftwareDeps: []string{"lacros"},
			},
		},
	})
}

func EDU3DModelingCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	sampleDesignURL := s.RequiredVar("spera.sampleDesignURL")

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

	bt := s.Param().(browser.Type)
	rotateIconPath := s.DataPath("rotate.png")
	if err := edu3dmodelingcuj.Run(ctx, cr, tabletMode, bt, s.OutDir(), sampleDesignURL, rotateIconPath); err != nil {
		s.Fatal("Failed to run tinkercad cuj: ", err)
	}
}
