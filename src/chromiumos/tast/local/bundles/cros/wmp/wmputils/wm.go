// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmputils contains utility functions for wmp tests.
package wmputils

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	timeout  = 30 * time.Second
	interval = time.Second
)

// EnsureOnlyBrowserWindowOpen ensures that there is only one open window that is the primary browser, and waits for the browser to be visible to avoid a race that may cause test flakiness.
// If there is no or more than one browser window(s) open, it throws an error.
func EnsureOnlyBrowserWindowOpen(ctx context.Context, tconn *chrome.TestConn, bt browser.Type) (*ash.Window, error) {
	var w *ash.Window
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if there is only one open window.
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the window list"))
		}
		if len(ws) != 1 {
			return errors.Errorf("expected 1 window, got %v", len(ws))
		}

		// Check if that is the browser window and visible (!IsAnimating also used as heuristic criteria for readiness to accept inputs).
		w = ws[0]
		if !w.IsVisible || w.IsAnimating || !ash.BrowserTypeMatch(bt)(w) {
			return errors.Errorf("expected %v browser window to become visible, State: %v", bt, w.State)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: interval}); err != nil {
		return nil, errors.Wrap(err, "expected 1 visible browser window")
	}
	return w, nil
}

// VerifyWindowCount verifies that there are `windowCount` app windows.
func VerifyWindowCount(ctx context.Context, tconn *chrome.TestConn, windowCount int) error {
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get all open windows")
	}

	if len(ws) != windowCount {
		// TODO(b/241118477): Remove exiting out of the location access popup step
		// once there is a more permanent fix to disable the popup. The popup is not
		// captured by the UI tree, so it is unable to interact with it using uiauto
		// libraries. In order to remove this popup window,
		// we will need to input keyboard commands.
		if len(ws) == windowCount+1 {
			// We need to press the enter key twice.
			// Once to get get the focus onto the popup window
			// and once to confirm a choice to exit the window.
			kb, err := input.Keyboard(ctx)
			if err != nil {
				return errors.Wrap(err, "cannot create keyboard")
			}
			defer kb.Close()
			if err := kb.Accel(ctx, "Enter"); err != nil {
				return errors.Wrap(err, "cannot press 'Enter'")
			}
			if err := kb.Accel(ctx, "Enter"); err != nil {
				return errors.Wrap(err, "cannot press 'Enter'")
			}
		}
		// Verify that there is now the correct number of windows.
		wc, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get all open windows")
		}
		if len(wc) != windowCount {
			return errors.Wrapf(err, "found inconsistent number of window(s): got %v, want %v", len(wc), windowCount)
		}
	}

	return nil
}

// WaitforAppsToLaunch waits for the given apps to launch.
func WaitforAppsToLaunch(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, appsList []apps.App) error {
	for _, app := range appsList {
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			return errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
		}

		// Some apps may take a long time to load such as Play Store. Wait for launch event to be completed.
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for the app launch event to be completed")
		}
	}

	if err := VerifyWindowCount(ctx, tconn, len(appsList)); err != nil {
		return errors.Wrap(err, "failed to verify window count")
	}

	return nil
}

// WaitforAppsToBeVisible waits for the windows of the given apps to be visible.
func WaitforAppsToBeVisible(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, appsList []apps.App) error {
	for _, app := range appsList {
		// Wait for the launched app window to become visible.
		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			if !w.IsVisible {
				return false
			}
			// The window title of Lacros is suffixed with "Chrome", not "Lacros".
			if w.WindowType == ash.WindowTypeLacros {
				return strings.Contains(w.Title, "Chrome")
			}
			return strings.Contains(w.Title, app.Name)
		}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
			return errors.Wrapf(err, "%s app window not visible after launching", app.Name)
		}
	}

	return nil
}

// OpenApps opens the given apps, waits for them to launch and their windows to appear..
func OpenApps(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, appsList []apps.App) error {
	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			return errors.Wrapf(err, "failed to open %s", app.Name)
		}
	}

	if err := WaitforAppsToLaunch(ctx, tconn, ac, appsList); err != nil {
		return errors.Wrap(err, "failed to wait for app launch")
	}

	return nil
}

