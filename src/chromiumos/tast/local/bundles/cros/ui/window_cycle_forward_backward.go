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
		Func: WindowCycleForwardBackward,
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

// waitForWindowActive waits for the window specified with title to be active.
// Checking that the window is active seems to be the best indicator that it is focused and on top.
func waitForWindowActive(ctx context.Context, tconn *chrome.TestConn, title string) error {
	return ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsActive && w.Title == title
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// WindowCycleForwardBackward verifies that we can cycle through open windows using Alt+Tab and Alt+Shift+Tab
func WindowCycleForwardBackward(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	checkApps := map[string]apps.App{
		"Chrome - New Tab": apps.Chrome,
		"Files - My files": apps.Files,
		"Settings":         apps.Settings,
	}

	for title, app := range checkApps {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %v app: %v", app.Name, err)
		}
		if err := waitForWindowActive(ctx, tconn, title); err != nil {
			s.Fatalf("%v window not active after launching: %v", title, err)
		}
	}

	// Get the keyboard
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// Cycle forwards (Alt + Tab) and backwards (Alt + Shift + Tab)
	for _, direction := range []string{"forward", "backward"} {
		for i := 0; i < 3; i++ { // press 'tab' 1, 2, and 3 times to verify cycling behavior
			// Open cycle window and get app order
			if err := keyboard.AccelPress(ctx, "Alt"); err != nil {
				s.Fatal("Failed to long press Alt: ", err)
			}
			defer keyboard.AccelRelease(ctx, "Alt") // in case test fails before we release the key

			if direction == "backward" {
				if err := keyboard.AccelPress(ctx, "Shift"); err != nil {
					s.Fatal("Failed to long press Shift: ", err)
				}
				defer keyboard.AccelRelease(ctx, "Shift")
			}

			if err := keyboard.Accel(ctx, "Tab"); err != nil {
				s.Fatal("Failed to press Tab: ", err)
			}

			cycleWindow, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "WindowCycleList (Alt+Tab)"}, 5*time.Second)
			if err != nil {
				s.Fatal("Failed to get Alt+Tab cycle menu: ", err)
			}
			defer cycleWindow.Release(ctx)

			openApps, err := cycleWindow.Descendants(ctx, ui.FindParams{
				Role:  ui.RoleTypeWindow,
				State: map[ui.StateType]bool{ui.StateTypeFocusable: true},
			})
			if err != nil {
				s.Fatal("Failed to get open windows in the cycle menu: ", err)
			}
			defer openApps.Release(ctx)

			// All 3 open apps should be present in the cycle menu
			openAppNames := make(map[string]bool)
			for j := 0; j < len(openApps); j++ {
				openAppNames[openApps[j].Name] = true
			}
			for title := range checkApps {
				if _, present := openAppNames[title]; !present {
					s.Fatalf("Failed to find %v window in cycle menu: %v", title, err)
				}
			}

			// Find the window that is i tab presses away and cycle to it.
			var target string
			if direction == "forward" {
				// The second window from the left (i.e. openApps[1]) is highlighted
				// after the first Tab press, and we advance by i windows, wrapping around to the front.
				target = openApps[(i+1)%len(openApps)].Name
			} else if direction == "backward" {
				// The rightmost window (i.e. openApps[len(openApps)-1]) is highlighted
				// after the first Shift+Tab press, and we go back by i windows.
				target = openApps[(len(openApps)-1)-i].Name
			}

			for j := 0; j < i; j++ {
				if err := keyboard.Accel(ctx, "Tab"); err != nil {
					s.Fatal("Failed to press Tab: ", err)
				}
			}

			if err := keyboard.AccelRelease(ctx, "Alt"); err != nil {
				s.Fatal("Failed to release Alt key: ", err)
			}

			if direction == "backward" {
				if err := keyboard.AccelRelease(ctx, "Shift"); err != nil {
					s.Fatal("Failed to release Shift: ", err)
				}
			}

			if err := waitForWindowActive(ctx, tconn, target); err != nil {
				s.Fatalf("%v window not focused after cycling to it: %v", target, err)
			}
		}
	}
}
