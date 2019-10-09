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

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowState,
		Desc:         "Checks that ARC applications correctly change the window state",
		Contacts:     []string{"phshah@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
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

	if err := act.WaitForIdle(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to wait for idle activity: ", err)
	}

	// Number of window state transition tests.
	const numTestCount = 25

	// Run the different test cases.
	for _, test := range []struct {
		name                       string
		tabletMode                 bool
		initialWindowState         arc.WindowState
		expectedInitialWindowState ash.WindowStateType
		finalWindowState           arc.WindowState
		expectedFinalWindowState   ash.WindowStateType
	}{
		// Clamshell Mode.
		{"MAXIMIZE <--> FULLSCREEN", false, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateFullscreen, ash.WindowStateFullscreen},
		{"MAXIMIZE <--> MINIMIZE", false, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateMinimized, ash.WindowStateMinimized},
		{"MAXIMIZE <--> NORMAL", false, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateNormal, ash.WindowStateNormal},
		{"FULLSCREEN <--> MINIMIZE", false, arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateMinimized, ash.WindowStateMinimized},
		{"FULLSCREEN <--> NORMAL", false, arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateNormal, ash.WindowStateNormal},
		{"NORMAL <--> MINIMIZE", false, arc.WindowStateNormal, ash.WindowStateNormal, arc.WindowStateMinimized, ash.WindowStateMinimized},
		// Tablet Mode.
		{"MAXIMIZE <--> FULLSCREEN", true, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateFullscreen, ash.WindowStateFullscreen},
		{"MAXIMIZE <--> MINIMIZE", true, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateMinimized, ash.WindowStateMinimized},
		{"MAXIMIZE <--> NORMAL", true, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateMaximized, ash.WindowStateMaximized},
		{"FULLSCREEN <--> MINIMIZE", true, arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateMinimized, ash.WindowStateMinimized},
		{"FULLSCREEN <--> NORMAL", true, arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateNormal, ash.WindowStateMaximized},
		{"NORMAL <--> MINIMIZE", true, arc.WindowStateNormal, ash.WindowStateMaximized, arc.WindowStateMinimized, ash.WindowStateMinimized},
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
