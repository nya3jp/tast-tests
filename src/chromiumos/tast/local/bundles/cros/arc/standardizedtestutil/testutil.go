// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package standardizedtestutil provides helper functions to assist with running standardized arc tests
// for against android applications.
package standardizedtestutil

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// Variables used by other tast tests
const (
	defaultTestCaseTimeout = 2 * time.Minute
	ShortUITimeout         = 30 * time.Second
)

// StandardizedTestFuncParams contains parameters that can be used by the standardized tests.
type StandardizedTestFuncParams struct {
	TestConn        *chrome.TestConn
	Arc             *arc.ARC
	Device          *ui.Device
	AppPkgName      string
	AppActivityName string
	Activity        *arc.Activity
}

// StandardizedTestFunc represents the test function.
type StandardizedTestFunc func(ctx context.Context, s *testing.State, testParameters StandardizedTestFuncParams)

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
		{Name: "Full Screen", Fn: fn, WindowStateType: ash.WindowStateFullscreen},
		{Name: "Normal", Fn: fn, WindowStateType: ash.WindowStateNormal},
		{Name: "Snapped left", Fn: fn, WindowStateType: ash.WindowStateLeftSnapped},
		{Name: "Snapped right", Fn: fn, WindowStateType: ash.WindowStateRightSnapped},
	}
}

// GetStandardizedClamshellHardwareDeps returns the hardware dependencies all clamshell tests share.
func GetStandardizedClamshellHardwareDeps() hwdep.Deps {
	return hwdep.D(hwdep.InternalDisplay(), hwdep.SkipOnModel(TabletOnlyModels...))
}

// GetStandardizedTabletTests returns the test cases required for tablet devices.
func GetStandardizedTabletTests(fn StandardizedTestFunc) []StandardizedTestCase {
	return []StandardizedTestCase{
		{Name: "Full Screen", Fn: fn, WindowStateType: ash.WindowStateFullscreen},
		{Name: "Maximized", Fn: fn, WindowStateType: ash.WindowStateMaximized},
	}
}

// GetStandardizedTabletHardwareDeps returns the hardware dependencies all tablet tests share.
func GetStandardizedTabletHardwareDeps() hwdep.Deps {
	return hwdep.D(hwdep.InternalDisplay(), hwdep.SkipOnModel(ClamshellOnlyModels...))
}

// RunStandardizedTestCases runs the provided test cases and handles cleanup between tests.
func RunStandardizedTestCases(ctx context.Context, s *testing.State, apkName, appPkgName, appActivity string, testCases []StandardizedTestCase) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	act, err := arc.NewActivity(a, appPkgName, appActivity)
	if err != nil {
		s.Fatal("Failed to create new app activity: ", err)
	}
	defer act.Close()

	// Run the different test cases.
	for idx, test := range testCases {
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

			// Take screenshot and dump ui info on failure.
			defer func(ctx context.Context) {
				if s.HasError() {
					filename := fmt.Sprintf("screenshot-standardized-arcappcompat-failed-test-%d.png", idx)
					path := filepath.Join(s.OutDir(), filename)
					if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
						testing.ContextLog(ctx, "Failed to capture screenshot, info: ", err)
					} else {
						testing.ContextLogf(ctx, "Saved screenshot to %s", filename)
					}
				}
			}(cleanupCtx)

			d, err := a.NewUIDevice(ctx)
			if err != nil {
				s.Fatal("Failed initializing UI Automator: ", err)
			}
			defer d.Close(ctx)

			if _, err := ash.SetARCAppWindowStateAndWait(ctx, tconn, appPkgName, test.WindowStateType); err != nil {
				s.Fatal("Failed to set window state: ", err)
			}

			test.Fn(ctx, s, StandardizedTestFuncParams{
				TestConn:        tconn,
				Arc:             a,
				Device:          d,
				AppPkgName:      appPkgName,
				AppActivityName: appActivity,
				Activity:        act,
			})
		})
		cancel()
	}
}

// StandardizedTouchscreenClick performs a click on the touchscreen.
func StandardizedTouchscreenClick(ctx context.Context, testParameters StandardizedTestFuncParams, selector *ui.Object) error {
	touchScreen, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "Unable to initialize touchscreen")
	}
	defer touchScreen.Close()

	touchScreenSingleEventWriter, err := touchScreen.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "Unable to initialize touchscreen single event writer")
	}
	defer touchScreenSingleEventWriter.Close()

	x, y, err := getTouchEventCoordinatesForElement(ctx, testParameters, touchScreen, selector)
	if err != nil {
		return errors.Wrap(err, "Unable to get touch screen coords")
	}

	// Move to the given point and end the write to simulate a click.
	if err := touchScreenSingleEventWriter.Move(*x, *y); err != nil {
		return errors.Wrap(err, "Unable to move into position")
	}

	if err := touchScreenSingleEventWriter.End(); err != nil {
		return errors.Wrap(err, "Unable to end click")
	}

	return nil
}

// getTouchEventCoordinatesForElement converts the points of an element to the corresponding Touchscreen coordinates.
func getTouchEventCoordinatesForElement(ctx context.Context, testParameters StandardizedTestFuncParams, touchScreen *input.TouchscreenEventWriter, selector *ui.Object) (*input.TouchCoord, *input.TouchCoord, error) {
	// Get the center of the element to make sure the element is actually clicked.
	uiElementBounds, err := selector.GetBounds(ctx)
	if err != nil {
		return nil, nil, err
	}

	uiElementBoundsCenter := uiElementBounds.CenterPoint()

	// Get the size of the display, according to the activity as that's what 'GetBounds' is referring to.
	actSize, err := testParameters.Activity.DisplaySize(ctx)
	if err != nil {
		return nil, nil, err
	}

	tcc := touchScreen.NewTouchCoordConverter(actSize)
	xCord, yCord := tcc.ConvertLocation(uiElementBoundsCenter)
	return &xCord, &yCord, nil
}

// ClickInputAndGuaranteeFocus makes sure an input exists, clicks it, and ensures it is focused.
func ClickInputAndGuaranteeFocus(ctx context.Context, selector *ui.Object) error {
	if err := selector.Exists(ctx); err != nil {
		return errors.Wrap(err, "unable to find the input")
	}

	if err := selector.Click(ctx); err != nil {
		return errors.Wrap(err, "unable to click the input")
	}

	isFocused, err := selector.IsFocused(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to check the inputs focus state")
	}

	if isFocused == false {
		return errors.Wrap(err, "unable to focus the input")
	}

	return nil
}

// TabletOnlyModels is a list of tablet only models to be skipped from clamshell mode runs.
var TabletOnlyModels = []string{
	"dru",
	"krane",
}

// ClamshellOnlyModels is a list of clamshell only models to be skipped from tablet mode runs.
var ClamshellOnlyModels = []string{
	"sarien",
	"elemi",
	"berknip",
	"dratini",

	// grunt:
	"careena",
	"kasumi",
	"treeya",
	"grunt",
	"barla",
	"aleena",
	"liara",
	"nuwani",

	// octopus:
	"bluebird",
	"apel",
	"blooglet",
	"blorb",
	"bobba",
	"casta",
	"dorp",
	"droid",
	"fleex",
	"foob",
	"garfour",
	"garg",
	"laser14",
	"lick",
	"mimrock",
	"nospike",
	"orbatrix",
	"phaser",
	"sparky",
}
