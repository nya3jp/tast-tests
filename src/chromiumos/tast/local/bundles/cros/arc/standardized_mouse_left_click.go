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
		Func:         StandardizedMouseLeftClick,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test that installs an app and tests standard mouse left click functionality. Tests are only performed in clamshell mode as tablets don't allow mice",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "no_chrome_dcheck"},
		Timeout:      10 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			Val:               standardizedtestutil.GetClamshellTest(runStandardizedMouseLeftClickTest),
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}, {
			Name:              "vm",
			Val:               standardizedtestutil.GetClamshellTest(runStandardizedMouseLeftClickTest),
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
		}},
	})
}

// StandardizedMouseLeftClick runs all the provided test cases.
func StandardizedMouseLeftClick(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".PointerLeftClickTestActivity"
	)

	t := s.Param().(standardizedtestutil.Test)
	standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
}

// runStandardizedMouseLeftClickTest runs the left click test.
func runStandardizedMouseLeftClickTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	btnLeftClickID := testParameters.AppPkgName + ":id/btnLeftClick"
	btnLeftClickSelector := testParameters.Device.Object(ui.ID(btnLeftClickID))

	// Setup the mouse
	mouse, err := input.Mouse(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to setup the mouse")
	}
	defer mouse.Close()

	if err := btnLeftClickSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "unable to find the button to click")
	}

	if err := testParameters.Device.Object(ui.Text("POINTER LEFT CLICK (1)")).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the success label should not yet exist")
	}

	if err := standardizedtestutil.MouseClickObject(ctx, testParameters, btnLeftClickSelector, mouse, standardizedtestutil.LeftPointerButton); err != nil {
		return errors.Wrap(err, "unable to click the button")
	}

	if err := testParameters.Device.Object(ui.Text("POINTER LEFT CLICK (1)")).WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the success label should exist")
	}

	if err := testParameters.Device.Object(ui.Text("POINTER LEFT CLICK (2)")).WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "a single mouse click triggered two click events")
	}

	return nil
}
