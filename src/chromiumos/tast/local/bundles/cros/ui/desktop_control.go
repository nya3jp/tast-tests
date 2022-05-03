// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DesktopControl,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Check if the performance around desktop UI components is good enough; see also go/cros-ui-perftests-cq#heading=h.fwfk0yg3teo1",
		Contacts: []string{
			"newcomer@chromium.org",
			"tbarzic@chromium.org",
			"kaznacheev@chromium.org",
			"mukai@chromium.org", // Tast author
		},
		Attr:         []string{"group:mainline"},
		Fixture:      "chromeLoggedIn",
		SoftwareDeps: []string{"chrome", "no_chrome_dcheck"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{
			{
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(perfutil.UnstableModels...)),
			},
			// TODO(crbug.com/1163981): remove "unstable" once we see stability on all platforms.
			{
				Name:              "unstable",
				ExtraAttr:         []string{"informational"},
				ExtraHardwareDeps: hwdep.D(hwdep.Model(perfutil.UnstableModels...)),
			},
		},
	})
}

func DesktopControl(ctx context.Context, s *testing.State) {
	expects := perfutil.CreateExpectations(ctx,
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToCollapsed",
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToExpanded",
	)
	// When custom expectation value needs to be set, modify expects here.

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure into the clamshell mode: ", err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := ash.CreateWindows(ctx, tconn, cr, "", 2); err != nil {
		s.Fatal("Failed to create new windows: ", err)
	}

	// This test assumes shelf visibility, setting the shelf behavior explicitly.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find the primary display info: ", err)
	}
	shelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, info.ID)
	if err != nil {
		s.Fatal("Failed to get the shelf behavior for display ID ", info.ID)
	}
	if shelfBehavior != ash.ShelfBehaviorNeverAutoHide {
		if err := ash.SetShelfBehavior(ctx, tconn, info.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
			s.Fatal("Failed to sete the shelf behavior to 'never auto-hide' for display ID ", info.ID)
		}
	}

	// Turn all windows into normal state.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateNormal)
	}); err != nil {
		s.Fatal("Failed to set all windows as normal state: ", err)
	}

	r := perfutil.NewRunner(cr.Browser())
	r.Runs = 3
	r.RunTracing = false

	// TODO(crbug.com/1321838): After feature ProductivityLauncher is enabled by
	// default, add a test that opens and closes the launcher and checks
	// animation performance with the histograms:
	// - Apps.ClamshellLauncher.AnimationSmoothness.OpenAppsPage
	// - Apps.ClamshellLauncher.AnimationSmoothness.Close
	// These will need to be added to `expects` above.

	// Controls of the quick settings:
	// - open the quick settings
	// - click the collapse button to shrink the quick settings
	// - click the collapse button again to expand the quick settings
	// - close the quick settings
	s.Log("Shrink and expand the quick settings")
	r.RunMultiple(ctx, s, "quick-settings", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// The option to be used for StableLeftClick and WaitLocationStable. Also,
		// it uses a longer interval (default interval is 100msecs), as the location
		// update may not happen very quickly.
		waitingOption := testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}

		uiBase := uiauto.New(tconn)
		ui := uiBase.WithPollOpts(waitingOption)
		statusarea := nodewith.ClassName("ash/StatusAreaWidgetDelegate")
		collapseButton := nodewith.ClassName("CollapseButton")

		if err := uiauto.Combine(
			"quick-settings open and close",
			// Seems that the quick settings open/close don't have metrics, but in
			// case that's added in the future, it is noted here.
			ui.WaitUntilExists(statusarea),
			ui.LeftClick(statusarea),
			// Wait for the collapse button to exist, for 20 seconds.
			uiBase.WithTimeout(20*time.Second).WaitUntilExists(collapseButton),
			// Click the collapse button to shrink.
			ui.LeftClick(collapseButton),
			// Click the collapse button to expand.
			ui.LeftClick(collapseButton),
			// Wait for the quick settings to be fully expanded.
			ui.WaitForLocation(collapseButton),
			// Close the quick settings by clicking the status area again.
			ui.LeftClick(statusarea))(ctx); err != nil {
			return errors.Wrap(err, "failed to proceed the test scenario")
		}

		// Right now there's no way to identify the quick settings is closed.
		// Just wait 1 second. TODO(crbug.com/1099502): replace by a test API.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for the quick settings to be closed")
		}
		return nil
	},
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToCollapsed",
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToExpanded",
	), perfutil.StoreAllWithHeuristics(""))

	// TODO(crbug.com/1141164): add notification-related metrics once it's
	// supported.

	// Check the validity of histogram data.
	for _, err := range r.Values().Verify(ctx, expects) {
		s.Error("Performance expectation missed: ", err)
	}
	// Storing the results for the future analyses.
	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed to save the values: ", err)
	}
}
