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
		Func:         StandardizedTouchscreenLongTap,
		Desc:         "Functional test that installs an app and tests that a standard touchscreen long tap works",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedTouchscreenLongTapTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedTouchscreenLongTapTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedTouchscreenLongTapTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetStandardizedTabletTests(runStandardizedTouchscreenLongTapTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedTabletHardwareDeps(),
		}},
	})
}

// StandardizedTouchscreenLongTap runs all the provided test cases.
func StandardizedTouchscreenLongTap(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedTouchscreenTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedtouchscreentest"
		activityName = ".MainActivity"
	)

	testCases := s.Param().([]standardizedtestutil.StandardizedTestCase)
	standardizedtestutil.RunStandardizedTestCases(ctx, s, apkName, appName, activityName, testCases)
}

func runStandardizedTouchscreenLongTapTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	btnLongTapID := testParameters.AppPkgName + ":id/btnLongTap"
	btnLongTapSelector := testParameters.Device.Object(ui.ID(btnLongTapID))

	successLabelSelector := testParameters.Device.Object(ui.Text("TOUCHSCREEN LONG TAP (1)"))
	tooManyTapsLabelSelector := testParameters.Device.Object(ui.Text("TOUCHSCREEN LONG TAP (2)"))

	if err := btnLongTapSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Unable to find the button to long tap, info: ", err)
	}

	if err := successLabelSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should not yet exist, info: ", err)
	}

	if err := standardizedtestutil.StandardizedTouchscreenTap(ctx, testParameters, btnLongTapSelector, standardizedtestutil.LongTouchscreenTap); err != nil {
		s.Fatal("Unable to long tap the button, info: ", err)
	}

	if err := successLabelSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should exist, info: ", err)
	}

	if err := tooManyTapsLabelSelector.WaitUntilGone(ctx, 0); err != nil {
		s.Fatal("A single long tap triggered multiple events, info: ", err)
	}
}
