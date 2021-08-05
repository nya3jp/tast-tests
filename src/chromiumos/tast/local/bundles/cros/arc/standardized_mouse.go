// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/bundles/cros/arc/standardizedtestutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedMouse,
		Desc:         "Functional test that installs an app and tests standard mouse input functionality. Tests are only performed in clamshell mode as tablets don't allow mice",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedMouseTests),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedMouseTests),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}},
	})
}

// StandardizedMouse runs all the provided test cases.
func StandardizedMouse(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedMouseTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedmousetest"
		activityName = ".MainActivity"
	)

	testCases := s.Param().([]standardizedtestutil.StandardizedTestCase)
	standardizedtestutil.RunStandardizedTestCases(ctx, s, apkName, appName, activityName, testCases)
}

// runStandardizedMouseTests runs all the tests that are part of the
// standardized mouse input suite.
func runStandardizedMouseTests(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	runLeftClickTest(ctx, s, testParameters)
	runRightClickTest(ctx, s, testParameters)
	runHoverTest(ctx, s, testParameters)
}

// runLeftClickTest ensures that the left mouse click works.
func runLeftClickTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	btnLeftClickID := testParameters.AppPkgName + ":id/btnLeftClick"
	btnLeftClickSelector := testParameters.Device.Object(ui.ID(btnLeftClickID))
	testSingleMouseClick(ctx, s, testParameters, btnLeftClickSelector, standardizedtestutil.LeftMouseButton)
}

// runRightClickTest ensures that the right mouse click works.
func runRightClickTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	btnRightClickID := testParameters.AppPkgName + ":id/btnRightClick"
	btnRightClickSelector := testParameters.Device.Object(ui.ID(btnRightClickID))
	testSingleMouseClick(ctx, s, testParameters, btnRightClickSelector, standardizedtestutil.RightMouseButton)
}

// runHoverTest ensures that hovering, and exiting an element works.
func runHoverTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	// Setup the selectors
	btnStartHoverTestID := testParameters.AppPkgName + ":id/btnStartHoverTest"
	btnStartHoverSelector := testParameters.Device.Object(ui.ID(btnStartHoverTestID))
	btnHoverSelector := testParameters.Device.Object(ui.Text("HOVER"))

	// Setup an anonymous function for validating state to simplify the calls below.
	isInValidState := func(expectedEnterExists, expectedExitExists standardizedtestutil.VerifyState) error {
		// There should never be more than one hover enter/exit event. The rest can be determined by the caller.
		return standardizedtestutil.VerifyMultipleObjectStates(ctx, []standardizedtestutil.VerifyObjectState{
			{Selector: testParameters.Device.Object(ui.Text("HOVER ENTER (1)")), State: expectedEnterExists},
			{Selector: testParameters.Device.Object(ui.Text("HOVER EXIT (1)")), State: expectedExitExists},
			{Selector: testParameters.Device.Object(ui.Text("HOVER ENTER (2)")), State: standardizedtestutil.VerifyNotExists},
			{Selector: testParameters.Device.Object(ui.Text("HOVER EXIT (2)")), State: standardizedtestutil.VerifyNotExists},
		})
	}

	// Reset the position of the mouse so that the hover isn't accidentally triggered immediately.
	if err := standardizedtestutil.MouseResetLocation(ctx, testParameters.TestConn); err != nil {
		s.Fatal("Unable to reset the mouse's position, info: ", err)
	}

	// Start the test and make sure the view is in a valid state.
	if err := btnStartHoverSelector.Click(ctx); err != nil {
		s.Fatal("Unable to start the hover test, info: ", err)
	}

	if err := btnHoverSelector.WaitForExists(ctx, time.Second*10); err != nil {
		s.Fatal("Unable to wait for hover selector to exist, info: ", err)
	}

	if err := isInValidState(standardizedtestutil.VerifyNotExists, standardizedtestutil.VerifyNotExists); err != nil {
		s.Fatal("Unable to validate initial hover state, info: ", err)
	}

	// Hover over the element and make sure the hover event appeared.
	if err := standardizedtestutil.MouseMoveToCenterOfObject(ctx, testParameters.TestConn, btnHoverSelector); err != nil {
		s.Fatal("Unable to hover over the element, info: ", err)
	}

	if err := isInValidState(standardizedtestutil.VerifyExists, standardizedtestutil.VerifyNotExists); err != nil {
		s.Fatal("Unable to validate hover enter action occurred, info: ", err)
	}

	// Leave the element and make sure the hover exit occurred
	if err := standardizedtestutil.MouseResetLocation(ctx, testParameters.TestConn); err != nil {
		s.Fatal("Unable to reset the mouse's position, info: ", err)
	}

	if err := isInValidState(standardizedtestutil.VerifyExists, standardizedtestutil.VerifyExists); err != nil {
		s.Fatal("Unable to validate hover exit action occurred, info: ", err)
	}
}

// testSingleMouseClick ensures that a single click works, and prints the expected `MOUSE <button> CLICK` text.
func testSingleMouseClick(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams, buttonSelector *ui.Object, standardizedMouseButton standardizedtestutil.StandardizedMouseButton) {
	// Setup the expected text. The button should only be clicked once so that's the entry being looked for.
	expectedSingleClick := fmt.Sprintf("MOUSE %v CLICK (1)", standardizedMouseButton)

	if err := buttonSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Unable to find the button to click, info: ", err)
	}

	if err := testParameters.Device.Object(ui.Text(expectedSingleClick)).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should not yet exist, info: ", err)
	}

	if err := standardizedtestutil.StandardizedMouseClickObject(ctx, testParameters.TestConn, buttonSelector, standardizedMouseButton); err != nil {
		s.Fatal("Unable to click the button, info: ", err)
	}

	if err := testParameters.Device.Object(ui.Text(expectedSingleClick)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should exist, info: ", err)
	}

	// Make sure a single click didn't fire two events.
	invalidDoubleClick := fmt.Sprintf("MOUSE %v CLICK (2)", standardizedMouseButton)
	if err := testParameters.Device.Object(ui.Text(invalidDoubleClick)).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("A single mouse click triggered two click events, info: ", err)
	}
}
