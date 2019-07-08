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

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowState,
		Desc:         "Checks that ARC applications correctly change the window state",
		Contacts:     []string{"phshah@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      5 * time.Minute,
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

	// Run the different test cases.
	for _, test := range []struct {
		name                       string
		tabletMode                 bool
		initialWindowState         arc.WindowState
		expectedInitialWindowState arc.WindowState
		finalWindowState           arc.WindowState
		expectedFinalWindowState   arc.WindowState
	}{
		// Clamshell Mode.
		{"MAXIMIZE <--> FULLSCREEN", false, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateFullscreen, arc.WindowStateFullscreen},
		{"MAXIMIZE <--> MINIMIZE", false, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized},
		{"MAXIMIZE <--> NORMAL", false, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateNormal, arc.WindowStateNormal},
		{"FULLSCREEN <--> MINIMIZE", false, arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateMinimized, arc.WindowStateMinimized},
		{"FULLSCREEN <--> NORMAL", false, arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateNormal, arc.WindowStateNormal},
		{"NORMAL <--> MINIMIZE", false, arc.WindowStateNormal, arc.WindowStateNormal, arc.WindowStateMinimized, arc.WindowStateMinimized},
		// Tablet Mode.
		{"MAXIMIZE <--> FULLSCREEN", true, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateFullscreen, arc.WindowStateFullscreen},
		{"MAXIMIZE <--> MINIMIZE", true, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized},
		{"MAXIMIZE <--> NORMAL", true, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMaximized},
		{"FULLSCREEN <--> MINIMIZE", true, arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateMinimized, arc.WindowStateMinimized},
		{"FULLSCREEN <--> NORMAL", true, arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateNormal, arc.WindowStateMaximized},
		{"NORMAL <--> MINIMIZE", true, arc.WindowStateNormal, arc.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized},
	} {
		s.Logf("Running %s with tablet mode enabled=%t", test.name, test.tabletMode)
		s.Logf("Setting tablet mode enabled to %t", test.tabletMode)
		if err := ash.SetTabletModeEnabled(ctx, tconn, test.tabletMode); err != nil {
			s.Fatalf("Failed to set tablet mode enabled to %t: %v", test.tabletMode, err)
		}

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
