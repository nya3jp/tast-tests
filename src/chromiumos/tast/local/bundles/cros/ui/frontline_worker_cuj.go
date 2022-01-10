// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/frontlineworkercuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/policyutil/mgs"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FrontlineWorkerCUJ,
		Desc:         "Measures the performance of Frontline Worker CUJ",
		Contacts:     []string{"xliu@cienet.com", "alston.huang@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      fixture.ManagedGuestSessionWithExtensions,
		Vars: []string{
			"ui.cujAccountPool", // Required. It is necessary to have account to use Google Sheets.
			"ui.sampleSheetURL", // Required. The URL of sample Google Sheet. It will be copied to create a new one to perform tests on.
			"ui.cuj_mode",       // Optional. Expecting "tablet" or "clamshell".
		},
		Params: []testing.Param{
			{
				Name:    "basic_browsing",
				Timeout: 10 * time.Minute,
				Val: frontlineWorkerParam{
					Workload: frontlineworkercuj.Browsering,
					Tier:     cuj.Basic,
				},
			},
			{
				Name:    "basic_collaborating",
				Timeout: 10 * time.Minute,
				Val: frontlineWorkerParam{
					Workload: frontlineworkercuj.Collaborating,
					Tier:     cuj.Basic,
				},
			},
		},
	})
}

type frontlineWorkerParam struct {
	Workload frontlineworkercuj.Workload
	Tier     cuj.Tier
}

// FrontlineWorkerCUJ measures the system performance.
func FrontlineWorkerCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*mgs.FixtData).Chrome()
	param := s.Param().(frontlineWorkerParam)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Shorten context a bit to allow for cleanup if Run fails.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

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

	if err := frontlineworkercuj.Run(ctx, s, cr, param.Workload, tabletMode); err != nil {
		s.Fatal("Failed to run frontlineworker cuj: ", err)
	}
}
