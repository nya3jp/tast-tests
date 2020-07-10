// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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

	// Launch the apps for the test
	checkApps := []apps.App{apps.Chrome, apps.Files, apps.Settings}
	for _, app := range checkApps {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %v app: %v", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed waiting for %v app: %v", app.Name, err)
		}
	}

	// Wait for all windows to be open and stop animating.
	// This ensures that the order of the apps is finalized before we begin alt+tabbing.
	// Without doing so, the window order may change unexpectedly due to windows loading
	// at different rates, causing the actual window focused by alt+tab to be different
	// than what the test expects.
	// Note: the combination of apps.Launch and ash.WaitForApp does not guarantee the
	// app window will be open and finalized, so this poll is needed.
	if testing.Poll(ctx, func(ctx context.Context) error {
		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			testing.PollBreak(err)
		}
		if len(windows) != len(checkApps) {
			return errors.New("new window not yet found")
		}
		for _, w := range windows {
			if w.IsAnimating {
				return errors.New("window still animating")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Expected windows did not open and finish animating: ", err)
	}

	// GetAllWindows returns windows in the order of most recent, matching the order they will appear in the cycle window.
	// We'll use this slice to find the expected window that should be brought to the top after each Alt+Tab cycle,
	// based on the number of times Tab was pressed. After successfully cycling windows, we'll update this to reflect
	// the expected order of the windows for the next Alt+Tab cycle.
	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get windows: ", err)
	}

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
		s.Log("Cycle direction: ", direction)
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

				testing.Sleep(ctx, 500*time.Millisecond)

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
