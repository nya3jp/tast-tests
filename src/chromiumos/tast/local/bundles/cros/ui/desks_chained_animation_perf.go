// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DesksChainedAnimationPerf,
		Desc:         "Measures the smoothness of a chained desk activation animation",
		Contacts:     []string{"afakhry@chromium.org", "sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
	})
}

func DesksChainedAnimationPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := perfutil.EnsureDisplayOn(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Add three desks for a total of four. Remove them at the end of the test.
	const numDesks = 3
	for i := 0; i < numDesks; i++ {
		if err = ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Error("Failed to create a new desk: ", err)
		}
		defer func() {
			if err = ash.RemoveActiveDesk(ctx, tconn); err != nil {
				s.Error("Failed to remove the active desk: ", err)
			}
		}()
	}

	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Starting at desk 1, activate desk 4 by activating each adjacent desk until we reach it.
		if err = ash.ActivateAdjacentDesksToTargetIndex(ctx, tconn, 3); err != nil {
			return errors.Wrap(err, "failed to activate the fourth desk")
		}
		// Go back to desk 1.
		if err = ash.ActivateAdjacentDesksToTargetIndex(ctx, tconn, 0); err != nil {
			return errors.Wrap(err, "failed to activate the first desk")
		}
		return nil
	},
		"Ash.Desks.AnimationSmoothness.DeskActivation"),
		perfutil.StoreSmoothness)

	if err := pv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
