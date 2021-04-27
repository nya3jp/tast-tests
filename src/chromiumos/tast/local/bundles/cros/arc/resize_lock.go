// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"image/color"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	resizeLockTestPkgName = "org.chromium.arc.testapp.resizelock"

	resizeLockApkName = "ArcResizeLockTest.apk"

	resizeLockMainActivityName = "org.chromium.arc.testapp.resizelock.MainActivity"

	resizeLockUnresizableActivityName = "org.chromium.arc.testapp.resizelock.UnresizableActivity"
)

type resizeLockTestFunc func(context.Context, *chrome.TestConn, *arc.ARC, *ui.Device, *chrome.Chrome) error

type resizeLockTestCase struct {
	name string
	fn   resizeLockTestFunc
}

var testCases = []resizeLockTestCase{
	resizeLockTestCase{
		name: "O4C App - Resizability",
		fn:   testO4CResizability,
	},
	resizeLockTestCase{
		name: "Resize Locked App - Resizability",
		fn:   testResizeLockedResizableActivityResizability,
	},
	resizeLockTestCase{
		name: "Unresizable App - Resizability",
		fn:   testUnresizableResizability,
	},
	resizeLockTestCase{
		name: "Resize Locked App - Splash",
		fn:   testResizeLockedResizableActivitySplash,
	},
	resizeLockTestCase{
		name: "Unresizable App - Splash",
		fn:   testUnresizableSplash,
	},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeLock,
		Desc:         "Checks that ARC++ Resize Lock works as expected",
		Contacts:     []string{"takise@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Val:               ResizeLock,
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func ResizeLock(ctx context.Context, s *testing.State) {
	// Ensure to enable the finch flag.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--enable-features=ArcResizeLock"))

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Could not start ARC: ", err)
	}
	defer a.Close(ctx)

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

	for _, test := range testCases {
		s.Logf("Running test %q", test.name)

		if err := test.fn(ctx, tconn, a, dev, cr); err != nil {
			path := fmt.Sprintf("%s/screenshot-resize-lock-failed-test-%s.png", s.OutDir(), test.name)
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Errorf("%s test failed: %v", test.name, err)
		}
	}
}

// testO4CResizability verifies that an O4C app is not resize locked even if it's newly-installed.
func testO4CResizability(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome) error {
	return testResizability(ctx, tconn, a, d, wm.Pkg24, wm.APKNameArcWMTestApp24, wm.ResizableUnspecifiedActivity, true /* shouldBeResizable */)
}

// testResizeLockedResizableActivityResizability verifies that a resize-locked app is unresizable.
func testResizeLockedResizableActivityResizability(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome) error {
	return testResizability(ctx, tconn, a, d, resizeLockTestPkgName, resizeLockApkName, resizeLockMainActivityName, false /* shouldBeResizable */)
}

// testUnresizableResizability verifies that a resize-locked app is unresizable.
func testUnresizableResizability(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome) error {
	return testResizability(ctx, tconn, a, d, resizeLockTestPkgName, resizeLockApkName, resizeLockUnresizableActivityName, false /* shouldBeResizable */)
}

// testResizeLockedResizableActivitySplash verifies that a resize-locked app is unresizable.
func testResizeLockedResizableActivitySplash(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome) error {
	return testSplash(ctx, tconn, a, d, cr, resizeLockTestPkgName, resizeLockApkName, resizeLockMainActivityName, true /* shouldShowSplash */)
}

// testUnresizableSplash verifies that a resize-locked app is unresizable.
func testUnresizableSplash(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome) error {
	return nil
	// return testSplash(ctx, tconn, a, d, cr, resizeLockTestPkgName, resizeLockApkName, resizeLockUnresizableActivityName, false /* shouldShowSplash */)
}

// testSplash checks whether the splash screen for compatibility mode is shown or not.
func testSplash(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, packageName, apkName, activityName string, shouldShowSplash bool) error {
	const (
		pixelcolorDiffMargin   = 10
		colorAreaPercentMargin = 20
	)

	if uninstalled, err := reinstallAPK(ctx, a, packageName, apkName); err != nil {
		return errors.Wrap(err, "failed to reinstall APK")
	} else if !uninstalled {
		defer a.Uninstall(ctx, arc.APKPath(apkName))
	}

	activity, err := arc.NewActivity(a, packageName, activityName)
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start O4C activity")
	}
	defer activity.Stop(ctx, tconn)

	return testing.Poll(ctx, func(ctx context.Context) error {
		bounds, err := activity.WindowBounds(ctx)
		if err != nil {
			return err
		}

		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to grab screenshot")
		}

		totalPixels := bounds.Height * bounds.Width
		bluePixels := imgcmp.CountPixels(img, color.RGBA{0, 0, 255, 255})
		bluePercent := bluePixels * 100 / totalPixels
		darkBluePixels := imgcmp.CountPixelsWithDiff(img, color.RGBA{20, 20, 124, 255}, pixelcolorDiffMargin)
		darkBluePercent := darkBluePixels * 100 / totalPixels

		// The splash screen has a scrim of "Grey 900, 60%", which makes the blue background of the app darker.
		if shouldShowSplash && darkBluePercent < colorAreaPercentMargin {
			return errors.Errorf("contains %d / %d (%d%%) dark blue pixels", darkBluePixels, totalPixels, darkBluePercent)
		}
		if shouldShowSplash && bluePercent > colorAreaPercentMargin {
			return errors.Errorf("contains %d / %d (%d%%) blue pixels", bluePixels, totalPixels, bluePercent)
		}
		if !shouldShowSplash && bluePercent < colorAreaPercentMargin {
			return errors.Errorf("contains %d / %d (%d%%) blue pixels", bluePixels, totalPixels, bluePercent)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// testResizability verifies the given app's resizability.
func testResizability(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, packageName, apkName, activityName string, shouldBeResizable bool) error {
	if uninstalled, err := reinstallAPK(ctx, a, packageName, apkName); err != nil {
		return errors.Wrap(err, "failed to reinstall APK")
	} else if !uninstalled {
		defer a.Uninstall(ctx, arc.APKPath(apkName))
	}

	activity, err := arc.NewActivity(a, packageName, activityName)
	if err != nil {
		return errors.Wrap(err, "failed to create activity")
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start O4C activity")
	}
	defer activity.Stop(ctx, tconn)

	return testing.Poll(ctx, func(ctx context.Context) error {
		window, err := ash.GetARCAppWindowInfo(ctx, tconn, packageName)
		if err != nil {
			return errors.New("failed to get ARC window infomation")
		}
		if window.CanResize != shouldBeResizable {
			return testing.PollBreak(errors.Errorf("resizability isn't in the expected state: got %b; want %b", window.CanResize, shouldBeResizable))
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// reinstallAPK uninstalls and installs an APK.
func reinstallAPK(ctx context.Context, a *arc.ARC, packageName, apkName string) (bool, error) {
	installed, err := a.PackageInstalled(ctx, packageName)
	if err != nil {
		return false, errors.Wrap(err, "failed to query installed state")
	}

	if installed {
		testing.ContextLog(ctx, "uninstalling: ", packageName)
		if a.Uninstall(ctx, packageName); err != nil {
			return installed, errors.Wrap(err, "failed to uninstall app")
		}
	}

	testing.ContextLog(ctx, "installing: ", arc.APKPath(apkName))
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		return installed, errors.Wrap(err, "failed to install app")
	}

	return installed, nil
}
