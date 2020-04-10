// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WindowCycle,
		Desc: "Checks Alt+Tab and Alt+Shift+Tab functionality for cycling windows",
		Contacts: []string{
			"kyleshima@chromium.org",
			"bhansknecht@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// waitForWindowActiveAndFinishAnimating waits for the window specified with title to be active and no longer animating.
// Checking that the window is active seems to be the best indicator that it is focused and on top.
// Waiting for windows to finish animating is important when launching apps, since if an app takes
// longer to load completely, it can end up on top of applications launched later once the animation completes.
func waitForWindowActiveAndFinishAnimating(ctx context.Context, tconn *chrome.TestConn, windowID int) error {
	return ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsActive && w.IsAnimating == false && w.ID == windowID
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// WindowCycle verifies that we can cycle through open windows using Alt+Tab and Alt+Shift+Tab
func WindowCycle(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Window IDs are assigned by the autotestPrivate API based on launch order.
	// The first opened window gets ID -10000, the second -10001, the third -10002, etc...
	id := -10000
	for _, app := range []apps.App{apps.Chrome, apps.Files, apps.Settings} {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %v app: %v", app.Name, err)
		}
		if err := waitForWindowActiveAndFinishAnimating(ctx, tconn, id); err != nil {
			s.Fatalf("%v window (window ID: %v) not active after launching: %v", app.Name, id, err)
		}
		id--
	}

	// Window IDs in order of how they should appear in the cycle window.
	// Initially in reverse order of launch, since windows are ordered by most-recent.
	// This will be used to track the expected order of windows and updated after each Alt+Tab
	order := []int{-10002, -10001, -10000}

	// Get the keyboard
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// Index of the window we'll cycle to
	var target int

	// Cycle forwards (Alt + Tab) and backwards (Alt + Shift + Tab)
	for _, direction := range []string{"forward", "backward"} {
		// Press 'tab' 1, 2, 3, and 4 times to verify cycling behavior.
		// This verifies we can tab to all open windows, and checks the
		// cycling behavior since 4 tab presses will cycle back around.
		for i := 0; i < 4; i++ {
			func() {
				// Open cycle window and get app order
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

				if err := keyboard.Accel(ctx, "Tab"); err != nil {
					s.Fatal("Failed to press Tab: ", err)
				}

				// Verify that the cycle window appears in the UI with the right number of windows
				cycleWindow, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "WindowCycleList (Alt+Tab)"}, 5*time.Second)
				if err != nil {
					s.Fatal("Failed to get Alt+Tab cycle menu: ", err)
				}
				defer cycleWindow.Release(ctx)

				// Check that there are 3 windows in the cycle menu
				openApps, err := cycleWindow.Descendants(ctx, ui.FindParams{
					Role:  ui.RoleTypeWindow,
					State: map[ui.StateType]bool{ui.StateTypeFocusable: true},
				})
				if err != nil {
					s.Fatal("Failed to get open windows in the cycle menu: ", err)
				}
				defer openApps.Release(ctx)

				if len(openApps) != len(order) {
					s.Fatalf("Wrong number of apps in cycle window. Expected %v, got %v", len(order), len(openApps))
				}

				// Find the index of the window that is i tab presses away and cycle to it.
				if direction == "forward" {
					// The second window from the left (i.e. order[1]) is highlighted
					// after the first Tab press, and we advance by i windows, wrapping around to the front.
					target = (i + 1) % len(order)
				} else if direction == "backward" {
					// The rightmost window (i.e. order[len(order)-1]) is highlighted
					// after the first Shift+Tab press, and we go back by i windows.
					target = (len(order) - 1) - (i % len(order))
				}

				for j := 0; j < i; j++ {
					if err := keyboard.Accel(ctx, "Tab"); err != nil {
						s.Fatal("Failed to press Tab: ", err)
					}
				}
			}()

			if err := waitForWindowActiveAndFinishAnimating(ctx, tconn, order[target]); err != nil {
				s.Fatalf("Window (ID: %v) not focused after cycling to it: %v", target, err)
			}

			// The expected app order after cycling - the target window is moved to the front,
			// while the order of the other windows is preserved.
			tmp := []int{order[target]}
			order = append(tmp, append(order[:target], order[target+1:]...)...)
		}
	}
}
