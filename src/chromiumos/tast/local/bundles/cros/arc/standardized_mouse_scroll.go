// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arc/standardizedtestutil"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type standardizedMouseScrollArgs struct {
	test              standardizedtestutil.Test
	resizeLockEnabled bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         StandardizedMouseScroll,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Functional test that installs an app and tests that a standard mouse scroll up, an down works",
		Contacts:     []string{"davidwelling@google.com", "cpiao@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "no_chrome_dcheck"},
		Timeout:      10 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{
			{
				Val: &standardizedMouseScrollArgs{
					test:              standardizedtestutil.GetClamshellTest(runStandardizedMouseScrollTest),
					resizeLockEnabled: false,
				},
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			},
			{
				Name: "vm",
				Val: &standardizedMouseScrollArgs{
					test:              standardizedtestutil.GetClamshellTest(runStandardizedMouseScrollTest),
					resizeLockEnabled: false,
				},
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			},
			{
				Name: "resize_lock_smooth_scroll_vm",
				Val: &standardizedMouseScrollArgs{
					test: standardizedtestutil.Test{
						Fn:           runStandardizedMouseScrollTest,
						InTabletMode: false,
						WindowStates: []standardizedtestutil.WindowState{
							{Name: "Normal", WindowStateType: ash.WindowStateNormal},
						},
					},
					resizeLockEnabled: true,
				},
				ExtraAttr:         []string{"informational"},
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			},
		},
	})
}

func StandardizedMouseScroll(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".ScrollTestActivity"
	)

	t := s.Param().(*standardizedMouseScrollArgs).test
	resizeLockEnabled := s.Param().(*standardizedMouseScrollArgs).resizeLockEnabled
	if resizeLockEnabled {
		standardizedtestutil.RunResizeLockTest(ctx, s, apkName, appName, activityName, t)
	} else {
		standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
	}
}

func runStandardizedMouseScrollTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	// Perform the down test first as the up test depends on it to be complete.
	txtScrollDownTestStateID := testParameters.AppPkgName + ":id/txtScrollDownTestState"
	txtScrollDownSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollDownTestStateID), ui.Text("COMPLETE"))
	if err := runMouseScroll(ctx, testParameters, txtScrollDownSuccessSelector, standardizedtestutil.DownScroll); err != nil {
		return errors.Wrap(err, "scroll down test failed")
	}

	txtScrollUpTestStateID := testParameters.AppPkgName + ":id/txtScrollUpTestState"
	txtScrollUpSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollUpTestStateID), ui.Text("COMPLETE"))
	if err := runMouseScroll(ctx, testParameters, txtScrollUpSuccessSelector, standardizedtestutil.UpScroll); err != nil {
		return errors.Wrap(err, "scroll up test failed")
	}

	return nil
}

func runMouseScroll(ctx context.Context, testParameters standardizedtestutil.TestFuncParams, txtSuccessSelector *ui.Object, scrollDirection standardizedtestutil.ScrollDirection) error {
	const (
		maxNumScrollIterations = 15
	)

	txtScrollableContentID := testParameters.AppPkgName + ":id/txtScrollableContent"
	txtScrollableContentSelector := testParameters.Device.Object(ui.ID(txtScrollableContentID))

	// Setup the mouse.
	mouse, err := input.Mouse(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to setup the mouse")
	}
	defer mouse.Close()

	if err := txtScrollableContentSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to find the scrollable content")
	}

	if err := txtSuccessSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to make sure the success label does not exist")
	}

	if err := standardizedtestutil.MouseMoveOntoObject(ctx, testParameters, txtScrollableContentSelector, mouse); err != nil {
		return errors.Wrap(err, "failed to move onto the scrollable content")
	}

	// Scroll multiple times, if the threshold is reached early, the test passes.
	testPassed := false
	for i := 0; i < maxNumScrollIterations; i++ {
		// Perform the scroll.
		if err := standardizedtestutil.MouseScroll(ctx, testParameters, scrollDirection, mouse); err != nil {
			return errors.Wrap(err, "failed to perform the scroll")
		}

		// Check to see if the test is done.
		if err := txtSuccessSelector.WaitForExists(ctx, 1*time.Second); err == nil {
			testPassed = true
			break
		}
	}

	// Error out if the test did not pass.
	if testPassed == false {
		return errors.Wrapf(err, "failed to scroll the content past the threshold after %v iterations", maxNumScrollIterations)
	}

	return nil
}
