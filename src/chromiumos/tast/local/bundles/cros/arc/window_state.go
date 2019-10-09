// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

// windowStateTest is used to represent a single window state transition test.
type windowStateTest struct {
	name                       string              // Name of test case
	initialWindowState         arc.WindowState     // Activity's initial window state
	expectedInitialWindowState ash.WindowStateType // Activity's expected, initial window state
	finalWindowState           arc.WindowState     // Activity's final window state
	expectedFinalWindowState   ash.WindowStateType // Activity's expected, final window state
}

// windowStateParams is used to represent a collection of tests to run in tablet mode or clamshell mode.
type windowStateParams struct {
	tabletMode bool              // True, if device should be in tablet mode
	tests      []windowStateTest // Activity's initial window state
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowState,
		Desc:         "Checks that ARC applications correctly change the window state",
		Contacts:     []string{"phshah@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "clamshell",
			Val: windowStateParams{
				false, // Clamshell mode
				[]windowStateTest{
					{"MAXIMIZE <--> FULLSCREEN", arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateFullscreen, ash.WindowStateFullscreen},
					{"MAXIMIZE <--> MINIMIZE", arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateMinimized, ash.WindowStateMinimized},
					{"MAXIMIZE <--> NORMAL", arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateNormal, ash.WindowStateNormal},
					{"FULLSCREEN <--> MINIMIZE", arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateMinimized, ash.WindowStateMinimized},
					{"FULLSCREEN <--> NORMAL", arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateNormal, ash.WindowStateNormal},
					{"NORMAL <--> MINIMIZE", arc.WindowStateNormal, ash.WindowStateNormal, arc.WindowStateMinimized, ash.WindowStateMinimized},
				},
			},
		}, {
			Name: "tablet",
			Val: windowStateParams{
				true, // Tablet Mode
				[]windowStateTest{
					{"MAXIMIZE <--> FULLSCREEN", arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateFullscreen, ash.WindowStateFullscreen},
					{"MAXIMIZE <--> MINIMIZE", arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateMinimized, ash.WindowStateMinimized},
					{"MAXIMIZE <--> NORMAL", arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateMaximized, ash.WindowStateMaximized},
					{"FULLSCREEN <--> MINIMIZE", arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateMinimized, ash.WindowStateMinimized},
					{"FULLSCREEN <--> NORMAL", arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateNormal, ash.WindowStateMaximized},
					{"NORMAL <--> MINIMIZE", arc.WindowStateNormal, ash.WindowStateMaximized, arc.WindowStateMinimized, ash.WindowStateMinimized},
				},
			},
		}},
	})
}

func WindowState(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Restore tablet mode to its original state on exit.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	// Start the Settings app.
	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the Settings activity: ", err)
	}

	if err := act.WaitForIdle(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to wait for idle activity: ", err)
	}

	// Number of window state transition tests.
	const numTestCount = 25

	testParams := s.Param().(windowStateParams)

	s.Logf("Setting tablet mode enabled to %t", testParams.tabletMode)
	if err := ash.SetTabletModeEnabled(ctx, tconn, testParams.tabletMode); err != nil {
		s.Fatalf("Failed to set tablet mode enabled to %t: %v", testParams.tabletMode, err)
	}

	// Run the different test cases.
	for _, test := range testParams.tests {
		// Set the activity to the initial WindowState.
		if err := act.SetWindowState(ctx, test.initialWindowState); err != nil {
			s.Fatalf("Failed to set the activity to the initial window state (%v): %v", test.initialWindowState, err)
		}

		if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), test.expectedInitialWindowState); err != nil {
			s.Fatal("Failed to wait for initial window state: ", err)
		}

		for i := 0; i < numTestCount; i++ {
			// First WindowState transition.
			if err := act.SetWindowState(ctx, test.initialWindowState); err != nil {
				s.Fatalf("Failed to set the activity to the first window state (%v) in iter %d: %v", test.initialWindowState, i, err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), test.expectedInitialWindowState); err != nil {
				s.Fatalf("Failed to wait for initial window state in iter %d: %v", i, err)
			}

			// Second WindowState transition.
			if err := act.SetWindowState(ctx, test.finalWindowState); err != nil {
				s.Fatalf("Failed to set the activity to the second window state (%v) in iter %d: %v", test.finalWindowState, i, err)
			}
			if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), test.expectedFinalWindowState); err != nil {
				s.Fatalf("Failed to wait for final window state in iter %d: %v", i, err)
			}
		}
	}
}
