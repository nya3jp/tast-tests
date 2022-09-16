// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowCycleAllDesks,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks Alt+Tab and Alt+Shift+Tab functionality for cycling windows for all desks",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"chromeos-wmp@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "no_kernel_upstream"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func WindowCycleAllDesks(ctx context.Context, s *testing.State) {
	// Reserve for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	// Launch the apps for the test. Different launch functions are used for Settings and Files,
	// since they include additional waiting for UI elements to load. This prevents the window
	// order from changing while cycling. If a window has not completely loaded, it may appear on
	// top once it finishes loading. If this happens while cycling windows, the test will likely fail.
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Settings: ", err)
	}
	if err := settings.WaitForSearchBox()(ctx); err != nil {
		s.Fatal("Failed waiting for Settings to load: ", err)
	}

	if _, err := filesapp.Launch(ctx, tconn); err != nil {
		s.Fatal("Failed to launch Files app: ", err)
	}

	ac := uiauto.New(tconn)
	// GetAllWindows returns windows in the order of most recent, matching the order they will appear in the cycle window.
	// We'll use this slice to find the expected window that should be brought to the top after each Alt+Tab cycle,
	// based on the number of times Tab was pressed. After successfully cycling windows, we'll update this to reflect
	// the expected order of the windows for the next Alt+Tab cycle.
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows")
	}

	// 1. Tests that when there is only one desk with multiple browsers/apps, we can press alt-tab to cycle all windows.
	if err := verifyWindowsForCycleMenu(ctx, tconn, ac, windows); err != nil {
		s.Fatal(err, "Failed to cycle windows for the only one desk.")
	}

	// 2. Tests that when there are 8 desks and each desk has at least one browser/app, we can cycle all the apps/browsers
	// that are opened in all desks when the default status of the alt-tab window is "All desks".
	// Add 7 desks for a total of 8. Remove them at the end of the test.
	totalDesks := 8
	for i := 1; i < totalDesks; i++ {
		if err = ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Error("Failed to create a new desk: ", err)
		}

		// Active the new created desk.
		if err = ash.ActivateDeskAtIndex(ctx, tconn, i); err != nil {
			s.Fatalf("Failed to activate desk with index %d: %v", i, err)
		}
		// Open one browser on the desk.
		if err := createBrowser(ctx, tconn); err != nil {
			s.Fatalf("Failed to create browser for desk with index %d: %v", i, err)
		}
	}

	windows, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows: ", err)
	}
	// Cycle windows for all 8 desks.
	if err := verifyWindowsForCycleMenu(ctx, tconn, ac, windows); err != nil {
		s.Fatal("Failed to cycle all desks windows: ", err)
	}
}

func createBrowser(ctx context.Context, tconn *chrome.TestConn) error {
	// Open one browser on the desk.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not find browser app info")
	}
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		return errors.Wrap(err, "failed to launch browser")
	}
	if err := ash.WaitForApp(ctx, tconn, browserApp.ID, time.Minute); err != nil {
		return errors.Wrap(err, "browser did not appear in shelf after launch")
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

func verifyWindowsForCycleMenu(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, windows []*ash.Window) error {
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
