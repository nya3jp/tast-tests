// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LauncherDragPerf,
		Desc: "Measures animation smoothness of lancher animations",
		Contacts: []string{
			"newcomer@chromium.org", "tbarzic@chromium.org", "cros-launcher-prod-notifications@google.com",
			"mukai@chromium.org", // original test author
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Pre: ash.LoggedInWith100FakeApps(),
		}, {
			Name:              "skia_renderer",
			Pre:               ash.LoggedInWith100FakeAppsWithSkiaRenderer(),
			ExtraHardwareDeps: hwdep.D(hwdep.Model("nocturne", "krane")),
		}},
	})
}

func LauncherDragPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	defer chromeui.WaitForLocationChangeCompleted(ctx, tconn)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// When the device is tablet-mode by default, entering into clamshell mode may
	// cause a lot of location change events which may not finish timely. So wait
	// here to ensure that the following test scenarios work as expected. See also
	// https://crbug.com/1076520.
	if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
		s.Fatal("Failed to wait for the location changes: ", err)
	}

	var primaryBounds coords.Rect
	var primaryWorkArea coords.Rect
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the display info: ", err)
	}
	for _, info := range infos {
		if info.IsPrimary {
			primaryBounds = info.Bounds
			primaryWorkArea = info.WorkArea
			break
		}
	}
	if primaryBounds.Empty() {
		s.Fatal("No primary display is found")
	}

	// Points for dragging to open/close the launchers.
	// - X of bottom: should be the background of the shelf (not on app
	//   icons). If there's only one icon, X position is slightly right of the icon. If there
	//   are multiple icons, pick up the middle point between the first and the
	//   second icon.
	// - X of top: about 1/4 of the screen to avoid the scroll indicator at the
	//   center of the screen.
	// - Y of bottom: in the middle of shelf (bottom of workarea + half of the
	//   shelf, which equals to the average of workarea height and display height)
	// - Y of top: 10px from the top of the screen; this is just like almost top
	//   of the screen.
	appIcons, err := chromeui.FindAll(ctx, tconn, chromeui.FindParams{ClassName: "ash/ShelfAppButton"})
	if err != nil {
		s.Fatal("Failed to find the app icons: ", err)
	}
	if len(appIcons) == 0 {
		// At least there must be one icon for Chrome itself.
		s.Fatal("No app icons are in the shelf")
	}
	defer appIcons.Release(ctx)
	var xPosition int
	if len(appIcons) == 1 {
		xPosition = appIcons[0].Location.Left + appIcons[0].Location.Width + 1
	} else {
		xPosition = ((appIcons[0].Location.Left + appIcons[0].Location.Width) + appIcons[1].Location.Left) / 2
	}
	bottom := coords.NewPoint(xPosition, primaryBounds.Top+(primaryBounds.Height+primaryWorkArea.Height)/2)
	top := coords.NewPoint(primaryBounds.Left+primaryBounds.Width/4, primaryBounds.Top+10)

	runner := perfutil.NewRunner(cr)
	currentWindows := 0
	// Run the dragging gesture for different numbers of browser windows (0 or 2).
	for _, windows := range []int{0, 2} {
		conns, err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, windows-currentWindows)
		if err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}
		if err := conns.Close(); err != nil {
			s.Error("Failed to close the connection to chrome")
		}
		currentWindows = windows
		// The best effort to stabilize CPU usage. This may or
		// may not be satisfied in time.
		if _, err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
			s.Error("Failed to wait for system UI to be stabilized: ", err)
		}

		suffix := fmt.Sprintf("%dwindows", currentWindows)
		runner.RunMultiple(ctx, s, suffix, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			// Drag from the bottom to the top; this should expand the app-list to
			// fullscreen.
			if err := mouse.Drag(ctx, tconn, bottom, top, time.Second); err != nil {
				return errors.Wrap(err, "failed to drag from the bottom to top")
			}
			if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
				return errors.Wrap(err, "failed to switch the state to 'FullscreenAllApps'")
			}
			// Drag from the top to the bottom; this should close the app-list.
			if err := mouse.Drag(ctx, tconn, top, bottom, time.Second); err != nil {
				return errors.Wrap(err, "failed to drag from the top to bottom")
			}
			if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
				return errors.Wrap(err, "failed to switch the state to 'Closed'")
			}
			return nil
		},
			"Apps.StateTransition.Drag.PresentationTime.ClamshellMode"),
			perfutil.StoreAll(perf.SmallerIsBetter, "ms", suffix))
	}
	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
