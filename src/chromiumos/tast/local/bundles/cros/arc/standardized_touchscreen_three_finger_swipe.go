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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedTouchscreenThreeFingerSwipe,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Functional test that installs an app and tests that a standard touchscreen three finger swipe works",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetClamshellTest(runStandardizedTouchscreenThreeFingerSwipeTest),
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetTabletTest(runStandardizedTouchscreenThreeFingerSwipeTest),
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetClamshellTest(runStandardizedTouchscreenThreeFingerSwipeTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetTabletTest(runStandardizedTouchscreenThreeFingerSwipeTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
		},
		},
	})
}

func StandardizedTouchscreenThreeFingerSwipe(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".ThreeFingerSwipeTestActivity"
	)

	t := s.Param().(standardizedtestutil.Test)
	standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
}

func runStandardizedTouchscreenThreeFingerSwipeTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	// Setup the selectors.
	txtThreeFingerSwipeID := testParameters.AppPkgName + ":id/txtThreeFingerSwipe"
	txtThreeFingerSwipeSelector := testParameters.Device.Object(ui.ID(txtThreeFingerSwipeID))

	txtTestStateID := testParameters.AppPkgName + ":id/txtTestState"

	// Ensure the test starts out in a pending state.
	if err := testParameters.Device.Object(ui.ID(txtTestStateID), ui.Text("PENDING")).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to make sure the app is in a pending state")
	}

	// Make sure a two finger swipe does not trigger the test.
	if err := standardizedtestutil.TouchscreenSwipe(ctx, testParameters, txtThreeFingerSwipeSelector, 2, standardizedtestutil.DownTouchscreenSwipe); err != nil {
		return errors.Wrap(err, "failed to perform a two finger swipe")
	}

	if err := testParameters.Device.Object(ui.ID(txtTestStateID), ui.Text("PENDING")).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to verify a two finger swipe does not trigger a swipe state")
	}

	// Perform a swipe in each direction.
	testsToPerform := []swipeDirectionTest{
		{SwipeDirection: standardizedtestutil.UpTouchscreenSwipe, ExpectedText: "Direction: UP"},
		{SwipeDirection: standardizedtestutil.DownTouchscreenSwipe, ExpectedText: "Direction: DOWN"},
		{SwipeDirection: standardizedtestutil.LeftTouchscreenSwipe, ExpectedText: "Direction: LEFT"},
		{SwipeDirection: standardizedtestutil.RightTouchscreenSwipe, ExpectedText: "Direction: RIGHT"},
	}

	for _, curTest := range testsToPerform {
		if err := standardizedtestutil.TouchscreenSwipe(ctx, testParameters, txtThreeFingerSwipeSelector, 3, curTest.SwipeDirection); err != nil {
			errors.Wrapf(err, "failed to perform a three finger swipe in the %v direction", curTest.SwipeDirection)
		}

		if err := testParameters.Device.Object(ui.ID(txtTestStateID), ui.TextStartsWith(curTest.ExpectedText)).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
			errors.Wrapf(err, "failed to verify a three finger swipe in the %v direction was performed", curTest.SwipeDirection)
		}
	}

	return nil
}

// swipeDirectionTest holds data related to the tests to run.
type swipeDirectionTest struct {
	SwipeDirection standardizedtestutil.TouchscreenSwipeDirection
	ExpectedText   string
}
