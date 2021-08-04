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
		Func:         StandardizedMouseLeftClick,
		Desc:         "Functional test that installs an app and tests standard mouse left click functionality. Tests are only performed in clamshell mode as tablets don't allow mice",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedMouseLeftClickTest),
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetStandardizedClamshellTests(runStandardizedMouseLeftClickTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
			ExtraHardwareDeps: standardizedtestutil.GetStandardizedClamshellHardwareDeps(),
		}},
	})
}

// StandardizedMouseLeftClick runs all the provided test cases.
func StandardizedMouseLeftClick(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedMouseTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedmousetest"
		activityName = ".MainActivity"
	)

	testCases := s.Param().([]standardizedtestutil.StandardizedTestCase)
	standardizedtestutil.RunStandardizedTestCases(ctx, s, apkName, appName, activityName, testCases)
}

// runStandardizedMouseLeftClickTest runs the left click test.
func runStandardizedMouseLeftClickTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.StandardizedTestFuncParams) {
	btnLeftClickID := testParameters.AppPkgName + ":id/btnLeftClick"
	btnLeftClickSelector := testParameters.Device.Object(ui.ID(btnLeftClickID))

	// Setup the mouse
	mouse, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal("Unable to setup the mouse, info: ", err)
	}
	defer mouse.Close()

	if err := btnLeftClickSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Unable to find the button to click, info: ", err)
	}

	if err := testParameters.Device.Object(ui.Text("MOUSE LEFT CLICK (1)")).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should not yet exist, info: ", err)
	}

	if err := standardizedtestutil.StandardizedMouseClickObject(ctx, testParameters, btnLeftClickSelector, mouse, standardizedtestutil.LeftMouseButton); err != nil {
		s.Fatal("Unable to click the button, info: ", err)
	}

	if err := testParameters.Device.Object(ui.Text("MOUSE LEFT CLICK (1)")).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The success label should exist, info: ", err)
	}

	if err := testParameters.Device.Object(ui.Text("MOUSE LEFT CLICK (2)")).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("A single mouse click triggered two click events, info: ", err)
	}
}
