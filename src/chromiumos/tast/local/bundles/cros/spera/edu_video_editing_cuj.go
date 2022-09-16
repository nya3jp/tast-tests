// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/spera/videoeditingcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EDUVideoEditingCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of editing video on the web",
		Contacts:     []string{"xliu@cienet.com", "jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			// Optional. Expecting "tablet" or "clamshell". Other values will be be taken as "clamshell".
			"spera.cuj_mode",
			"spera.collectTrace", // Optional. Expecting "enable" or "disable", default is "disable".
		},
		Data: []string{cujrecorder.SystemTraceConfigFile},
		Params: []testing.Param{
			{
				Name:    "premium_wevideo",
				Fixture: "enrolledLoggedInToCUJUser",
				Timeout: 5 * time.Minute,
				Val:     browser.TypeAsh,
			},
			{
				Name:              "premium_lacros_wevideo",
				Timeout:           5 * time.Minute,
				Fixture:           "enrolledLoggedInToCUJUserLacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Val:               browser.TypeLacros,
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
	if err := videoeditingcuj.Run(ctx, s.OutDir(), traceConfigPath, cr, tabletMode, s.Param().(browser.Type)); err != nil {
		s.Fatal("Failed to run the video editing on the web cuj: ", err)
	}
}
