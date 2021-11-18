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
		Func:         StandardizedTouchscreenLongTap,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Functional test that installs an app and tests that a standard touchscreen long tap works",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetClamshellTests(runStandardizedTouchscreenLongTapTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInClamshellMode",
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetTabletTests(runStandardizedTouchscreenLongTapTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetClamshellTests(runStandardizedTouchscreenLongTapTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInClamshellMode",
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetTabletTests(runStandardizedTouchscreenLongTapTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBootedInTabletMode",
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
		}},
	})
}

// StandardizedTouchscreenLongTap runs all the provided test cases.
func StandardizedTouchscreenLongTap(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".TapTestActivity"
	)

	testCases := s.Param().([]standardizedtestutil.TestCase)
	standardizedtestutil.RunTestCases(ctx, s, apkName, appName, activityName, testCases)
}

func runStandardizedTouchscreenLongTapTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	btnLongTapID := testParameters.AppPkgName + ":id/btnLongTap"
	btnLongTapSelector := testParameters.Device.Object(ui.ID(btnLongTapID))

	successLabelSelector := testParameters.Device.Object(ui.Text("TOUCHSCREEN LONG TAP (1)"))
	tooManyTapsLabelSelector := testParameters.Device.Object(ui.Text("TOUCHSCREEN LONG TAP (2)"))

	if err := btnLongTapSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "unable to find the button to long tap")
	}

	if err := successLabelSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the success label should not yet exist")
	}

	if err := standardizedtestutil.TouchscreenTap(ctx, testParameters, btnLongTapSelector, standardizedtestutil.LongTouchscreenTap); err != nil {
		return errors.Wrap(err, "unable to long tap the button")
	}

	if err := successLabelSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the success label should exist")
	}

	if err := tooManyTapsLabelSelector.WaitUntilGone(ctx, 0); err != nil {
		return errors.Wrap(err, "a single long tap triggered multiple events")
	}

	return nil
}
