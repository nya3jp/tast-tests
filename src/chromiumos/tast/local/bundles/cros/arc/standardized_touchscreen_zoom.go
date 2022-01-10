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
		Func:         StandardizedTouchscreenZoom,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Functional test that installs an app and tests that a standard touchscreen zoom in, and zoom out gestures work",
		Contacts:     []string{"davidwelling@google.com", "cros-appcompat-test-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "arcBooted",
		Params: []testing.Param{
			{
				Val:               standardizedtestutil.GetClamshellTest(runStandardizedTouchscreenZoomTest),
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			}, {
				Name:              "tablet_mode",
				Val:               standardizedtestutil.GetTabletTest(runStandardizedTouchscreenZoomTest),
				ExtraSoftwareDeps: []string{"android_p"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
			}, {
				Name:              "vm",
				Val:               standardizedtestutil.GetClamshellTest(runStandardizedTouchscreenZoomTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.ClamshellHardwareDep),
			}, {
				Name:              "vm_tablet_mode",
				Val:               standardizedtestutil.GetTabletTest(runStandardizedTouchscreenZoomTest),
				ExtraSoftwareDeps: []string{"android_vm"},
				ExtraHardwareDeps: hwdep.D(standardizedtestutil.TabletHardwareDep),
			}},
	})
}

func StandardizedTouchscreenZoom(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcStandardizedInputTest.apk"
		appName      = "org.chromium.arc.testapp.arcstandardizedinputtest"
		activityName = ".ZoomTestActivity"
	)

	t := s.Param().(standardizedtestutil.Test)
	standardizedtestutil.RunTest(ctx, s, apkName, appName, activityName, t)
}

func runStandardizedTouchscreenZoomTest(ctx context.Context, testParameters standardizedtestutil.TestFuncParams) error {
	txtZoomID := testParameters.AppPkgName + ":id/txtZoom"
	txtZoomSelector := testParameters.Device.Object(ui.ID(txtZoomID))

	txtZoomInStateID := testParameters.AppPkgName + ":id/txtZoomInState"
	zoomInSuccessLabelSelector := testParameters.Device.Object(ui.ID(txtZoomInStateID), ui.Text("ZOOM IN: COMPLETE"))

	txtZoomOutStateID := testParameters.AppPkgName + ":id/txtZoomOutState"
	zoomOutSuccessLabelSelector := testParameters.Device.Object(ui.ID(txtZoomOutStateID), ui.Text("ZOOM OUT: COMPLETE"))

	touchScreen, err := input.Touchscreen(ctx)
	if err != nil {
		return errors.Wrap(err, "unable to initialize the touchscreen")
	}
	defer touchScreen.Close()

	if err := txtZoomSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "unable to find the element to zoom in on")
	}

	// No labels should be in their complete state before the tests begin.
	if err := zoomInSuccessLabelSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the zoom in success label should not yet exist")
	}

	if err := zoomOutSuccessLabelSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the zoom out success label should not yet exist")
	}

	// After the zoom in, only the zoom in label should be in the success state.
	if err := standardizedtestutil.TouchscreenZoom(ctx, touchScreen, testParameters, txtZoomSelector, standardizedtestutil.ZoomIn); err != nil {
		return errors.Wrap(err, "unable to perform the zoom")
	}

	if err := zoomInSuccessLabelSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the zoom in success label should exist")
	}

	if err := zoomOutSuccessLabelSelector.WaitUntilGone(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the zoom out success label should not yet exist")
	}

	// After the zoom out, all zoom labels should be in the success state.
	if err := standardizedtestutil.TouchscreenZoom(ctx, touchScreen, testParameters, txtZoomSelector, standardizedtestutil.ZoomOut); err != nil {
		return errors.Wrap(err, "unable to perform the zoom")
	}

	if err := zoomInSuccessLabelSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the zoom in success label should exist")
	}

	if err := zoomOutSuccessLabelSelector.WaitForExists(ctx, standardizedtestutil.ShortUITimeout); err != nil {
		return errors.Wrap(err, "the zoom out success label should exist")
	}

	return nil
}
