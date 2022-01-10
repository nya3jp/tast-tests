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
		Func:         StandardizedTouchscreenTap,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Functional test that installs an app and tests that a standard touchscreen tap works",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		// TODO(b/210260303): Remove models after tap issue is resolved for kukui devices.
		HardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.SkipOnModel("kakadu", "katsu", "kodama")),
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetClamshellTest(runStandardizedTouchscreenTapTest),
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "tablet_mode",
			Val:               standardizedtestutil.GetTabletTest(runStandardizedTouchscreenTapTest),
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetClamshellTest(runStandardizedTouchscreenTapTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "vm_tablet_mode",
			Val:               standardizedtestutil.GetTabletTest(runStandardizedTouchscreenTapTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
		}},
	})
}

// StandardizedTouchscreenTap runs all the provided test cases.
func StandardizedTouchscreenTap(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".TapTestActivity"
	)

	t := s.Param().(standardizedtestutil.Test)
	standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
}

func runStandardizedTouchscreenTapTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	btnTapID := testParameters.AppPkgName + ":id/btnTap"
	btnTapSelector := testParameters.Device.Object(ui.ID(btnTapID))

	successLabelSelector := testParameters.Device.Object(ui.Text("TOUCHSCREEN TAP (1)"))
	tooManyTapsLabelSelector := testParameters.Device.Object(ui.Text("TOUCHSCREEN TAP (2)"))

	if err := btnTapSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "unable to find the button to tap")
	}

	if err := successLabelSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the success label should not yet exist")
	}

	if err := standardizedtestutil.TouchscreenTap(ctx, testParameters, btnTapSelector, standardizedtestutil.ShortTouchscreenTap); err != nil {
		return errors.Wrap(err, "unable to tap the button")
	}

	if err := successLabelSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the success label should exist")
	}

	if err := tooManyTapsLabelSelector.WaitUntilGone(ctx, 0); err != nil {
		return errors.Wrap(err, "a single tap triggered multiple events")
	}

	return nil
}
