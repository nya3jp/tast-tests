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
		Func:         StandardizedMouseRightClick,
		Desc:         "Functional test that installs an app and tests standard mouse right click functionality. Tests are only performed in clamshell mode as tablets don't allow mice",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedMouseRightClickTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedMouseRightClickTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}},
	})
}

func StandardizedMouseRightClick(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedMouseTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedmousetest"
		activityName = ".MainActivity"
	)

	testCases := s.Param().([]standardizedtestutil.StandardizedTestCase)
	standardizedtestutil.RunStandardizedTestCases(ctx, s, apkName, appName, activityName, testCases)
}

func runStandardizedMouseRightClickTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	btnRightClickID := testParameters.AppPkgName + ":id/btnRightClick"
	btnRightClickSelector := testParameters.Device.Object(ui.ID(btnRightClickID))

	// Setup the mouse
	mouse, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal("Unable to setup the mouse, info: ", err)
	}
	defer mouse.Close()

	if err := btnRightClickSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Unable to find the button to click, info: ", err)
	}

	if err := testParameters.Device.Object(ui.Text("MOUSE RIGHT CLICK (1)")).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should not yet exist, info: ", err)
	}

	if err := standardizedtestutil.StandardizedMouseClickObject(ctx, testParameters, btnRightClickSelector, mouse, standardizedtestutil.RightMouseButton); err != nil {
		s.Fatal("Unable to click the button, info: ", err)
	}

	if err := testParameters.Device.Object(ui.Text("MOUSE RIGHT CLICK (1)")).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should exist, info: ", err)
	}

	if err := testParameters.Device.Object(ui.Text("MOUSE RIGHT CLICK (2)")).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("A single mouse click triggered two click events, info: ", err)
	}
}
