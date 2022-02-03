// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BubbleLauncherAnimationPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures animation smoothness of bubble launcher animations",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"jamescook@chromium.org",
			"tbarzic@chromium.org",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Fixture:      "chromeLoggedInWith100FakeAppsProductivityLauncher",
		Data:         []string{"animation.html", "animation.js"},
	})
}

const (
	openHistogramName  = "Apps.ClamshellLauncher.AnimationSmoothness.OpenAppsPage"
	closeHistogramName = "Apps.ClamshellLauncher.AnimationSmoothness.Close"
)

func openAndCloseLauncher(ctx context.Context, tconn *chrome.TestConn, ui *uiauto.Context) error {
	// The bubble UI node is created before the show animation finishes, so wait for the
	// smoothness histogram to be sure the launcher is fully open.
	histo, err := metrics.GetHistogram(ctx, tconn, openHistogramName)
	if err != nil {
		return errors.Wrap(err, "couldn't get initial open histogram")
	}
	// Open the launcher.
	if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelSearch); err != nil {
		return errors.Wrap(err, "failed to press Search")
	}
	bubble := nodewith.ClassName(ash.AppListBubbleClassName)
	if err := ui.WaitUntilExists(bubble)(ctx); err != nil {
		return errors.Wrap(err, "could not open bubble by pressing Search key")
	}
	if _, err := metrics.WaitForHistogramUpdate(ctx, tconn, openHistogramName, histo, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait for open histogram")
	}
	// The open animation is done. Press the search key again to close the launcher.
	if err := ash.TriggerLauncherStateChange(ctx, tconn, ash.AccelSearch); err != nil {
		return errors.Wrap(err, "failed to press Search again")
	}
	if err := ui.WaitUntilGone(bubble)(ctx); err != nil {
		return errors.Wrap(err, "could not close bubble by pressing Search key")
	}
	return nil
}

func BubbleLauncherAnimationPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	// Bubble launcher requires clamshell mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	// Run an http server to serve the test contents for accessing from the browser.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL + "/animation.html"

	// Ensure launcher is closed by clicking in the top-left corner of the screen.
	// Use the mouse because there's no keyboard shortcut that closes the launcher
	// independent of its current state.
	ui := uiauto.New(tconn)
	bubble := nodewith.ClassName(ash.AppListBubbleClassName)
	if err := uiauto.Combine("close bubble by clicking in screen corner",
		mouse.Click(tconn, coords.Point{X: 0, Y: 0}, mouse.LeftButton),
		ui.WaitUntilGone(bubble),
	)(ctx); err != nil {
		s.Fatal("Could not close bubble by clicking in screen corner: ", err)
	}

	// Move the cursor so it doesn't overlap the bubble. mouse.Click() shows the
	// cursor but does not change the mouse position.
	if err := mouse.Move(tconn, coords.Point{X: 0, Y: 0}, 0)(ctx); err != nil {
		s.Fatal("Could not move mouse to screen corner: ", err)
	}

	// Wait for CPU to idle, since it's likely this test fixture triggered a login.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait for CPU to become idle: ", err)
	}

	// Run the launcher open/close flow with no browser windows open.
	// This aligns with ui.LauncherAnimationPerf for the legacy launcher.
	name := "0windows"
	runner := perfutil.NewRunner(cr.Browser())
	runner.RunMultiple(ctx, s, name,
		perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			return openAndCloseLauncher(ctx, tconn, ui)
		}, openHistogramName, closeHistogramName),
		perfutil.StoreAll(perf.BiggerIsBetter, "percent", name))

	// Open 2 browser windows with web contents playing an animation.
	if err := ash.CreateWindows(ctx, tconn, cr, url, 2); err != nil {
		s.Fatal("Failed to create browser windows: ", err)
	}
	// Maximize all windows to ensure a consistent state.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
	}); err != nil {
		s.Fatal("Failed to maximize windows: ", err)
	}

	// Wait for 1 seconds to stabilize the result. According to the bug below, this
	// can't be cpu.WaitUntilIdle(). The web contents animation is consuming CPU.
	// TODO(crbug.com/1001314): Find a better way to way on the idleness of Ash.
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	// Run the the flow again with 2 browser windows open.
	// This aligns with ui.LauncherAnimationPerf for the legacy launcher.
	name = "2windows"
	runner.RunMultiple(ctx, s, name,
		perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			return openAndCloseLauncher(ctx, tconn, ui)
		}, openHistogramName, closeHistogramName),
		perfutil.StoreAll(perf.BiggerIsBetter, "percent", name))

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
	// Close all the windows. Otherwise the test ends with browser windows
	// playing an animation, which is distracting during local development.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Error("Failed to get all open windows: ", err)
	}
	for _, w := range ws {
		w.CloseWindow(ctx, tconn) // Ignore errors.
	}
}
