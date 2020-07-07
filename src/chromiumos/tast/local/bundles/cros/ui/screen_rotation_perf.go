// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenRotationPerf,
		Desc:         "Measures animation smoothness of screen rotation in tablet mode",
		Contacts:     []string{"chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
	})
}

func ScreenRotationPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	currentWindows := 0
	runner := perfutil.NewRunner(cr)
	// Run the screen rotation in overview mode with 2 or 8 windows.
	for _, windows := range []int{2, 8} {
		conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, windows-currentWindows)
		if err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}
		defer conns.Close()
		currentWindows = windows

		if err = cpu.WaitUntilIdle(ctx); err != nil {
			s.Fatal("Failed to because CPU didn't idle in time: ", err)
		}

		if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter into the overview mode: ", err)
		}

		suffix := fmt.Sprintf("%dwindows", windows)
		runner.RunMultiple(ctx, s, suffix, perfutil.RunAndWaitAll(tconn, func() error {
			for _, rotation := range []display.RotationAngle{display.Rotate90, display.Rotate180, display.Rotate270, display.Rotate0} {
				if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, rotation); err != nil {
					return errors.Wrap(err, "failed to rotate display")
				}
			}
			return nil
		}, "Ash.Rotation.AnimationSmoothness"),
			perfutil.StoreAll(perf.BiggerIsBetter, "percent", suffix))
	}

	if err := runner.Values().Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
