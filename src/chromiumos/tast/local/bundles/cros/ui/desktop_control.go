// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DesktopControl,
		Desc: "Check if the performance around desktop UI components is good enough; see also go/cros-ui-perftests-cq#heading=h.fwfk0yg3teo1",
		Contacts: []string{
			"newcomer@chromium.org",
			"tbarzic@chromium.org",
			"kaznacheev@chromium.org",
			"mukai@chromium.org", // Tast author
		},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          chrome.LoggedIn(),
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
	})
}

func DesktopControl(ctx context.Context, s *testing.State) {
	expects := perfutil.CreateExpectations(ctx,
		"Apps.StateTransition.AnimationSmoothness.Peeking.ClamshellMode",
		"Apps.StateTransition.AnimationSmoothness.Half.ClamshellMode",
		"Apps.StateTransition.AnimationSmoothness.FullscreenSearch.ClamshellMode",
		"Apps.StateTransition.AnimationSmoothness.FullscreenAllApps.ClamshellMode",
		"Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode",
		"Apps.StateTransition.Drag.PresentationTime",
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToCollapsed",
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToExpanded",
	)
	// When custom expectation value needs to be set, modify expects here.

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure into the clamshell mode: ", err)
	}
	defer cleanup(ctx)

	conns, err := ash.CreateWindows(ctx, tconn, cr, "", 2)
	if err != nil {
		s.Fatal("Failed to create new windows: ", err)
	}
	if err := conns.Close(); err != nil {
		s.Fatal("Failed to close the connections: ", err)
	}

	// Turn all windows into normal state.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateNormal)
	}); err != nil {
		s.Fatal("Failed to set all windows as normal state: ", err)
	}

	r := perfutil.NewRunner(cr)
	r.Runs = 3
	r.RunTracing = false

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to take keyboard: ", err)
	}

	// Open the launcher, enter a search query, expands to the fullscreen, closes.
	s.Log("Open/close the launcher")
	r.RunMultiple(ctx, s, "launcher", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		for i, action := range []struct {
			key         string
			isAccel     bool
			targetState ash.LauncherState
		}{
			{"Search", true, ash.Peeking},                // Search key to open the launcher as peeking
			{"lorem ipsum", false, ash.Half},             // Type some query
			{"Shift+Search", true, ash.FullscreenSearch}, // Shift+Search to turn into fullsceen
			{"Esc", true, ash.FullscreenAllApps},         // Esc to cancel the query
			{"Esc", true, ash.Closed},                    // Esc to close.
		} {
			if action.isAccel {
				if err := kw.Accel(ctx, action.key); err != nil {
					return errors.Wrapf(err, "failed to type key %q at step %d", action.key, i)
				}
			} else {
				if err := kw.Type(ctx, action.key); err != nil {
					return errors.Wrapf(err, "failed to type %q at step %d", action.key, i)
				}
			}
			if err := ash.WaitForLauncherState(ctx, tconn, action.targetState); err != nil {
				return errors.Wrapf(err, "the launcher isn't in the state %q at step %d", action.targetState, i)
			}
		}
		return nil
	},
		"Apps.StateTransition.AnimationSmoothness.Peeking.ClamshellMode",
		"Apps.StateTransition.AnimationSmoothness.Half.ClamshellMode",
		"Apps.StateTransition.AnimationSmoothness.FullscreenSearch.ClamshellMode",
		"Apps.StateTransition.AnimationSmoothness.FullscreenAllApps.ClamshellMode",
		"Apps.StateTransition.AnimationSmoothness.Close.ClamshellMode",
	), perfutil.StoreAllWithHeuristics)

	// Open the launcher by drag.
	s.Log("Open/close the launcher by drag")
	// Assumes that there's only the chrome icon in the shelf, which is the
	// default status of chrome.LoggedIn precondition. Just use 1/4 of the screen
	// width can avoid any icons on the shelf and savely drag the launcher.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info")
	}
	dragStart := coords.NewPoint(info.Bounds.Left+info.Bounds.Width/4, info.Bounds.Bottom()-1)
	dragEnd := coords.NewPoint(dragStart.X, info.Bounds.Top+1)
	r.RunMultiple(ctx, s, "launcher-drag", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		if err := mouse.Drag(ctx, tconn, dragStart, dragEnd, time.Second); err != nil {
			return errors.Wrap(err, "failed to drag to open the launcher")
		}
		if err := ash.WaitForLauncherState(ctx, tconn, ash.FullscreenAllApps); err != nil {
			return errors.Wrap(err, "launcher isn't in fullscreen")
		}
		if err := mouse.Drag(ctx, tconn, dragEnd, dragStart, time.Second); err != nil {
			return errors.Wrap(err, "failed to drag to close the launcher")
		}
		if err := ash.WaitForLauncherState(ctx, tconn, ash.Closed); err != nil {
			return errors.Wrap(err, "launcher isn't closed")
		}
		return nil
	},
		"Apps.StateTransition.Drag.PresentationTime.ClamshellMode",
	), perfutil.StoreAllWithHeuristics)

	// Open the quick settings, shrink -> expand.
	s.Log("Shrink and expand the quick settings")
	r.RunMultiple(ctx, s, "quick-settings", perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		releaseCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, time.Second)
		defer cancel()

		// Seems that the quick settings open/close don't have metrics, but in case
		// that's added in the future, it is noted here.
		statusArea, err := ui.Find(ctx, tconn, ui.FindParams{ClassName: "ash/StatusAreaWidgetDelegate"})
		if err != nil {
			return errors.Wrap(err, "failed to find the status area")
		}
		if err := statusArea.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click the status area")
		}
		defer statusArea.Release(releaseCtx)

		collapseButton, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "CollapseButton"}, 10*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find the collapse button, possibly quick settings is not open yet")
		}
		defer collapseButton.Release(releaseCtx)

		// Click the collapse button to collapse.
		if err := collapseButton.StableLeftClick(ctx, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to click the collapse button")
		}

		// Click the collapse button again to expand.
		if err := collapseButton.StableLeftClick(ctx, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to click the collapse button")
		}

		if err := collapseButton.WaitLocationStable(ctx, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for the collapse button location to be stabilized")
		}

		// Close the quick settings by clicking the status area again.
		if err := statusArea.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click the status area")
		}

		// Right now there's no way to identify the quick settings is closed.
		// Just wait 1 second. TODO(): replace by a test API.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait for the quick settings to be closed")
		}
		return nil
	},
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToCollapsed",
		"ChromeOS.SystemTray.AnimationSmoothness.TransitionToExpanded",
	), perfutil.StoreAllWithHeuristics)

	// TODO(): add notification-related metrics once it's supported.

	// Check the validity of histogram data.
	for _, err := range r.Values().Verify(ctx, expects) {
		s.Error("Performance expectation missed: ", err)
	}
	// Storing the results for the future analyses.
	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed to save the values: ", err)
	}
}
