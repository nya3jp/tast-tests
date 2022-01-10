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
		Func:         StandardizedTouchscreenScroll,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Functional test that installs an app and tests that a standard touchscreen scroll up, an ddown works",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "arcBooted",
		Params: []testing.Param{
			{
				Val:               standardizedtestutil.GetClamshellTest(runStandardizedTouchScreenScrollTest),
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			},
			{
				Name:              "tablet_mode",
				Val:               standardizedtestutil.GetTabletTest(runStandardizedTouchScreenScrollTest),
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
			},
			{
				Name:              "vm",
				Val:               standardizedtestutil.GetClamshellTest(runStandardizedTouchScreenScrollTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			},
			{
				Name:              "vm_tablet_mode",
				Val:               standardizedtestutil.GetTabletTest(runStandardizedTouchScreenScrollTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
			},
		},
	})
}

func StandardizedTouchscreenScroll(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".ScrollTestActivity"
	)

	t := s.Param().(standardizedtestutil.Test)
	standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
}

func runStandardizedTouchScreenScrollTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	touchScreen, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to initialize the touchscreen")
	}
	defer touchScreen.Close()

	// Perform the down test first as the up test depends on it to be complete.
	txtScrollDownTestStateID := testParameters.AppPkgName + ":id/txtScrollDownTestState"
	txtScrollDownSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollDownTestStateID), ui.Text("COMPLETE"))
	if err := performTest(ctx, testParameters, txtScrollDownSuccessSelector, touchScreen, standardizedtestutil.DownScroll); err != nil {
		return errors.Wrap(err, "unable to perform down scroll")
	}

	txtScrollUpTestStateID := testParameters.AppPkgName + ":id/txtScrollUpTestState"
	txtScrollUpSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollUpTestStateID), ui.Text("COMPLETE"))
	if err := performTest(ctx, testParameters, txtScrollUpSuccessSelector, touchScreen, standardizedtestutil.UpScroll); err != nil {
		return errors.Wrap(err, "unable to perform up scroll")
	}

	return nil
}

func performTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams, txtSuccessSelector *ui.Object, touchScreen *input.TouchscreenEventWriter, scrollDirection standardizedtestutil.ScrollDirection) error {
	const (
		maxNumScrollIterations = 15
	)

	txtScrollableContentID := testParameters.AppPkgName + ":id/txtScrollableContent"
	txtScrollableContentSelector := testParameters.Device.Object(ui.ID(txtScrollableContentID))

	if err := txtScrollableContentSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "unable to find the scrollable content")
	}

	if err := txtSuccessSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the success label should not yet exist")
	}

	// Scroll multiple times, if the threshold is reached early, the test passes.
	testPassed := false
	for i := 0; i < maxNumScrollIterations; i++ {
		// Perform the scroll.
		if err := standardizedtestutil.TouchscreenScroll(ctx, touchScreen, testParameters, txtScrollableContentSelector, scrollDirection); err != nil {
			return errors.Wrap(err, "unable to perform a scroll")
		}

		// Check to see if the test is done.
		if err := txtSuccessSelector.WaitForExists(ctx, 1*time.Second); err == nil {
			testPassed = true
			break
		}
	}

	// Error out if the test did not pass.
	if testPassed == false {
		errors.Errorf("unable to scroll the content past the threshold after %v iterations", maxNumScrollIterations)
	}

	return nil
}
