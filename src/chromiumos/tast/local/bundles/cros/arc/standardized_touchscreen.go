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
		Func:         StandardizedTouchscreen,
		Desc:         "Functional test that installs an app and tests standard touchscreen input functionality",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedTouchscreenTests),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedTouchscreenTests),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedTouchscreenTests),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedTouchscreenTests),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
		}},
	})
}

// StandardizedTouchscreen runs all the provided test cases.
func StandardizedTouchscreen(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedTouchscreenTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedtouchscreentest"
		activityName = ".MainActivity"
	)

	testCases := s.Param().([]standardizedtestutil.StandardizedTestCase)
	standardizedtestutil.RunStandardizedTestCases(ctx, s, apkName, appName, activityName, testCases)
}

// runStandardizedTouchscreenTests runs all the tests that are part of the
// standardized touchscreen input suite.
func runStandardizedTouchscreenTests(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	runClickTest(ctx, s, testParameters)
}

// runClickTest ensures that the touch click event works.
func runClickTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	btnClickID := testParameters.AppPkgName + ":id/btnClick"
	btnClickSelector := testParameters.Device.Object(ui.ID(btnClickID))

	successLabelSelector := testParameters.Device.Object(ui.Text("TOUCHSCREEN CLICK (1)"))
	tooManyClicksLabelSelector := testParameters.Device.Object(ui.Text("TOUCHSCREEN CLICK (2)"))

	if err := btnClickSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Unable to find the button to click, info: ", err)
	}

	if err := successLabelSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should not yet exist, info: ", err)
	}

	if err := standardizedtestutil.StandardizedTouchscreenClick(ctx, testParameters, btnClickSelector); err != nil {
		s.Fatal("Unable to click the button, info: ", err)
	}

	if err := successLabelSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should exist, info: ", err)
	}

	if err := tooManyClicksLabelSelector.WaitUntilGone(ctx, 0); err != nil {
		s.Fatal("A single click triggered two click events, info: ", err)
	}
}
