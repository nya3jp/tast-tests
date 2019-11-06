// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DesksAnimationPerf,
		Desc:         "Measures the smoothness of the desk-activation and removal animations",
		Contacts:     []string{"afakhry@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func DesksAnimationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	// Create a new desk other than the default desk, activate it, then remove it.
	if err = ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create a new desk: ", err)
	}
	if err = ash.ActivateDeskAtIndex(ctx, tconn, 1); err != nil {
		s.Fatal("Failed to activate the second desk: ", err)
	}
	if err = ash.RemoveActiveDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to remove the active desk: ", err)
	}

	pv := perf.NewValues()
	for _, histName := range []string{
		"Ash.Desks.AnimationSmoothness.DeskActivation",
		"Ash.Desks.AnimationSmoothness.DeskRemoval",
	} {
		histogram, err := metrics.GetHistogram(ctx, cr, histName)
		if err != nil {
			s.Fatalf("Failed to get histogram %s: %v", histName, err)
		}

		pv.Set(perf.Metric{
			Name:      histName,
			Unit:      "percent",
			Direction: perf.BiggerIsBetter,
		}, histogram.Mean())
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
