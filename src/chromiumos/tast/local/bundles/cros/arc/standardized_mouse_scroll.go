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
		Func:         StandardizedMouseScroll,
		Desc:         "Functional test that installs an app and tests that a standard mouse scroll up, an down works",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Val:               standardizedtestutil.GetClamshellTests(runStandardizedMouseScrollTest),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBooted",
				ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
			},
			{
				Name:              "vm",
				Val:               standardizedtestutil.GetClamshellTests(runStandardizedMouseScrollTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBooted",
				ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
			},
		},
	})
}

func StandardizedMouseScroll(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".ScrollTestActivity"
	)

	testCases := s.Param().([]standardizedtestutil.TestCase)
	standardizedtestutil.RunTestCases(ctx, s, apkName, appName, activityName, testCases)
}

func runStandardizedMouseScrollTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.TestFuncParams) {
	// Perform the down test first as the up test depends on it to be complete.
	txtScrollDownTestStateID := testParameters.AppPkgName + ":id/txtScrollDownTestState"
	txtScrollDownSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollDownTestStateID), ui.Text("COMPLETE"))
	runMouseScroll(ctx, s, testParameters, txtScrollDownSuccessSelector, standardizedtestutil.DownScroll)

	txtScrollUpTestStateID := testParameters.AppPkgName + ":id/txtScrollUpTestState"
	txtScrollUpSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollUpTestStateID), ui.Text("COMPLETE"))
	runMouseScroll(ctx, s, testParameters, txtScrollUpSuccessSelector, standardizedtestutil.UpScroll)
}

func runMouseScroll(ctx context.Context, s *testing.State, testParameters standardizedtestutil.TestFuncParams, txtSuccessSelector *ui.Object, scrollDirection standardizedtestutil.ScrollDirection) {
	const (
		maxNumScrollIterations = 15
	)

	txtScrollableContentID := testParameters.AppPkgName + ":id/txtScrollableContent"
	txtScrollableContentSelector := testParameters.Device.Object(ui.ID(txtScrollableContentID))

	// Setup the mouse.
	mouse, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal("Unable to setup the mouse, info: ", err)
	}
	defer mouse.Close()

	if err := txtScrollableContentSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Failed to find the scrollable content: ", err)
	}

	if err := txtSuccessSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Failed to make sure the success label does not exist: ", err)
	}

	if err := standardizedtestutil.MouseMoveOntoObject(ctx, testParameters, txtScrollableContentSelector, mouse); err != nil {
		s.Fatal("Failed to move onto the scrollable content: ", err)
	}

	// Scroll multiple times, if the threshold is reached early, the test passes.
	testPassed := false
	for i := 0; i < maxNumScrollIterations; i++ {
		// Perform the scroll.
		if err := standardizedtestutil.MouseScroll(ctx, testParameters, scrollDirection, mouse); err != nil {
			s.Fatal("Failed to perform the scroll: ", err)
		}

		// Check to see if the test is done.
		if err := txtSuccessSelector.WaitForExists(ctx, 1*time.Second); err == nil {
			testPassed = true
			break
		}
	}

	// Error out if the test did not pass.
	if testPassed == false {
		s.Fatalf("Failed to scroll the content past the threshold after %v iterations", maxNumScrollIterations)
	}
}
