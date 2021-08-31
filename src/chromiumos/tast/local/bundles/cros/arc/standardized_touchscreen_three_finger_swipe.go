// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/bundles/cros/arc/standardizedtestutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedTouchscreenThreeFingerSwipe,
		Desc:         "Functional test that installs an app and tests that a standard touchscreen three finger swipe works",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedTouchscreenThreeFingerSwipeTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedTouchscreenThreeFingerSwipeTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedTouchscreenThreeFingerSwipeTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedTouchscreenThreeFingerSwipeTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
		},
		},
	})
}

func StandardizedTouchscreenThreeFingerSwipe(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedTouchscreenTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedtouchscreentest"
		activityName = ".ThreeFingerSwipeTestActivity"
	)

	testCases := s.Param().([]standardizedtestutil.StandardizedTestCase)
	standardizedtestutil.RunStandardizedTestCases(ctx, s, apkName, appName, activityName, testCases)
}

func runStandardizedTouchscreenThreeFingerSwipeTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	// Setup the selectors.
	txtThreeFingerSwipeID := testParameters.AppPkgName + ":id/txtThreeFingerSwipe"
	txtThreeFingerSwipeSelector := testParameters.Device.Object(ui.ID(txtThreeFingerSwipeID))

	txtTestStateID := testParameters.AppPkgName + ":id/txtTestState"

	// Ensure the test starts out in a pending state.
	if err := testParameters.Device.Object(ui.ID(txtTestStateID), ui.Text("PENDING")).Exists(ctx); err != nil {
		s.Fatal("Failed to make sure the app is in a pending state: ", err)
	}

	// Make sure a two finger swipe does not trigger the test.
	if err := standardizedtestutil.StandardizedTouchscreenSwipe(ctx, testParameters, txtThreeFingerSwipeSelector, 2, standardizedtestutil.DownTouchscreenSwipe); err != nil {
		s.Fatal("Failed to perform a two finger swipe: ", err)
	}

	if err := testParameters.Device.Object(ui.ID(txtTestStateID), ui.Text("PENDING")).Exists(ctx); err != nil {
		s.Fatal("Failed to verify a two finger swipe does not trigger a swipe state: ", err)
	}

	// Perform a swipe in each direction.
	testsToPerform := []swipeDirectionTest{
		{SwipeDirection: standardizedtestutil.UpTouchscreenSwipe, ExpectedText: "Direction: UP"},
		{SwipeDirection: standardizedtestutil.DownTouchscreenSwipe, ExpectedText: "Direction: DOWN"},
		{SwipeDirection: standardizedtestutil.LeftTouchscreenSwipe, ExpectedText: "Direction: LEFT"},
		{SwipeDirection: standardizedtestutil.RightTouchscreenSwipe, ExpectedText: "Direction: RIGHT"},
	}

	for _, curTest := range testsToPerform {
		if err := standardizedtestutil.StandardizedTouchscreenSwipe(ctx, testParameters, txtThreeFingerSwipeSelector, 3, curTest.SwipeDirection); err != nil {
			s.Fatalf("Failed to perform a three finger swipe in the %v direction: %v", curTest.SwipeDirection, err)
		}

		if err := testParameters.Device.Object(ui.ID(txtTestStateID), ui.TextStartsWith(curTest.ExpectedText)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
			s.Fatalf("Failed to verify a three finger swipe in the %v direction was performed: %v", curTest.SwipeDirection, err)
		}
	}
}

// swipeDirectionTest holds data related to the tests to run.
type swipeDirectionTest struct {
	SwipeDirection standardizedtestutil.StandardizedTouchscreenSwipeDirection
	ExpectedText   string
}
