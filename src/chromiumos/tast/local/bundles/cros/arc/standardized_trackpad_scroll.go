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
		Func:         StandardizedTrackpadScroll,
		Desc:         "Functional test that installs an app and tests standard trackpad scroll up and scroll down functionality. Tests are only performed in clamshell mode as tablets don't support the trackpad",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Val:               standardizedtestutil.GetClamshellTests(runStandardizedTrackpadScrollTest),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBooted",
				ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
			},
			{
				Name:              "vm",
				Val:               standardizedtestutil.GetClamshellTests(runStandardizedTrackpadScrollTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBooted",
				ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
			},
		},
	})
}

func StandardizedTrackpadScroll(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".ScrollTestActivity"
	)

	testCases := s.Param().([]standardizedtestutil.TestCase)
	standardizedtestutil.RunTestCases(ctx, s, apkName, appName, activityName, testCases)
}

func runStandardizedTrackpadScrollTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.TestFuncParams) {
	trackpad, err := input.Trackpad(ctx)
	if err != nil {
		s.Fatal("Failed to initialize the trackpad: ", err)
	}
	defer trackpad.Close()

	// Perform the down test first as the up test depends on it to be complete.
	txtScrollDownTestStateID := testParameters.AppPkgName + ":id/txtScrollDownTestState"
	txtScrollDownSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollDownTestStateID), ui.Text("COMPLETE"))
	performTrackpadScrollTest(ctx, s, testParameters, txtScrollDownSuccessSelector, trackpad, standardizedtestutil.DownScroll)

	txtScrollUpTestStateID := testParameters.AppPkgName + ":id/txtScrollUpTestState"
	txtScrollUpSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollUpTestStateID), ui.Text("COMPLETE"))
	performTrackpadScrollTest(ctx, s, testParameters, txtScrollUpSuccessSelector, trackpad, standardizedtestutil.UpScroll)
}

func performTrackpadScrollTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.TestFuncParams, txtSuccessSelector *ui.Object, trackpad *input.TrackpadEventWriter, scrollDirection standardizedtestutil.ScrollDirection) {
	const (
		maxNumScrollIterations = 15
	)

	txtScrollableContentID := testParameters.AppPkgName + ":id/txtScrollableContent"
	txtScrollableContentSelector := testParameters.Device.Object(ui.ID(txtScrollableContentID))

	if err := txtScrollableContentSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Failed to find the scrollable content: ", err)
	}

	if err := txtSuccessSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Failed to find the success label: ", err)
	}

	// Scroll multiple times, if the threshold is reached early, the test passes.
	testPassed := false
	for i := 0; i < maxNumScrollIterations; i++ {
		// Perform the scroll.
		if err := standardizedtestutil.TrackpadScroll(ctx, trackpad, testParameters, txtScrollableContentSelector, scrollDirection); err != nil {
			s.Fatal("Failed to perform a scroll: ", err)
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
