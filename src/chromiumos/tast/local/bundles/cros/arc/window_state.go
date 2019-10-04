// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

type windowStateTestInfo struct {
	name                       string          // Name of test case
	initialWindowState         arc.WindowState // Activity's initial window state
	expectedInitialWindowState arc.WindowState // Activity's expected, initial window state
	finalWindowState           arc.WindowState // Activity's final window state
	expectedFinalWindowState   arc.WindowState // Activity's expected, final window state
}

type windowStateTest struct {
	tabletMode bool                  // True, if device should be in tablet mode
	tests      []windowStateTestInfo // Activity's initial window state
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowState,
		Desc:         "Checks that ARC applications correctly change the window state",
		Contacts:     []string{"phshah@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "clamshell",
			Val: windowStateTest{
				false, // Clamshell mode
				[]windowStateTestInfo{
					{"MAXIMIZE <--> FULLSCREEN", arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateFullscreen, arc.WindowStateFullscreen},
					{"MAXIMIZE <--> MINIMIZE", arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized},
					{"MAXIMIZE <--> NORMAL", arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateNormal, arc.WindowStateNormal},
					{"FULLSCREEN <--> MINIMIZE", arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateMinimized, arc.WindowStateMinimized},
					{"FULLSCREEN <--> NORMAL", arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateNormal, arc.WindowStateNormal},
					{"NORMAL <--> MINIMIZE", arc.WindowStateNormal, arc.WindowStateNormal, arc.WindowStateMinimized, arc.WindowStateMinimized},
				},
			},
		}, {
			Name: "tablet",
			Val: windowStateTest{
				true, // Tablet Mode
				[]windowStateTestInfo{
					{"MAXIMIZE <--> FULLSCREEN", arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateFullscreen, arc.WindowStateFullscreen},
					{"MAXIMIZE <--> MINIMIZE", arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized},
					{"MAXIMIZE <--> NORMAL", arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMaximized},
					{"FULLSCREEN <--> MINIMIZE", arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateMinimized, arc.WindowStateMinimized},
					{"FULLSCREEN <--> NORMAL", arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateNormal, arc.WindowStateMaximized},
					{"NORMAL <--> MINIMIZE", arc.WindowStateNormal, arc.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized},
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

	// Number of window state transition tests.
	const numTestCount = 25

	testOpts := s.Param().(windowStateTest)

	s.Logf("Setting tablet mode enabled to %t", testOpts.tabletMode)
	if err := ash.SetTabletModeEnabled(ctx, tconn, testOpts.tabletMode); err != nil {
		s.Fatalf("Failed to set tablet mode enabled to %t: %v", testOpts.tabletMode, err)
	}

	// Run the different test cases.
	for _, test := range testOpts.tests {

		// Set the activity to the initial WindowState.
		if err := act.SetWindowState(ctx, test.initialWindowState); err != nil {
			s.Fatalf("Failed to set the activity to the initial window state (%v): %v", test.initialWindowState, err)
		}
		if err := act.WaitForIdle(ctx, 4*time.Second); err != nil {
			s.Fatal("Failed to wait for idle activity: ", err)
		}
		if err := verifyActivityWindowState(ctx, act, test.expectedInitialWindowState); err != nil {
			s.Fatal("Failed to verify the initial window state: ", err)
		}

		for i := 0; i < numTestCount; i++ {
			// First WindowState transition.
			if err := act.SetWindowState(ctx, test.initialWindowState); err != nil {
				s.Fatalf("Failed to set the activity to the first window state (%v) in iter %d: %v", test.initialWindowState, i, err)
			}
			if err := act.WaitForIdle(ctx, 4*time.Second); err != nil {
				s.Fatal("Failed to wait for idle activity: ", err)
			}
			if err := verifyActivityWindowState(ctx, act, test.expectedInitialWindowState); err != nil {
				s.Fatalf("Failed to verify the first window state in iter %d: %v", i, err)
			}

			// Second WindowState transition.
			if err := act.SetWindowState(ctx, test.finalWindowState); err != nil {
				s.Fatalf("Failed to set the activity to the second window state (%v) in iter %d: %v", test.finalWindowState, i, err)
			}
			if err := act.WaitForIdle(ctx, 4*time.Second); err != nil {
				s.Fatal("Failed to wait for idle activity: ", err)
			}
			if err := verifyActivityWindowState(ctx, act, test.expectedFinalWindowState); err != nil {
				s.Fatalf("Failed to verify the second window state in iter %d: %v", i, err)
			}
		}
	}
}

// verifyActivityWindowState verifies that the activity's current window state is the expected window state.
func verifyActivityWindowState(ctx context.Context, act *arc.Activity, expected arc.WindowState) error {
	actualWindowState, err := act.GetWindowState(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get window state")
	}
	if actualWindowState != expected {
		return errors.Errorf("unexpected window state: got %v; want %v", actualWindowState, expected)
	}
	return nil
}
