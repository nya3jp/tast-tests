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

// windowStateTest is used to represent a single window state transition test.
type windowStateTest struct {
	name                          string              // Name of test case.
	initialWindowState            arc.WindowState     // Activity's initial window state.
	expectedInitialArcWindowState arc.WindowState     // Activity's expected, initial ARC window state.
	expectedInitialAshWindowState ash.WindowStateType // Activity's expected, initial ASH window state.
	finalWindowState              arc.WindowState     // Activity's final window state.
	expectedFinalArcWindowState   arc.WindowState     // Activity's expected, final ARC window state.
	expectedFinalAshWindowState   ash.WindowStateType // Activity's expected, final ASH window state.
}

// windowStateParams is used to represent a collection of tests to run in tablet mode or clamshell mode.
type windowStateParams struct {
	tabletMode     bool              // True, if device should be in tablet mode.
	testIterations int               // Number of test iterations.
	tests          []windowStateTest // Activity's initial window state.
}

// clamshellWindowStateTests contains list of clamshell mode test cases.
var clamshellWindowStateTests = []windowStateTest{
	{"MAXIMIZE <--> FULLSCREEN", arc.WindowStateMaximized, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateFullscreen, arc.WindowStateFullscreen, ash.WindowStateFullscreen},
	{"MAXIMIZE <--> MINIMIZE", arc.WindowStateMaximized, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized, ash.WindowStateMinimized},
	{"MAXIMIZE <--> NORMAL", arc.WindowStateMaximized, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateNormal, arc.WindowStateNormal, ash.WindowStateNormal},
	{"FULLSCREEN <--> MINIMIZE", arc.WindowStateFullscreen, arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateMinimized, arc.WindowStateMinimized, ash.WindowStateMinimized},
	{"FULLSCREEN <--> NORMAL", arc.WindowStateFullscreen, arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateNormal, arc.WindowStateNormal, ash.WindowStateNormal},
	{"NORMAL <--> MINIMIZE", arc.WindowStateNormal, arc.WindowStateNormal, ash.WindowStateNormal, arc.WindowStateMinimized, arc.WindowStateMinimized, ash.WindowStateMinimized},
}

