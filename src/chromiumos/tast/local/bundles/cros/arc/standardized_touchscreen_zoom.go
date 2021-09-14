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
		Func:         StandardizedTouchscreenZoom,
		Desc:         "Functional test that installs an app and tests that a standard touchscreen zoom in, and zoom out gestures work",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{
			{
				Val:               standardizedtestutil.GetClamshellTests(runStandardizedTouchscreenZoomTest),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBooted",
				ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
			}, {
				Name:              "tablet_mode",
				Val:               standardizedtestutil.GetTabletTests(runStandardizedTouchscreenZoomTest),
				ExtraSoftwareDeps: []string{"android_p"},
				Fixture:           "arcBootedInTabletMode",
				ExtraHardwareDeps: standardizedtestutil.GetTabletHardwareDeps(),
			}, {
				Name:              "vm",
				Val:               standardizedtestutil.GetClamshellTests(runStandardizedTouchscreenZoomTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBooted",
				ExtraHardwareDeps: standardizedtestutil.GetClamshellHardwareDeps(),
			}, {
				Name:              "vm_tablet_mode",
				Val:               standardizedtestutil.GetTabletTests(runStandardizedTouchscreenZoomTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				Fixture:           "arcBootedInTabletMode",
				ExtraHardwareDeps: standardizedtestutil.GetTabletHardwareDeps(),
			}},
	})
}

func StandardizedTouchscreenZoom(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".ZoomTestActivity"
	)

	testCases := s.Param().([]standardizedtestutil.TestCase)
	standardizedtestutil.RunTestCases(ctx, s, apkName, appName, activityName, testCases)
}

func runStandardizedTouchscreenZoomTest(ctx context.Context, s *testing.State, testParameters standardizedtestutil.TestFuncParams) {
	txtZoomID := testParameters.AppPkgName + ":id/txtZoom"
	txtZoomSelector := testParameters.Device.Object(ui.ID(txtZoomID))

	txtZoomInStateID := testParameters.AppPkgName + ":id/txtZoomInState"
	zoomInSuccessLabelSelector := testParameters.Device.Object(ui.ID(txtZoomInStateID), ui.Text("ZOOM IN: COMPLETE"))

	txtZoomOutStateID := testParameters.AppPkgName + ":id/txtZoomOutState"
	zoomOutSuccessLabelSelector := testParameters.Device.Object(ui.ID(txtZoomOutStateID), ui.Text("ZOOM OUT: COMPLETE"))

	touchScreen, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Unable to initialize the touchscreen, info: ", err)
	}
	defer touchScreen.Close()

	if err := txtZoomSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("Unable to find the element to zoom in on, info: ", err)
	}

	// No labels should be in their complete state before the tests begin.
	if err := zoomInSuccessLabelSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The zoom in success label should not yet exist, info: ", err)
	}

	if err := zoomOutSuccessLabelSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The zoom out success label should not yet exist, info: ", err)
	}

	// After the zoom in, only the zoom in label should be in the success state.
	if err := standardizedtestutil.TouchscreenZoom(ctx, touchScreen, testParameters, txtZoomSelector, standardizedtestutil.ZoomIn); err != nil {
		s.Fatal("Unable to perform the zoom, info: ", err)
	}

	if err := zoomInSuccessLabelSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The zoom in success label should exist, info: ", err)
	}

	if err := zoomOutSuccessLabelSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The zoom out success label should not yet exist, info: ", err)
	}

	// After the zoom out, all zoom labels should be in the success state.
	if err := standardizedtestutil.TouchscreenZoom(ctx, touchScreen, testParameters, txtZoomSelector, standardizedtestutil.ZoomOut); err != nil {
		s.Fatal("Unable to perform the zoom, info: ", err)
	}

	if err := zoomInSuccessLabelSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The zoom in success label should exist, info: ", err)
	}

	if err := zoomOutSuccessLabelSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		s.Fatal("The zoom out success label should exist, info: ", err)
	}
}
