// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	resizeLockTestPkgName = "org.chromium.arc.testapp.resizelock"

	resizeLockApkName = "ArcResizeLockTest.apk"
)

type resizeLockTestFunc func(context.Context, *chrome.TestConn, *arc.ARC, *arc.Activity, *ui.Device, *display.DisplayMode) error

type resizeLockTestParams struct {
	name       string
	fn         resizeLockTestFunc
}

var resizeLockTests = []resizeLockTestParams{
	{name: "O4C", fn: testO4C},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeLock,
		Desc:         "Checks that ARC++ Resize Lock works as expected",
		Contacts:     []string{"takise@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Val:               ResizeLock,
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ResizeLock(ctx context.Context, s *testing.State) {
	// For debugging, create a Chrome session with chrome.ExtraArgs("--show-taps")
	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC

	// ResizeLock is only applied to newly-installed apps, so ensure to reinstall the apk.
	if installed, err := a.PackageInstalled(ctx, resizeLockTestPkgName); err != nil {
		if err != nil {
			s.Fatal("Failed to query installed state: ", err)
		} else if installed {
			if a.Uninstall(ctx, arc.APKPath(resizeLockApkName)); err != nil {
				s.Fatal("Failed to uninstall app: ", err)
			}
		}
	}

	s.Log("Installing ", resizeLockApkName)
	if err := a.Install(ctx, arc.APKPath(resizeLockApkName)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}
	defer a.Uninstall(ctx, arc.APKPath(resizeLockApkName))

	mainActivity, err := arc.NewActivity(a, resizeLockTestPkgName, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create main activity: ", err)
	}
	defer mainActivity.Close()

	dev, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer dev.Close(ctx)

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	origShelfAlignment, err := ash.GetShelfAlignment(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf alignment: ", err)
	}
	if err := ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, ash.ShelfAlignmentBottom); err != nil {
		s.Fatal("Failed to set shelf alignment to Bottom: ", err)
	}
	// Be nice and restore shelf alignment to its original state on exit.
	defer ash.SetShelfAlignment(ctx, tconn, dispInfo.ID, origShelfAlignment)

	origShelfBehavior, err := ash.GetShelfBehavior(ctx, tconn, dispInfo.ID)
	if err != nil {
		s.Fatal("Failed to get shelf behavior: ", err)
	}
	if err := ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, ash.ShelfBehaviorNeverAutoHide); err != nil {
		s.Fatal("Failed to set shelf behavior to Never Auto Hide: ", err)
	}
	// Be nice and restore shelf behavior to its original state on exit.
	defer ash.SetShelfBehavior(ctx, tconn, dispInfo.ID, origShelfBehavior)

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	dispMode, err := ash.InternalDisplayMode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get display mode: ", err)
	}

	tabletModes := []bool{false, true}
	// Run all subtests twice. First, with tablet mode disabled. And then, with it enabled.
	for _, tabletMode := range tabletModes {
		s.Logf("Running tests with tablet mode enabled=%t", tabletMode)
		if err := ash.SetTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
			s.Fatalf("Failed to set tablet mode enabled to %t: %v", tabletMode, err)
		}

		s.Logf("Running tests with tablet mode enabled=%t", tabletMode)
		for idx, test := range resizeLockTests {
			testing.ContextLog(ctx, "About to run test: ", test.name)

			if err := test.fn(ctx, tconn, a, mainActivity, dev, dispMode); err != nil {
				path := fmt.Sprintf("%s/screenshot-resize-lock-failed-test-%d.png", s.OutDir(), idx)
				if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
					s.Log("Failed to capture screenshot: ", err)
				}
				s.Errorf("%s test with tablet mode(%t) failed: %v", test.name, tabletMode, err)
			}
		}
	}
}

// testO4C verifies that O4C apps are not resize locked even if it's newly-installed.
func testO4C(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, mainActivity *arc.Activity, dev *ui.Device, dispMode *display.DisplayMode) error {
	// ResizeLock is only applied to newly-installed apps, so ensure to reinstall the apk.
	if err := a.Uninstall(ctx, arc.APKPath(wm.APKNameArcWMTestApp24)); err != nil {
		return errors.Wrap(err, "failed to uninstall app")
	}

	if err := a.Install(ctx, arc.APKPath(wm.APKNameArcWMTestApp24)); err != nil {
		return errors.Wrap(err, "failed to install app")
	}
	defer a.Uninstall(ctx, arc.APKPath(wm.APKNameArcWMTestApp24))

	mainActivity, err := arc.NewActivity(a, wm.Pkg24, wm.ResizableUnspecifiedActivity)
	if err != nil {
		return errors.Wrap(err, "failed to create resizable unspecified activity")
	}
	defer mainActivity.Close()

	window, err := ash.GetARCAppWindowInfo(ctx, tconn, wm.Pkg24)
	if err != nil {
		return errors.Wrap(err, "failed to get the window info of O4C app")
	}

	return nil
}

