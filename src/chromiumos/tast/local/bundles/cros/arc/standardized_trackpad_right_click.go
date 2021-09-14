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
		Func:         StandardizedTrackpadRightClick,
		Desc:         "Functional test that installs an app and tests standard trackpad right click functionality. Tests are only performed in clamshell mode as tablets don't support the trackpad",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Val:               standardizedtestutil.GetClamshellTests(runStandardizedTrackpadRightClickTest),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBooted",
				ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
			}, {
				Name:              "vm",
				Val:               standardizedtestutil.GetClamshellTests(runStandardizedTrackpadRightClickTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBooted",
				ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
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

	testCases := s.Param().([]standardizedtestutil.TestCase)
	standardizedtestutil.RunTestCases(ctx, s, apkName, appName, activityName, testCases)
}

func runStandardizedTrackpadRightClickTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.TestFuncParams) {
	btnRightClickID := testParameters.AppPkgName + ":id/btnRightClick"
	btnRightClickSelector := testParameters.Device.Object(ui.ID(btnRightClickID))

	trackpad, err := input.Trackpad(ctx)
	if err != nil {
		s.Fatal("Failed to setup the trackpad: ", err)
	}
	defer trackpad.Close()

	if err := btnRightClickSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Failed to find the button to click: ", err)
	}

	if err := standardizedtestutil.TrackpadClickObject(ctx, testParameters, btnRightClickSelector, trackpad, standardizedtestutil.RightPointerButton); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}

	if err := testParameters.Device.Object(ui.Text("POINTER RIGHT CLICK (1)")).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Failed to verify success: ", err)
	}

	if err := testParameters.Device.Object(ui.Text("POINTER RIGHT CLICK (2)")).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Failed to verify only one click event was fired: ", err)
	}
}
