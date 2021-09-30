// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WindowCycle,
		Desc: "Checks Alt+Tab and Alt+Shift+Tab functionality for cycling windows",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// waitForWindowActiveAndFinishAnimating waits for the window specified with title to be active and no longer animating.
// Checking that the window is active seems to be the best indicator that it is focused and on top.
func waitForWindowActiveAndFinishAnimating(ctx context.Context, tconn *chrome.TestConn, windowID int) error {
	return ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsActive && w.IsAnimating == false && w.ID == windowID
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// WindowCycle verifies that we can cycle through open windows using Alt+Tab and Alt+Shift+Tab.
func WindowCycle(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Launch the apps for the test. Different launch functions are used for Settings and Files,
	// since they include additional waiting for UI elements to load. This prevents the window
	// order from changing while cycling. If a window has not completely loaded, it may appear on
	// top once it finishes loading. If this happens while cycling windows, the test will likely fail.
	const numApps = 3
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatal("Failed to launch Chrome: ", err)
	}

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

	// GetAllWindows returns windows in the order of most recent, matching the order they will appear in the cycle window.
	// We'll use this slice to find the expected window that should be brought to the top after each Alt+Tab cycle,
	// based on the number of times Tab was pressed. After successfully cycling windows, we'll update this to reflect
	// the expected order of the windows for the next Alt+Tab cycle.
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get windows: ", err)
	}
	if len(windows) != numApps {
		s.Fatalf("Unexpected number of windows open; wanted %v, got %v", numApps, len(windows))
	}

	// Get the keyboard
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// Index of the window we'll cycle to.
	var target int

	// Params for the window cycle menu.
	cycleMenuParams := ui.FindParams{ClassName: "WindowCycleList (Alt+Tab)"}

	// Cycle forwards (Alt + Tab) and backwards (Alt + Shift + Tab).
	for _, direction := range []string{"forward", "backward"} {
		s.Log("Cycle direction: ", direction)
		// Press 'tab' 1, 2, 3, and 4 times to verify cycling behavior.
		// This verifies we can tab to all open windows, and checks the
		// cycling behavior since 4 tab presses will cycle back around.
		for i := 0; i < 4; i++ {
			func() {
				// Make sure the cycle menu isn't open already before we try to alt+tab.
				if err := ui.WaitUntilGone(ctx, tconn, cycleMenuParams, 5*time.Second); err != nil {
					s.Fatal("Cycle menu unexpectedly open before pressing alt+tab: ", err)
				}

				// Open cycle window and get app order.
				if err := keyboard.AccelPress(ctx, "Alt"); err != nil {
					s.Fatal("Failed to long press Alt: ", err)
				}
				defer keyboard.AccelRelease(ctx, "Alt")

				if direction == "backward" {
					if err := keyboard.AccelPress(ctx, "Shift"); err != nil {
						s.Fatal("Failed to long press Shift: ", err)
					}
					defer keyboard.AccelRelease(ctx, "Shift")
				}

				testing.Sleep(ctx, 500*time.Millisecond)

				if err := keyboard.Accel(ctx, "Tab"); err != nil {
					s.Fatal("Failed to press Tab: ", err)
				}

				// Verify that the cycle window appears in the UI with the right number of windows.
				cycleWindow, err := ui.FindWithTimeout(ctx, tconn, cycleMenuParams, 5*time.Second)
				if err != nil {
					s.Fatal("Failed to get Alt+Tab cycle menu: ", err)
				}
				defer cycleWindow.Release(ctx)

				// Check that there are 3 windows in the cycle menu.
				openApps, err := cycleWindow.Descendants(ctx, ui.FindParams{
					Role:  ui.RoleTypeWindow,
					State: map[ui.StateType]bool{ui.StateTypeFocusable: true},
				})
				if err != nil {
					s.Fatal("Failed to get open windows in the cycle menu: ", err)
				}
				defer openApps.Release(ctx)

				if len(openApps) != len(windows) {
					s.Fatalf("Wrong number of apps in cycle window. Expected %v, got %v", len(windows), len(openApps))
				}
				s.Log("Number of apps in the cycle window matches number of open apps")

				// Find the index of the window that is i tab presses away and cycle to it.
				if direction == "forward" {
					// The second window from the left (i.e. windows[1]) is highlighted
					// after the first Tab press, and we advance by i windows, wrapping around to the front.
					target = (i + 1) % len(windows)
				} else if direction == "backward" {
					// The rightmost window (i.e. windows[len(windows)-1]) is highlighted
					// after the first Shift+Tab press, and we go back by i windows.
					target = (len(windows) - 1) - (i % len(windows))
				}

				for j := 0; j < i; j++ {
					if err := keyboard.Accel(ctx, "Tab"); err != nil {
						s.Fatal("Failed to press Tab: ", err)
					}
				}
			}()

			if err := waitForWindowActiveAndFinishAnimating(ctx, tconn, windows[target].ID); err != nil {
				s.Fatalf("Window (ID: %v) not focused after cycling to it: %v", windows[target].ID, err)
			}

			// The expected app order after cycling - the target window is moved to the front,
			// while the order of the other windows is preserved.
			tmp := []*ash.Window{windows[target]}
			windows = append(tmp, append(windows[:target], windows[target+1:]...)...)
		}
	}
}