// tabletWindowStateTests contains list of tablet mode test cases.
var tabletWindowStateTests = []windowStateTest{
	{"MAXIMIZE <--> FULLSCREEN", arc.WindowStateMaximized, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateFullscreen, arc.WindowStateFullscreen, ash.WindowStateFullscreen},
	{"MAXIMIZE <--> MINIMIZE", arc.WindowStateMaximized, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized, ash.WindowStateMinimized},
	{"MAXIMIZE <--> NORMAL", arc.WindowStateMaximized, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateMaximized, arc.WindowStateMaximized, ash.WindowStateMaximized},
	{"FULLSCREEN <--> MINIMIZE", arc.WindowStateFullscreen, arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateMinimized, arc.WindowStateMinimized, ash.WindowStateMinimized},
	{"FULLSCREEN <--> NORMAL", arc.WindowStateFullscreen, arc.WindowStateFullscreen, ash.WindowStateFullscreen, arc.WindowStateNormal, arc.WindowStateMaximized, ash.WindowStateMaximized},
	{"NORMAL <--> MINIMIZE", arc.WindowStateNormal, arc.WindowStateMaximized, ash.WindowStateMaximized, arc.WindowStateMinimized, arc.WindowStateMinimized, ash.WindowStateMinimized},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowState,
		Desc:         "Checks that ARC applications correctly change the window state",
		Contacts:     []string{"phshah@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "clamshell",
			Val: windowStateParams{
				false, // Clamshell mode.
				1,     // Num test iterations.
				clamshellWindowStateTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name: "clamshell_vm",
			Val: windowStateParams{
				false, // Clamshell mode.
				1,     // Num test iterations.
				clamshellWindowStateTests,
			},
			ExtraSoftwareDeps: []string{"android_vm_p"},
			Pre:               arc.VMBooted(),
		}, {
			Name: "clamshell_stress",
			Val: windowStateParams{
				false, // Clamshell mode.
				25,    // Num test iterations.
				clamshellWindowStateTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name: "clamshell_stress_vm",
			Val: windowStateParams{
				false, // Clamshell mode.
				25,    // Num test iterations.
				clamshellWindowStateTests,
			},
			ExtraSoftwareDeps: []string{"android_vm_p"},
			Pre:               arc.VMBooted(),
		}, {
			Name: "tablet",
			Val: windowStateParams{
				true, // Tablet Mode.
				1,    // Num test iterations.
				tabletWindowStateTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name: "tablet_vm",
			Val: windowStateParams{
				true, // Tablet Mode.
				1,    // Num test iterations.
				tabletWindowStateTests,
			},
			ExtraSoftwareDeps: []string{"android_vm_p"},
			Pre:               arc.VMBooted(),
		}, {
			Name: "tablet_stress",
			Val: windowStateParams{
				true, // Tablet Mode.
				25,   // Num test iterations.
				tabletWindowStateTests,
			},
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name: "tablet_stress_vm",
			Val: windowStateParams{
				true, // Tablet Mode.
				25,   // Num test iterations.
				tabletWindowStateTests,
			},
			ExtraSoftwareDeps: []string{"android_vm_p"},
			Pre:               arc.VMBooted(),
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

	testParams := s.Param().(windowStateParams)

	deviceMode := "clamshell"
	if testParams.tabletMode {
		deviceMode = "tablet"
	}
	s.Logf("Setting device to %v mode", deviceMode)
	if err := ash.SetTabletModeEnabled(ctx, tconn, testParams.tabletMode); err != nil {
		s.Fatalf("Failed to set tablet mode enabled to %t: %v", testParams.tabletMode, err)
	}

	// Run the different test cases.
	for _, test := range testParams.tests {
		s.Log("Testing ", test.name)
		if err := func() error {
			// Start the Settings app.
			act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
			if err != nil {
				return errors.Wrap(err, "failed to create new activity")
			}
			// Close the resources associated with the Activity instance.
			defer act.Close()

			if err := act.Start(ctx); err != nil {
				return errors.Wrap(err, "failed to start the Settings activity")
			}
			// Stop the activity for each test case
			defer act.Stop(ctx)

			if err := ash.WaitForVisible(ctx, tconn, act.PackageName()); err != nil {
				return errors.Wrap(err, "failed to wait for visible activity")
			}

			// Set the activity to the initial WindowState.
			if err := setAndVerifyWindowState(ctx, act, tconn, test.initialWindowState, test.expectedInitialAshWindowState, test.expectedInitialArcWindowState); err != nil {
				return errors.Wrap(err, "failed to set initial window state")
			}

			for i := 0; i < testParams.testIterations; i++ {
				// Initial WindowState transition.
				if err := setAndVerifyWindowState(ctx, act, tconn, test.initialWindowState, test.expectedInitialAshWindowState, test.expectedInitialArcWindowState); err != nil {
					return errors.Wrapf(err, "failed to set the initial window state in iter %d", i)
				}

				// Final WindowState transition.
				if err := setAndVerifyWindowState(ctx, act, tconn, test.finalWindowState, test.expectedFinalAshWindowState, test.expectedFinalArcWindowState); err != nil {
					return errors.Wrapf(err, "failed to set the final window state in iter %d", i)
				}
			}
			return nil
		}(); err != nil {
			s.Fatalf("%q subtest failed: %v", test.name, err)
		}
	}
}

// setAndVerifyWindowState sets and verifies the desired window state transition.
func setAndVerifyWindowState(ctx context.Context, act *arc.Activity, tconn *chrome.TestConn, arcWindowState arc.WindowState, expectedAshWindowState ash.WindowStateType, expectedArcWindowState arc.WindowState) error {
	if err := act.SetWindowState(ctx, arcWindowState); err != nil {
		return errors.Wrapf(err, "failed to set window state (%v)", arcWindowState)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), expectedAshWindowState); err != nil {
		return errors.Wrapf(err, "failed to wait for a window state to appear on the Chrome side (%v)", expectedAshWindowState)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		actualArcWindowState, err := act.GetWindowState(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "could not get ARC window state"))
		}
		if actualArcWindowState != expectedArcWindowState {
			return errors.Errorf("unexpected ARC window state: got %v; want %v", actualArcWindowState, expectedArcWindowState)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "timed out waiting for ARC window state transition")
	}
	return nil
}
