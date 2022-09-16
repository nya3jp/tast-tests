// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LauncherPageSwitchPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Measures smoothness of switching pages within the launcher",
		Contacts: []string{
			"newcomer@chromium.org", "tbarzic@chromium.org", "cros-launcher-prod-notifications@google.com",
			"mukai@chromium.org", // original test author
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedInWith100FakeApps",
		Timeout:      3 * time.Minute,
	})
}

func LauncherPageSwitchPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	const dragDuration = 2 * time.Second
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Pages only exist in the tablet mode launcher.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	var pc pointer.Context
	if pc, err = pointer.NewTouch(ctx, tconn); err != nil {
		s.Fatal("Failed to create a touch controller")
	}
	defer pc.Close()

	if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, 2); err != nil {
		s.Fatal("Failed to create windows: ", err)
	}

	// Currently tast test may show a couple of notifications like "sign-in error"
	// and they may overlap with UI components of launcher. This prevents intended
	// actions on certain devices and causes test failures. Open and close the
	// quick settings to dismiss those notification popups. See
	// https://crbug.com/1084185.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to open/close the quick settings: ", err)
	}

	// Press "Search" to hide open windows and show the home launcher.
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to obtain the keyboard")
	}
	defer kw.Close()

	accel := "Search"
	if err := kw.Accel(ctx, accel); err != nil {
		s.Fatalf("Failed to type %s: %v", accel, err)
	}

	// Wait for the launcher state change.
	if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	// Find the apps grid view bounds.
	ac := uiauto.New(tconn)
	appsGridView := nodewith.ClassName("AppsGridView")
	pageSwitcher := nodewith.ClassName("PageSwitcher")
	pageButtons := nodewith.ClassName("IconButton").Ancestor(pageSwitcher)

	buttonsInfo, err := ac.NodesInfo(ctx, pageButtons)
	if err != nil {
		s.Fatal("Failed to find the page switcher buttons: ", err)
	}
	if len(buttonsInfo) < 3 {
		s.Fatalf("There are too few pages (%d), want more than 2 pages", len(buttonsInfo))
	}

	runner := perfutil.NewRunner(cr.Browser())

	const pageSwitchTimeout = 2 * time.Second
	clickPageButtonAndWait := func(idx int) action.Action {
		return ac.WaitForEvent(pageSwitcher, event.Alert, pc.Click(pageButtons.Nth(idx)))
	}
	// First: scroll by click. Clicking the second one, clicking the first one to
	// go back, clicking the last one to long-jump, clicking the first one again
	// to long-jump back to the original page.
	s.Log("Starting the scroll by click")
	runner.RunMultiple(ctx, "click", uiperf.Run(s, perfutil.RunAndWaitAll(tconn, action.Combine(
		"switch page by buttons",
		clickPageButtonAndWait(1),
		clickPageButtonAndWait(0),
		clickPageButtonAndWait(len(buttonsInfo)-1),
		clickPageButtonAndWait(0),
	), "Apps.PaginationTransition.AnimationSmoothness.TabletMode")),
		perfutil.StoreSmoothness)

	// Second: scroll by drags. This involves two types of operations, drag-up
	// from the bottom for scrolling to the next page, and the drag-down from the
	// top for scrolling to the previous page. In order to prevent overscrolling
	// on the first page which might cause unexpected effects (like closing of the
	// launcher itself), and because the first page might be in a special
	// arrangement of the default apps with blank area at the bottom which
	// prevents the drag-up gesture, switches to the second page before starting
	// this scenario.
	s.Log("Starting the scroll by drags")
	if err := clickPageButtonAndWait(1)(ctx); err != nil {
		s.Fatal("Failed to switch to the second page: ", err)
	}
	// Reset back to the first page at the end of the test; apps-grid page
	// selection may stay in the last page, and that causes troubles on another
	// test. See https://crbug.com/1081285.
	defer func() {
		if err := pc.Click(pageButtons.First())(ctx); err != nil {
			s.Fatal("Failed to click the first page button: ", err)
		}
	}()
	appsGridLocation, err := ac.Location(ctx, appsGridView)
	if err != nil {
		s.Fatal("Failed to get the location of appsgridview: ", err)
	}
	// drag-up gesture positions; starting at a bottom part of the apps-grid (at
	// 4/5 height), and moves to the top of the apps grid bounds. The starting
	// height shouldn't be very bottom of the apps-grid, since it may fall into the
	// hotseat (the gesture won't cause page-switch in such case).
	// The X position should not be the center of the width since it would fall
	// into an app icon. For now, it just sets 2/5 width position to avoid app
	// icons.
	dragUpStart := coords.NewPoint(
		appsGridLocation.Left+appsGridLocation.Width*2/5,
		appsGridLocation.Top+appsGridLocation.Height*4/5)
	dragUpEnd := coords.NewPoint(dragUpStart.X, appsGridLocation.Top+1)

	// drag-down gesture positions; starting at the top of the apps-grid (but
	// not at the edge), and moves to the bottom edge of the apps-grid. Same
	// X position as the drag-up gesture.
	dragDownStart := coords.NewPoint(dragUpStart.X, appsGridLocation.Top+1)
	dragDownEnd := coords.NewPoint(dragDownStart.X, dragDownStart.Y+appsGridLocation.Height)

	runner.RunMultiple(ctx, "drag", uiperf.Run(s, perfutil.RunAndWaitAll(tconn, action.Combine(
		"launcher page drag",
		// Drag-up operation.
		ac.WaitForEvent(pageSwitcher, event.Alert, pc.Drag(dragUpStart, pc.DragTo(dragUpEnd, dragDuration))),
		// Drag-down operation.
		ac.WaitForEvent(pageSwitcher, event.Alert, pc.Drag(dragDownStart, pc.DragTo(dragDownEnd, dragDuration))),
	),
		"Apps.PaginationTransition.DragScroll.PresentationTime.TabletMode",
		"Apps.PaginationTransition.DragScroll.PresentationTime.MaxLatency.TabletMode")),
		perfutil.StoreLatency)

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
