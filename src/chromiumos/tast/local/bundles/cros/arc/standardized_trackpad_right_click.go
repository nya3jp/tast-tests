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
		Func:         StandardizedTrackpadRightClick,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Functional test that installs an app and tests standard trackpad right click functionality. Tests are only performed in clamshell mode as tablets don't support the trackpad",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{
			{
				Val:               standardizedtestutil.GetClamshellTest(runStandardizedTrackpadRightClickTest),
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			}, {
				Name:              "vm",
				Val:               standardizedtestutil.GetClamshellTest(runStandardizedTrackpadRightClickTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			},
		},
	})
}

func StandardizedTrackpadRightClick(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".PointerRightClickTestActivity"
	)

	t := s.Param().(standardizedtestutil.Test)
	standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
}

func runStandardizedTrackpadRightClickTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	btnRightClickID := testParameters.AppPkgName + ":id/btnRightClick"
	btnRightClickSelector := testParameters.Device.Object(ui.ID(btnRightClickID))

	trackpad, err := input.Trackpad(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup the trackpad")
	}
	defer trackpad.Close()

	if err := btnRightClickSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to find the button to click")
	}

	if err := standardizedtestutil.TrackpadClickObject(ctx, testParameters, btnRightClickSelector, trackpad, standardizedtestutil.RightPointerButton); err != nil {
		return errors.Wrap(err, "failed to click the button")
	}

	if err := testParameters.Device.Object(ui.Text("POINTER RIGHT CLICK (1)")).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to verify success")
	}

	if err := testParameters.Device.Object(ui.Text("POINTER RIGHT CLICK (2)")).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to verify only one click event was fired")
	}

	return nil
}
