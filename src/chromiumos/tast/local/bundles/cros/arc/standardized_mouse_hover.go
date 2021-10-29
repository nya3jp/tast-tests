// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/bundles/cros/arc/standardizedtestutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedMouseHover,
		Desc:         "Functional test that installs an app and tests standard mouse hover functionality. Tests are only performed in clamshell mode as tablets don't allow mice",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Val:               standardizedtestutil.GetClamshellTests(runStandardizedMouseHoverTest),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBootedInClamshellMode",
				ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
			}, {
				Name:              "vm",
				Val:               standardizedtestutil.GetClamshellTests(runStandardizedMouseHoverTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBootedInClamshellMode",
				ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
			}},
	})
}

func StandardizedMouseHover(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedMouseTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedmousetest"
		activityName = ".HoverTestActivity"
	)

	testCases := s.Param().([]standardizedtestutil.TestCase)
	standardizedtestutil.RunTestCases(ctx, s, apkName, appName, activityName, testCases)
}

func runStandardizedMouseHoverTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	// Setup the mouse.
	mouse, err := input.Mouse(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup the mouse")
	}
	defer mouse.Close()

	// Setup selectors.
	btnStartHoverTestSelector := testParameters.Device.Object(ui.ID(testParameters.AppPkgName + ":id/btnStartHoverTest"))
	txtHoverEnterID := testParameters.AppPkgName + ":id/txtHoverEnterState"
	txtHoverExitID := testParameters.AppPkgName + ":id/txtHoverExitState"
	txtStatusSelector := testParameters.Device.Object(ui.ID(testParameters.AppPkgName + ":id/txtStatus"))

	// Ensure the app is in the initial state.
	if err := txtStatusSelector.WaitForText(ctx, "Status: Not Started", standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to ensure test hasn't started")
	}

	// Click to start the test.
	if err := standardizedtestutil.MouseClickObject(ctx, testParameters, btnStartHoverTestSelector, mouse, standardizedtestutil.LeftPointerButton); err != nil {
		return errors.Wrap(err, "failed to click the button to start the test")
	}

	// Ensure the test is ready.
	if err := txtStatusSelector.WaitForText(ctx, "Status: Started", standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to ensure test has started")
	}

	// Move over the hover element.
	txtToHoverSelector := testParameters.Device.Object(ui.ID(testParameters.AppPkgName + ":id/txtToHover"))
	if err := standardizedtestutil.MouseMoveOntoObject(ctx, testParameters, txtToHoverSelector, mouse); err != nil {
		return errors.Wrap(err, "failed to move the mouse onto the hover element")
	}

	// Verify the 'hover enter' state.
	if err := testParameters.Device.Object(ui.ID(txtHoverEnterID), ui.Text("HOVER ENTER: COMPLETE")).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to find completed hover enter text")
	}

	// Move over the end state element.
	txtToEndOnSelector := testParameters.Device.Object(ui.ID(testParameters.AppPkgName + ":id/txtToEndOn"))
	if err := standardizedtestutil.MouseMoveOntoObject(ctx, testParameters, txtToEndOnSelector, mouse); err != nil {
		return errors.Wrap(err, "failed to move the mouse onto the end element")
	}

	// Verify the 'hover off' state.
	if err := testParameters.Device.Object(ui.ID(txtHoverExitID), ui.Text("HOVER EXIT: COMPLETE")).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to find completed hover exit text")
	}

	return nil
}
