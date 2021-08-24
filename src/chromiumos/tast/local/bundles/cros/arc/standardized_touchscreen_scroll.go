// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/bundles/cros/arc/standardizedtestutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedTouchscreenScroll,
		Desc:         "Functional test that installs an app and tests that a standard touchscreen scroll up, an ddown works",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedTouchScreenScrollTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		},
			{
				Name:              "tablet_mode",
				Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedTouchScreenScrollTest),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBootedInTabletMode",
				ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
			},
			{
				Name:              "vm",
				Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedTouchScreenScrollTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBooted",
				ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
			},
			{
				Name:              "vm_tablet_mode",
				Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedTouchScreenScrollTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBootedInTabletMode",
				ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
			},
		},
	})
}

func StandardizedTouchscreenScroll(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedTouchscreenTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedtouchscreentest"
		activityName = ".ScrollTestActivity"
	)

	testCases := s.Param().([]standardizedtestutil.StandardizedTestCase)
	standardizedtestutil.RunStandardizedTestCases(ctx, s, apkName, appName, activityName, testCases)
}

func runStandardizedTouchScreenScrollTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	touchScreen, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Unable to initialize the touchscreen, info: ", err)
	}
	defer touchScreen.Close()

	// Perform the down test first as the up test depends on it to be complete.
	txtScrollDownTestStateID := testParameters.AppPkgName + ":id/txtScrollDownTestState"
	txtScrollDownSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollDownTestStateID), ui.Text("COMPLETE"))
	performTest(ctx, s, testParameters, txtScrollDownSuccessSelector, touchScreen, standardizedtestutil.DownTouchscreenScroll)

	txtScrollUpTestStateID := testParameters.AppPkgName + ":id/txtScrollUpTestState"
	txtScrollUpSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollUpTestStateID), ui.Text("COMPLETE"))
	performTest(ctx, s, testParameters, txtScrollUpSuccessSelector, touchScreen, standardizedtestutil.UpTouchscreenScroll)
}

func performTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams, txtSuccessSelector *ui.Object, touchScreen *input.TouchscreenEventWriter, scrollDirection standardizedtestutil.StandardizedTouchscreenScrollDirection) {
	const (
		maxNumScrollIterations = 15
	)

	txtScrollableContentID := testParameters.AppPkgName + ":id/txtScrollableContent"
	txtScrollableContentSelector := testParameters.Device.Object(ui.ID(txtScrollableContentID))

	if err := txtScrollableContentSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Unable to find the scrollable content, info: ", err)
	}

	if err := txtSuccessSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should not yet exist, info: ", err)
	}

	// Scroll multiple times, if the threshold is reached early, the test passes.
	testPassed := false
	for i := 0; i < maxNumScrollIterations; i++ {
		// Perform the scroll.
		if err := standardizedtestutil.StandardizedTouchscreenScroll(ctx, touchScreen, testParameters, txtScrollableContentSelector, scrollDirection); err != nil {
			s.Fatal("Unable to perform a scroll, info: ", err)
		}

		// Check to see if the test is done.
		if err := txtSuccessSelector.WaitForExists(ctx, 1*time.Second); err == nil {
			testPassed = true
			break
		}
	}

	// Error out if the test did not pass.
	if testPassed == false {
		s.Fatalf("Unable to scroll the content past the threshold after %v iterations", maxNumScrollIterations)
	}
}
