// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testutil

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// StandardizedTestFunc represents the test function.
type StandardizedTestFunc func(ctx context.Context, s *testing.State, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, appPkgName, appActivity string, activity *arc.Activity)

// StandardizedTestCase holds information about a test to run.
type StandardizedTestCase struct {
	Name            string
	Fn              StandardizedTestFunc
	Timeout         time.Duration
	WindowStateType ash.WindowStateType
}

// GetStandardizedClamshellTests returns the test cases required for clamshell devices.
func GetStandardizedClamshellTests(fn StandardizedTestFunc) []StandardizedTestCase {
	return []StandardizedTestCase{
		{Name: "Normal", Fn: fn, WindowStateType: ash.WindowStateNormal},
		{Name: "Snapped left", Fn: fn, WindowStateType: ash.WindowStateLeftSnapped},
		{Name: "Snapped right", Fn: fn, WindowStateType: ash.WindowStateRightSnapped},
		{Name: "Full Screen", Fn: fn, WindowStateType: ash.WindowStateFullscreen},
	}
}

// GetStandardizedClamshellHardwareDeps returns the hardware dependencies all clamshell tests share.
func GetStandardizedClamshellHardwareDeps() hwdep.Deps {
	return hwdep.D(hwdep.SkipOnModel(TabletOnlyModels...))
}

// GetStandardizedTabletTests returns the test cases required for tablet devices.
func GetStandardizedTabletTests(fn StandardizedTestFunc) []StandardizedTestCase {
	return []StandardizedTestCase{
		{Name: "Maximized", Fn: fn, WindowStateType: ash.WindowStateMaximized},
		{Name: "Full Screen", Fn: fn, WindowStateType: ash.WindowStateFullscreen},
	}
}

// GetStandardizedTabletHardwareDeps returns the hardware dependencies all tablet tests share.
func GetStandardizedTabletHardwareDeps() hwdep.Deps {
	return hwdep.D(hwdep.SkipOnModel(ClamshellOnlyModels...))
}

// RunStandardizedTestCases runs the provided test cases and handles cleanup between tests.
func RunStandardizedTestCases(ctx context.Context, s *testing.State, apkName, appPkgName, appActivity string, testCases []StandardizedTestCase) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Could not initialize UI Automator: ", err)
	}
	defer d.Close(ctx)

	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()

	// Run the different test cases.
	for _, test := range testCases {
		// If a timeout is not specified, limited individual test cases to the default.
		// This makes sure that one test case doesn't use all of the time when it fails.
		timeout := defaultTestCaseTimeout
		if test.Timeout != 0 {
			timeout = test.Timeout
		}
		testCaseCtx, cancel := ctxutil.Shorten(ctx, timeout)
		defer cancel()

		s.Run(testCaseCtx, test.Name, func(cleanupCtx context.Context, s *testing.State) {
			// Save time for cleanup
			ctx, cancel := ctxutil.Shorten(cleanupCtx, 20*time.Second)
			defer cancel()

			// Launch the app.
			if err := act.Start(ctx, tconn); err != nil {
				s.Fatal("Failed to start app: ", err)
			}

			// Close the app between iterations.
			defer func(ctx context.Context) {
				if err := act.Stop(ctx, tconn); err != nil {
					s.Fatal("Failed to stop app: ", err)
				}
			}(cleanupCtx)

			if err := cpu.WaitUntilIdle(ctx); err != nil {
				s.Fatal("Failed to wait until CPU idle: ", err)
			}

			d, err := a.NewUIDevice(ctx)
			if err != nil {
				s.Fatal("Failed initializing UI Automator: ", err)
			}
			defer d.Close(ctx)

			if err := setWindowState(ctx, tconn, appPkgName, test.WindowStateType); err != nil {
				s.Fatal("Failed to ensure window state: ", err)
			}

			// Changing the window state causes a CPU spike - give it a chance to idle.
			if err := cpu.WaitUntilIdle(ctx); err != nil {
				s.Fatal("Failed to wait until CPU idle: ", err)
			}

			test.Fn(ctx, s, tconn, a, d, appPkgName, appActivity, act)
		})
		cancel()
	}
}

// setWindowState sets the state of the window and returns an error if the new state was not applied.
func setWindowState(ctx context.Context, tconn *chrome.TestConn, pkgName string, expectedState ash.WindowStateType) error {
	wmEvent, ok := ash.WindowStateTypeToEventType[expectedState]
	if !ok {
		return errors.Errorf("didn't find the event for window state: %q", expectedState)
	}

	state, err := ash.SetARCAppWindowState(ctx, tconn, pkgName, wmEvent)
	if err != nil {
		return err
	}
	if state != expectedState {
		return errors.Errorf("unexpected window state: got %s; want %s", state, expectedState)
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, pkgName, expectedState); err != nil {
		return errors.Wrapf(err, "failed to wait for activity to enter %v state", expectedState)
	}

	return nil
}
