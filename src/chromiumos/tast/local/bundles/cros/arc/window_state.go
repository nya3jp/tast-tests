// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     WindowState,
		Desc:     "Checks that ARC++ applications correctly change the window state",
		Contacts: []string{"phshah@chromium.org", "arc-eng@google.com"},
		Attr:     []string{"informational"},
		// Adding 'tablet_mode' since moving/resizing the window requires screen touch support.
		SoftwareDeps: []string{"android_p", "chrome", "tablet_mode"},
		Timeout:      5 * time.Minute,
	})
}

func WindowState(ctx context.Context, s *testing.State) {
	// Force Chrome to be in clamshell mode, where windows are resizable.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--force-tablet-mode=clamshell"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Start ARC++
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Start the Settings app
	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	// Restore tablet mode to its original state on exit
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	// Run the different test cases
	for idx, test := range []struct {
		name                       string
		tabletMode                 bool
		initialWindowState         arc.WindowState
		expectedInitialWindowState arc.WindowState
		finalWindowState           arc.WindowState
		expectedFinalWindowState   arc.WindowState
	}{
		// Clamshell Mode
		{"MAXIMIZE <--> FULLSCREEN", false, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateFullscreen, arc.WindowStateFullscreen},
		{"MAXIMIZE <--> MINIMIZE", false, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized},
		{"MAXIMIZE <--> NORMAL", false, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateNormal, arc.WindowStateNormal},
		{"FULLSCREEN <--> MINIMIZE", false, arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateMinimized, arc.WindowStateMinimized},
		{"FULLSCREEN <--> NORMAL", false, arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateNormal, arc.WindowStateNormal},
		{"NORMAL <--> MINIMIZE", false, arc.WindowStateNormal, arc.WindowStateNormal, arc.WindowStateMinimized, arc.WindowStateMinimized},
		// Tablet Mode
		{"MAXIMIZE <--> FULLSCREEN", true, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateFullscreen, arc.WindowStateFullscreen},
		{"MAXIMIZE <--> MINIMIZE", true, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized},
		{"MAXIMIZE <--> NORMAL", true, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMaximized},
		{"FULLSCREEN <--> MINIMIZE", true, arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateMinimized, arc.WindowStateMinimized},
		{"FULLSCREEN <--> NORMAL", true, arc.WindowStateFullscreen, arc.WindowStateFullscreen, arc.WindowStateNormal, arc.WindowStateMaximized},
		{"NORMAL <--> MINIMIZE", true, arc.WindowStateNormal, arc.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized},
	} {
		s.Logf("Running %s with tablet mode enabled=%t", test.name, test.tabletMode)
		currentTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get tablet mode: ", err)
		}
		if currentTabletMode != test.tabletMode {
			s.Logf("Setting tablet mode enabled to %t", test.tabletMode)
			if err := ash.SetTabletModeEnabled(ctx, tconn, test.tabletMode); err != nil {
				s.Fatalf("%d Failed to set tablet mode enabled to %t: %v", idx, test.tabletMode, err)
			}
		}

		// Set the activity to the initial WindowState.
		if err := changeActivityWindowState(ctx, act, test.initialWindowState, test.expectedInitialWindowState); err != nil {
			s.Fatalf("%d: %v", idx, err)
		}
		for i := 0; i < 25; i++ {
			// Change to first WindowState
			if err := changeActivityWindowState(ctx, act, test.initialWindowState, test.expectedInitialWindowState); err != nil {
				s.Fatalf("%d: %v", idx, err)
			}
			// Change to second WindowState
			if err := changeActivityWindowState(ctx, act, test.finalWindowState, test.expectedFinalWindowState); err != nil {
				s.Fatalf("%d: %v", idx, err)
			}
		}
	}
}

func changeActivityWindowState(ctx context.Context, act *arc.Activity, targetWindowState arc.WindowState, expectedWindowState arc.WindowState) error {
	// Set the new WindowState
	if err := act.SetWindowState(ctx, targetWindowState); err != nil {
		return errors.Wrapf(err, "Failed to set window state to %s: ", arc.WindowStateToString(targetWindowState))
	}
	// Verify the new WindowState
	actualWindowState, err := act.GetWindowState(ctx)
	if err != nil {
		return errors.Wrap(err, "Could not get window state")
	}
	if actualWindowState != expectedWindowState {
		return errors.Errorf("Received incorrect window state. Expected: %s Actual: %s", arc.WindowStateToString(expectedWindowState), arc.WindowStateToString(actualWindowState))
	}
	return nil
}
