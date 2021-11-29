// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LauncherDragPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures animation smoothness of lancher animations",
		Contacts: []string{
			"newcomer@chromium.org", "tbarzic@chromium.org", "cros-launcher-prod-notifications@google.com",
			"mukai@chromium.org", // original test author
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			Fixture: "chromeLoggedInWith100FakeApps",
		}, {
			Name:    "skia_renderer",
			Fixture: "chromeLoggedInWith100FakeAppsSkiaRenderer",
		}, {
			Name:    "double_buffer_compositing",
			Fixture: "chromeLoggedInWith100FakeAppsDoubleBuffering",
		}},
		Data: []string{"animation.html", "animation.js"},
	})
}

func LauncherDragPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

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

	// It turns out that the shelf icon bounds are not stable, and the test
	// will fail when it refers to an old bounds.  Waiting for the icons bounds
	// to be stable.
	var shelfInfo *ash.ScrollableShelfInfoClass
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		gotShelfInfo, err := ash.FetchScrollableShelfInfoForState(ctx, tconn, &ash.ShelfState{})
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to fetch scrollable shelf info"))
		}
		// Will update shelfInfo anyways.
		defer func() {
			shelfInfo = gotShelfInfo
		}()
		// For the first invocation, it just return failure and update shelfInfo.
		if shelfInfo == nil {
			return errors.New("still waiting for shelf icon bounds to be stable")
		}
		// Check if the icon bounds are same from the previous run, and reports
		// an error when some bounds are different.
		if len(shelfInfo.IconsBoundsInScreen) != len(gotShelfInfo.IconsBoundsInScreen) {
			return errors.New("number of icons is different")
		}
		for i, iconBounds := range shelfInfo.IconsBoundsInScreen {
			gotBounds := gotShelfInfo.IconsBoundsInScreen[i]
			if *iconBounds != *gotBounds {
				return errors.Errorf("%d-th icon bounds different: got %+v want %+v", i, *gotBounds, *iconBounds)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: time.Second}); err != nil {
		s.Fatal("Failed to fetch scrollable shelf info: ", err)
	}

	if len(shelfInfo.IconsBoundsInScreen) == 0 {
		s.Fatal("No icons found in shelf")
	}

	// Points for dragging to open/close the launchers.
	// - X of bottom: should be the background of the shelf (not on app
	//   icons). If there's only one icon, X position is slightly right of the icon. If there
	//   are multiple icons, pick up the middle point between the first and the
	//   second *visible* icon.
	// - X of top: about 1/4 of the screen to avoid the scroll indicator at the
	//   center of the screen.
	// - Y of bottom: in the middle of shelf (bottom of workarea + half of the
	//   shelf, which equals to the average of workarea height and display height)
	// - Y of top: 10px from the top of the screen; this is just like almost top
	//   of the screen.
	appIcons := shelfInfo.IconsBoundsInScreen
	var xPosition int
	if len(appIcons) == 1 {
		xPosition = appIcons[0].Right() + 1
	} else {
		// Find the first visible icon.
		firstVisibleIconIndex := 0
		for i, iconBounds := range appIcons {
			if iconBounds.Left > shelfInfo.LeftArrowBounds.Right() {
				firstVisibleIconIndex = i
				break
			}
		}
		xPosition = (appIcons[firstVisibleIconIndex].Right() + appIcons[firstVisibleIconIndex+1].Left) / 2
	}

	bottom := coords.NewPoint(xPosition, primaryBounds.Top+(primaryBounds.Height+primaryWorkArea.Height)/2)
	top := coords.NewPoint(primaryBounds.Left+primaryBounds.Width/4, primaryBounds.Top+10)

	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL + "/animation.html"

	runner := perfutil.NewRunner(cr.Browser())
	currentWindows := 0
	// Run the dragging gesture for different numbers of browser windows (0 or 2).
	for _, windows := range []int{0, 2} {
		if err := ash.CreateWindows(ctx, tconn, cr, url, windows-currentWindows); err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}
		currentWindows = windows

		suffix := fmt.Sprintf("%dwindows", currentWindows)
		runner.RunMultiple(ctx, s, suffix, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			// Drag from the bottom to the top; this should expand the app-list to
			// fullscreen.
			if err := mouse.Drag(tconn, bottom, top, time.Second)(ctx); err != nil {
				return errors.Wrap(err, "failed to drag from the bottom to top")
			}
			if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
				return errors.Wrap(err, "failed to switch the state to 'FullscreenAllApps'")
			}
			// Drag from the top to the bottom; this should close the app-list.
			if err := mouse.Drag(tconn, top, bottom, time.Second)(ctx); err != nil {
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
