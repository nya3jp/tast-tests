// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/standardizedtestutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedMouseHover,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test that installs an app and tests standard mouse hover functionality. Tests are only performed in clamshell mode as tablets don't allow mice",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "no_chrome_dcheck"},
		Timeout:      10 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{
			{
				Val:               standardizedtestutil.GetClamshellTest(runStandardizedMouseHoverTest),
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			}, {
				Name:              "vm",
				Val:               standardizedtestutil.GetClamshellTest(runStandardizedMouseHoverTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			}},
	})
}

func StandardizedMouseHover(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".HoverTestActivity"
	)

	t := s.Param().(standardizedtestutil.Test)
	standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
}

func runStandardizedMouseHoverTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	const intentStartHoverTest = "org.chromium.arc.testapp.arcstandardizedinputtest.ACTION_START_HOVER_TEST"

	// Setup the mouse.
	mouse, err := input.Mouse(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup the mouse")
	}
	defer mouse.Close()

	// Setup selectors.
	txtHoverEnterID := testParameters.AppPkgName + ":id/txtHoverEnterState"
	txtHoverExitID := testParameters.AppPkgName + ":id/txtHoverExitState"
	txtStatusSelector := testParameters.Device.Object(ui.ID(testParameters.AppPkgName + ":id/txtStatus"))
	txtToHoverSelector := testParameters.Device.Object(ui.ID(testParameters.AppPkgName + ":id/txtToHover"))

	// Move over the status element so that starting the test doesn't immediately trigger a hover event.
	if err := standardizedtestutil.MouseMoveOntoObject(ctx, testParameters, txtStatusSelector, mouse); err != nil {
		return errors.Wrap(err, "failed to move the mouse onto the status element")
	}

	// Ensure the app is in the initial state.
	if err := txtStatusSelector.WaitForText(ctx, "Status: Not Started", standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to ensure test hasn't started")
	}

	// Start the test.
	if _, err := testParameters.Arc.BroadcastIntent(ctx, intentStartHoverTest); err != nil {
		return errors.Wrap(err, "failed to start the test")
	}

	// Ensure the test is ready.
	if err := txtStatusSelector.WaitForText(ctx, "Status: Started", standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to ensure test has started")
	}

	// Move over the hover element.
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
