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
		Func:         StandardizedTrackpadScroll,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Functional test that installs an app and tests standard trackpad scroll up and scroll down functionality. Tests are only performed in clamshell mode as tablets don't support the trackpad",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{
			{
				Val:               standardizedtestutil.GetClamshellTest(runStandardizedTrackpadScrollTest),
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			},
			{
				Name:              "vm",
				Val:               standardizedtestutil.GetClamshellTest(runStandardizedTrackpadScrollTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			},
		},
	})
}

func StandardizedTrackpadScroll(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".ScrollTestActivity"
	)

	t := s.Param().(standardizedtestutil.Test)
	standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
}

// runStandardizedTrackpadScrollTest performs up and down scroll tests using the trackpad.
func runStandardizedTrackpadScrollTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	trackpad, err := input.Trackpad(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize the trackpad")
	}
	defer trackpad.Close()

	// Perform the down test first as the up test depends on it to be complete.
	txtScrollDownTestStateID := testParameters.AppPkgName + ":id/txtScrollDownTestState"
	txtScrollDownSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollDownTestStateID), ui.Text("COMPLETE"))
	if err := performTrackpadScrollTest(ctx, testParameters, txtScrollDownSuccessSelector, trackpad, standardizedtestutil.DownScroll); err != nil {
		return errors.Wrap(err, "unable to perform down scroll")
	}

	txtScrollUpTestStateID := testParameters.AppPkgName + ":id/txtScrollUpTestState"
	txtScrollUpSuccessSelector := testParameters.Device.Object(ui.ID(txtScrollUpTestStateID), ui.Text("COMPLETE"))
	if err := performTrackpadScrollTest(ctx, testParameters, txtScrollUpSuccessSelector, trackpad, standardizedtestutil.UpScroll); err != nil {
		return errors.Wrap(err, "unable to perform up scroll")
	}

	return nil
}

// performTrackpadScrollTest runs a scroll test in a provided direction, and checks for the correct end state.
func performTrackpadScrollTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams, txtSuccessSelector *ui.Object, trackpad *input.TrackpadEventWriter, scrollDirection standardizedtestutil.ScrollDirection) error {
	const maxNumScrollIterations = 15

	txtScrollableContentID := testParameters.AppPkgName + ":id/txtScrollableContent"
	txtScrollableContentSelector := testParameters.Device.Object(ui.ID(txtScrollableContentID))

	if err := txtScrollableContentSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to find the scrollable content")
	}

	if err := txtSuccessSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "failed to find the success label")
	}

	// Scroll multiple times, if the threshold is reached early, the test passes.
	testPassed := false
	for i := 0; i < maxNumScrollIterations; i++ {
		// Perform the scroll.
		if err := standardizedtestutil.TrackpadScroll(ctx, trackpad, testParameters, txtScrollableContentSelector, scrollDirection); err != nil {
			return errors.Wrap(err, "failed to perform a scroll")
		}

		// Check to see if the test is done.
		if err := txtSuccessSelector.WaitForExists(ctx, 1*time.Second); err == nil {
			testPassed = true
			break
		}
	}

	// Error out if the test did not pass.
	if testPassed == false {
		return errors.Errorf("failed to scroll the content past the threshold after %v iterations", maxNumScrollIterations)
	}

	return nil
}
