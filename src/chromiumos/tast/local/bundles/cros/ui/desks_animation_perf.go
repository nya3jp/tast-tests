// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DesksAnimationPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures the smoothness of the desk-activation and removal animations",
		Contacts:     []string{"afakhry@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
	})
}

func DesksAnimationPerf(ctx context.Context, s *testing.State) {
	// Reserve five seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer ash.CleanUpDesks(cleanupCtx, tconn)
	pv := perfutil.RunMultiple(ctx, cr.Browser(), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Create a new desk other than the default desk, activate it, then remove it.
		if err = ash.CreateNewDesk(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create a new desk")
		}
		if err = ash.ActivateDeskAtIndex(ctx, tconn, 1); err != nil {
			return errors.Wrap(err, "failed to activate the second desk")
		}
		if err = ash.RemoveActiveDesk(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to remove the active desk")
		}
		return nil
	},
		"Ash.Desks.AnimationSmoothness.DeskActivation",
		"Ash.Desks.AnimationSmoothness.DeskRemoval")),
		perfutil.StoreSmoothness)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
