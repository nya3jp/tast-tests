// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/videoeditingcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

type videoEditingParam struct {
	tier cuj.Tier
	app  string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EDUVideoEditingCUJ,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Measures the performance of editing video on the web",
		Contacts:     []string{"xliu@cienet.com", "jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "enrolledLoggedInToCUJUser",
		Vars: []string{
			// Optional. Expecting "tablet" or "clamshell". Other values will be be taken as "clamshell".
			"ui.cuj_mode",
		},
		Params: []testing.Param{
			{
				Name:    "premium_wevideo",
				Timeout: 5 * time.Minute,
				Val: videoEditingParam{
					tier: cuj.Premium,
					app:  "WeVideo",
				},
			},
		},
	})
}

func EDUVideoEditingCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

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
	if err := videoeditingcuj.Run(ctx, s.OutDir(), cr, tabletMode); err != nil {
		s.Fatal("Failed to run the video editing on the web cuj: ", err)
	}
}