// waitForWindowActiveAndFinishAnimating waits for the window specified with title to be active and no longer animating.
// Checking that the window is active seems to be the best indicator that it is focused and on top.
func waitForWindowActiveAndFinishAnimating(ctx context.Context, tconn *chrome.TestConn, windowID int) error {
	return ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.OnActiveDesk && w.IsActive && w.IsVisible && !w.IsAnimating && w.ID == windowID
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

func openCycleWindowAndPressTabNTimes(ctx context.Context, ac *uiauto.Context, direction string, numWindows, n int) error {
	// Finder for the window cycle menu when there is only one desk.
	cycleMenu := nodewith.ClassName("WindowCycleView")
	// Get the keyboard
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer keyboard.Close()

	// Make sure the cycle menu isn't open already before we try to alt+tab.
	if err := ac.WithTimeout(5 * time.Second).WaitUntilGone(cycleMenu)(ctx); err != nil {
		return errors.Wrap(err, "cycle menu unexpectedly open before pressing Alt+Tab")
	}

	// Open cycle window and get app order.
	if err := keyboard.AccelPress(ctx, "Alt"); err != nil {
		return errors.Wrap(err, "failed to long press Alt")
	}
	defer keyboard.AccelRelease(ctx, "Alt")

	if direction == "backward" {
		if err := keyboard.AccelPress(ctx, "Shift"); err != nil {
			return errors.Wrap(err, "failed to long press Shift")
		}
		defer keyboard.AccelRelease(ctx, "Shift")
	}

	if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to sleep before press tab to open Alt+Tab window")
	}

	if err := keyboard.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "failed to press Tab")
	}

	// Verify that the cycle window appears in the UI with the right number of windows.
	if err := ac.WithTimeout(5 * time.Second).WaitUntilExists(cycleMenu)(ctx); err != nil {
		return errors.Wrap(err, "failed to get Alt+Tab cycle menu")
	}

	// Check that there are `numApps` windows in the cycle menu.
	openApps, err := ac.NodesInfo(ctx, nodewith.Role(role.Window).Focusable().Ancestor(cycleMenu))
	if err != nil {
		return errors.Wrap(err, "failed to get open windows in the cycle menu")
	}

	if len(openApps) != numWindows {
		return errors.Errorf("Wrong number of apps in cycle window. Expected %v, got %v", numWindows, len(openApps))
	}

	for j := 0; j < n; j++ {
		if err := keyboard.Accel(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to press Tab")
		}
	}
	return nil
}

// VerifyWindowsForCycleMenu opens Alt+Tab window and Press Tab to cycle all windows.
func VerifyWindowsForCycleMenu(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, windows []*ash.Window) error {
	// Get the number of windows.
	numWindows := len(windows)
	// Index of the window we'll cycle to.
	var target int

	// Cycle forwards (Alt + Tab) and backwards (Alt + Shift + Tab).
	for _, direction := range []string{"forward", "backward"} {
		// Press 'tab' 1, 2, 3, ..., until `numWindows` times to verify cycling behavior.
		// This verifies we can tab to all open windows, and checks the
		// cycling behavior since numWindows+1 tab presses will cycle back around.
		for i := 0; i < numWindows+1; i++ {
			if err := openCycleWindowAndPressTabNTimes(ctx, ac, direction, numWindows, i); err != nil {
				return errors.Wrap(err, "failed to open Alt+Tab window and press n times Tab")
			}

			// Find the index of the window that is i tab presses away and cycle to it.
			if direction == "forward" {
				// The second window from the left (i.e. windows[1]) is highlighted
				// after the first Tab press, and we advance by i windows, wrapping around to the front.
				target = (i + 1) % numWindows
			} else if direction == "backward" {
				// The rightmost window (i.e. windows[len(windows)-1]) is highlighted
				// after the first Shift+Tab press, and we go back by i windows.
				target = (numWindows - 1) - (i % numWindows)
			}
			if err := waitForWindowActiveAndFinishAnimating(ctx, tconn, windows[target].ID); err != nil {
				return errors.Wrapf(err, "window (ID: %v) not focused after cycling to it", windows[target].ID)
			}

			// The expected app order after cycling - the target window is moved to the front,
			// while the order of the other windows is preserved.
			tmp := []*ash.Window{windows[target]}
			windows = append(tmp, append(windows[:target], windows[target+1:]...)...)
		}
	}
	return nil
}
