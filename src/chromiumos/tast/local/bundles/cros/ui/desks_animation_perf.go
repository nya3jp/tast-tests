// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
		Func:         DesksAnimationPerf,
		Desc:         "Measures the smoothness of the desk-activation and removal animations",
		Contacts:     []string{"afakhry@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          chrome.LoggedIn(),
	})
}

func DesksAnimationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	pv := perfutil.RunMultiple(ctx, s, cr, perfutil.RunAndWaitAll(tconn, func() error {
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
		"Ash.Desks.AnimationSmoothness.DeskRemoval"),
		perfutil.StoreSmoothness)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
