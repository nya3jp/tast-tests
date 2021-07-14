// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"image/color"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	resizeLockTestPkgName             = "org.chromium.arc.testapp.resizelock"
	resizeLockApkName                 = "ArcResizeLockTest.apk"
	resizeLockMainActivityName        = "org.chromium.arc.testapp.resizelock.MainActivity"
	resizeLockUnresizableActivityName = "org.chromium.arc.testapp.resizelock.UnresizableActivity"

	// Verifying splash visbility requres 3 different resize-locked apps.
	resizeLock2PkgName = "org.chromium.arc.testapp.resizelock2"
	resizeLock3PkgName = "org.chromium.arc.testapp.resizelock3"
	resizeLock2ApkName = "ArcResizeLockTest2.apk"
	resizeLock3ApkName = "ArcResizeLockTest3.apk"

	// Used to (i) find the resize lock mode buttons on the compat-mode menu and (ii) check the state of the compat-mode button
	phoneButtonName     = "Phone"
	tabletButtonName    = "Tablet"
	resizableButtonName = "Resizable"

	// Currently the automation API doesn't support unique ID, so use the classnames to find the elements of interest.
	centerButtonClassName  = "FrameCenterButton"
	checkBoxClassName      = "Checkbox"
	bubbleDialogClassName  = "BubbleDialogDelegateView"
	overlayDialogClassName = "OverlayDialog"

	// A11y names are available for buttons
	splashCloseButtonName = "Got it"
	confirmButtonName     = "Allow"
	cancelButtonName      = "Cancel"
)

// Represents the size of a window.
type orientation int

const (
	phoneOrientation orientation = iota
	tabletOrientation
	maximizedOrientation
)

// Represents the high-level state of the app from the resize-lock feature's perspective.
type resizeLockMode int

const (
	phoneResizeLockMode resizeLockMode = iota
	tabletResizeLockMode
	resizableResizeLockMode
	nonEligibleResizeLockMode
)

// Represents the expected behavior and action to take for the resizability confirmation dialog.
type confirmationDialogAction int

const (
	dialogActionNoDialog confirmationDialogAction = iota
	dialogActionCancel
	dialogActionConfirm
	dialogActionConfirmWithDoNotAskMeAgainChecked
)

type resizeLockTestFunc func(context.Context, *chrome.TestConn, *arc.ARC, *ui.Device, *chrome.Chrome) error

type resizeLockTestCase struct {
	name string
	fn   resizeLockTestFunc
}

var testCases = []resizeLockTestCase{
	resizeLockTestCase{
		name: "Resize Locked App - CUJ",
		fn:   testResizeLockedAppCUJ,
	},
	resizeLockTestCase{
		name: "O4C App",
		fn:   testO4CApp,
	},
	resizeLockTestCase{
		name: "Maximized App",
		fn:   testMaximizedApp,
	},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeLock,
		Desc:         "Checks that ARC++ Resize Lock works as expected",
		Contacts:     []string{"takise@chromium.org", "toshikikikuchi@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      5 * time.Minute,
	})
}

func ResizeLock(ctx context.Context, s *testing.State) {
	// Ensure to enable the finch flag.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs("--enable-features=ArcResizeLock"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	dev, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to initialize UI Automator: ", err)
	}
	defer dev.Close(ctx)

	dispInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get primary display info: ", err)
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

	tabletModeStatus, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeStatus)

	// Place a maximized activity below to ensure that the display has a white background.
	// This is necessary because currently checking the visibility of the translucent window border relies on taking a screenshot.
	// The WM23 app is used here as the WM24 app is used for testing O4C (Optimized for Chromebook).
	if alreadyInstalled, err := reinstallAPK(ctx, a, wm.Pkg23, wm.APKNameArcWMTestApp23); err != nil {
		s.Fatal("Failed to reinstall the WM23 app: ", err)
	} else if !alreadyInstalled {
		defer a.Uninstall(ctx, arc.APKPath(wm.APKNameArcWMTestApp23))
	}
	activity, err := arc.NewActivity(a, wm.Pkg23, wm.NonResizableLandscapeActivity)
	if err != nil {
		s.Fatal("Failed to create the WM23 unresizable landscape activity: ", err)
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start the WM23 unresizable landscape activity: ", err)
	}
	defer activity.Stop(ctx, tconn)

	for _, test := range testCases {
		s.Logf("Running test %q", test.name)

		if err := test.fn(ctx, tconn, a, dev, cr); err != nil {
			path := fmt.Sprintf("%s/screenshot-resize-lock-failed-test-%s.png", s.OutDir(), test.name)
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Errorf("Failed to run test %s: %v", test.name, err)
		}
	}
}

// testO4CApp verifies that an O4C app is not resize locked even if it's newly-installed.
func testO4CApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome) error {
	return testNonResizeLocked(ctx, tconn, a, d, cr, wm.Pkg24, wm.APKNameArcWMTestApp24, wm.ResizableUnspecifiedActivity)
}

// testMaximizedApp verifies that an maximized app is not resize locked even if it's newly-installed.
func testMaximizedApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome) error {
	return testNonResizeLocked(ctx, tconn, a, d, cr, resizeLockTestPkgName, resizeLockApkName, resizeLockUnresizableActivityName)
}

// testNonResizeLocked verifies that the given app is not resize locked.
func testNonResizeLocked(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, packageName, apkName, activityName string) error {
	if alreadyInstalled, err := reinstallAPK(ctx, a, packageName, apkName); err != nil {
		return errors.Wrap(err, "failed to reinstall APK")
	} else if !alreadyInstalled {
		defer a.Uninstall(ctx, arc.APKPath(apkName))
	}

	activity, err := arc.NewActivity(a, packageName, activityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", activityName)
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start %s", activityName)
	}
	defer activity.Stop(ctx, tconn)

	// Verify the initial state of the given non-resize-locked app.
	if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, nonEligibleResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify the resize lock state of %s", activityName)
	}
	return nil
}

// testResizeLockedAppCUJ goes though the critical user journey of a resize-locked app, and verifies the app behaves expectedly.
func testResizeLockedAppCUJ(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome) error {
	// Launch 3 different resize-locked apps and verify that the splash screen is shown twice per user, once per app at most.
	for i, test := range []struct {
		apkName      string
		pkgName      string
		activityName string
	}{
		{resizeLockApkName, resizeLockTestPkgName, resizeLockMainActivityName},
		{resizeLock2ApkName, resizeLock2PkgName, resizeLockMainActivityName},
		{resizeLock3ApkName, resizeLock3PkgName, resizeLockMainActivityName},
	} {
		if err := checkSplashVisibility(ctx, tconn, a, d, cr, test.apkName, test.pkgName, test.activityName, i < 2 /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify the splash screen visibiity of %s", test.apkName)
		}
	}

	if alreadyInstalled, err := reinstallAPK(ctx, a, resizeLockTestPkgName, resizeLockApkName); err != nil {
		return errors.Wrap(err, "failed to reinstall APK")
	} else if !alreadyInstalled {
		defer a.Uninstall(ctx, arc.APKPath(resizeLockApkName))
	}

	activity, err := arc.NewActivity(a, resizeLockTestPkgName, resizeLockMainActivityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", resizeLockMainActivityName)
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start %s", resizeLockMainActivityName)
	}
	defer activity.Stop(ctx, tconn)

	// Verify the initial state of a normal resize-locked app.
	if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, phoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify resize lock state of %s", resizeLockMainActivityName)
	}

	// Toggle between Phone and Tablet.
	if err := toggleResizeLockMode(ctx, tconn, a, d, cr, activity, phoneResizeLockMode, tabletResizeLockMode, dialogActionNoDialog); err != nil {
		return errors.Wrapf(err, "failed to change the resize lock mode of %s from phone to tablet", resizeLockMainActivityName)
	}
	if err := toggleResizeLockMode(ctx, tconn, a, d, cr, activity, tabletResizeLockMode, phoneResizeLockMode, dialogActionNoDialog); err != nil {
		return errors.Wrapf(err, "failed to change the resize lock mode of %s from tablet to phone", resizeLockMainActivityName)
	}

	// Toggle between Phone and Resizable without "Don't ask me again" checked.
	if err := toggleResizeLockMode(ctx, tconn, a, d, cr, activity, phoneResizeLockMode, resizableResizeLockMode, dialogActionConfirm); err != nil {
		return errors.Wrapf(err, "failed to change the resize lock mode of %s from phone to resizable with the checkbox off", resizeLockMainActivityName)
	}
	if err := toggleResizeLockMode(ctx, tconn, a, d, cr, activity, resizableResizeLockMode, phoneResizeLockMode, dialogActionNoDialog); err != nil {
		return errors.Wrapf(err, "failed to change the resize lock mode of %s from resizable to phone", resizeLockMainActivityName)
	}

	// Toggle between Phone and Resizable with "Don't ask me again" checked.
	if err := toggleResizeLockMode(ctx, tconn, a, d, cr, activity, phoneResizeLockMode, resizableResizeLockMode, dialogActionConfirmWithDoNotAskMeAgainChecked); err != nil {
		return errors.Wrapf(err, "failed to change the resize lock mode of %s from phone to resizable with the checkbox on", resizeLockMainActivityName)
	}
	if err := toggleResizeLockMode(ctx, tconn, a, d, cr, activity, resizableResizeLockMode, phoneResizeLockMode, dialogActionNoDialog); err != nil {
		return errors.Wrapf(err, "failed to change the resize lock mode of %s from resizable to phone", resizeLockMainActivityName)
	}
	if err := toggleResizeLockMode(ctx, tconn, a, d, cr, activity, phoneResizeLockMode, resizableResizeLockMode, dialogActionNoDialog); err != nil {
		return errors.Wrapf(err, "failed to change the resize lock mode of %s from phone to resizable", resizeLockMainActivityName)
	}

	return nil
}

// checkSplashVisibility installs the given app, launchs the given activity twice, and verifies the visibility of the splash screen.
func checkSplashVisibility(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, apkName, pkgName, activityName string, isSplashVisible bool) error {
	if alreadyInstalled, err := reinstallAPK(ctx, a, pkgName, apkName); err != nil {
		return errors.Wrap(err, "failed to reinstall APK")
	} else if !alreadyInstalled {
		defer a.Uninstall(ctx, arc.APKPath(apkName))
	}

	activity, err := arc.NewActivity(a, pkgName, activityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", activityName)
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start %s", activityName)
	}
	defer activity.Stop(ctx, tconn)

	if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, phoneResizeLockMode, isSplashVisible /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify resize lock state of %s", resizeLockMainActivityName)
	}

	// Close and reopen the activity, and verify that the splash is not shown on the same app more than once.
	if err := activity.Stop(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to stop %s", activityName)
	}

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start %s", activityName)
	}
	defer activity.Stop(ctx, tconn)

	if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, phoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify resize lock state of %s", resizeLockMainActivityName)
	}

	return nil
}

// checkResizeLockState verifies the various properties that depend on resize lock state.
func checkResizeLockState(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, mode resizeLockMode, isSplashVisible bool) error {
	resizeLocked := mode == phoneResizeLockMode || mode == tabletResizeLockMode

	if err := checkResizability(ctx, tconn, a, d, activity, mode); err != nil {
		return errors.Wrapf(err, "failed to verify the resizability of %s", activity.ActivityName())
	}

	if err := checkVisibility(ctx, tconn, bubbleDialogClassName, isSplashVisible); err != nil {
		return errors.Wrapf(err, "failed to verify the visibility of the splash screen on %s", activity.ActivityName())
	}

	if err := checkCompatModeButton(ctx, tconn, a, d, cr, activity, mode); err != nil {
		return errors.Wrapf(err, "failed to verify the type of the compat mode button of %s", activity.ActivityName())
	}

	if err := checkBorder(ctx, tconn, a, d, cr, activity, resizeLocked /* shouldShowBorder */); err != nil {
		return errors.Wrapf(err, "failed to verify the visibility of the resize lock window border of %s", activity.ActivityName())
	}

	// There's no orientation rule for non-resize-locked apps, so only check the phone and tablet modes.
	if mode == tabletResizeLockMode {
		if err := checkOrientation(ctx, tconn, a, d, cr, activity, tabletOrientation); err != nil {
			return errors.Wrapf(err, "failed to verify %s has tablet orientation", activity.ActivityName())
		}
	} else if mode == phoneResizeLockMode {
		if err := checkOrientation(ctx, tconn, a, d, cr, activity, phoneOrientation); err != nil {
			return errors.Wrapf(err, "failed to verify %s has tablet orientation", activity.ActivityName())
		}
	}

	if err := checkMaximizeRestoreButtonVisibility(ctx, tconn, a, d, cr, activity, mode); err != nil {
		return errors.Wrapf(err, "failed to verify the visibility of maximize/restore button for %s", activity.ActivityName())
	}

	return nil
}

// getExpectedResizability returns the resizability based on the resize lock state and the pure resizability of the app.
func getExpectedResizability(activity *arc.Activity, mode resizeLockMode) bool {
	// Resize-locked apps are unresizable.
	if mode == phoneResizeLockMode || mode == tabletResizeLockMode {
		return false
	}

	// The activity with resizability false in its manifest is unresizable.
	if activity.ActivityName() == resizeLockUnresizableActivityName {
		return false
	}

	return true
}

// checkMaximizeRestoreButtonVisibility verifies the visibility of the maximize/restore button of the given app.
func checkMaximizeRestoreButtonVisibility(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, mode resizeLockMode) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		expected := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonClose
		// The visibility of the maximize/restore button matches the resizability of the app.
		if getExpectedResizability(activity, mode) {
			expected |= ash.CaptionButtonMaximizeAndRestore
		}
		return wm.CompareCaption(ctx, tconn, activity.PackageName(), expected)
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// checkOrientation verifies the orientation of the given app.
func checkOrientation(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, orientation orientation) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		window, err := ash.GetARCAppWindowInfo(ctx, tconn, activity.PackageName())
		if err != nil {
			return errors.Wrapf(err, "failed to ARC window infomation for package name %s", activity.PackageName())
		}
		switch orientation {
		case phoneOrientation:
			if err := ash.WaitForARCAppWindowState(ctx, tconn, activity.PackageName(), ash.WindowStateNormal); err != nil {
				return errors.Wrap(err, "failed to verify the window state of the phone orientation")
			}
			if window.BoundsInRoot.Width > window.BoundsInRoot.Height {
				return errors.New("failed to verify the window bounds of the phone orientation")
			}
		case tabletOrientation:
			if err := ash.WaitForARCAppWindowState(ctx, tconn, activity.PackageName(), ash.WindowStateNormal); err != nil {
				return errors.Wrap(err, "failed to verify the window state of the tablet orientation")
			}
			if window.BoundsInRoot.Height > window.BoundsInRoot.Width {
				return errors.New("failed to verify the window bounds of the tablet orientation")
			}
		case maximizedOrientation:
			return ash.WaitForARCAppWindowState(ctx, tconn, activity.PackageName(), ash.WindowStateMaximized)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// checkCompatModeButton verifies the state of the compat-mode button of the given app.
func checkCompatModeButton(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, mode resizeLockMode) error {
	if mode == nonEligibleResizeLockMode {
		return checkVisibility(ctx, tconn, centerButtonClassName, false /* visible */)
	}

	return testing.Poll(ctx, func(ctx context.Context) error {
		button, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: centerButtonClassName})
		if err != nil {
			return errors.Wrap(err, "failed to find the compat-mode button")
		}
		button.Release(ctx)

		if button.Name != getResizeLockModeButtonName(mode) {
			return errors.New("failed to verify the name of compat-mode button")
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// checkBorder checks whether the special window border for compatibility mode is shown or not.
// This functions takes a screenshot of the display, and counts the number of pixels that are dark gray around the window border.
func checkBorder(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, shouldShowBorder bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		const (
			pixelColorDiffMargin           = 5
			borderColorPixelCountThreshold = 1000
			borderWidthPX                  = 6
		)

		bounds, err := activity.WindowBounds(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get the window bounds of %s", activity.ActivityName())
		}

		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			return testing.PollBreak(err)
		}

		rect := img.Bounds()
		borderColorPixels := 0
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				onLeftBorder := bounds.Left-borderWidthPX <= x && x < bounds.Left
				onTopBorder := bounds.Top-borderWidthPX <= y && y < bounds.Top
				onRightBorder := bounds.Right() < x && x <= bounds.Right()+borderWidthPX
				onBottomBorder := bounds.Bottom() < y && y <= bounds.Bottom()+borderWidthPX
				onBorder := onLeftBorder || onTopBorder || onRightBorder || onBottomBorder
				if onBorder && colorcmp.ColorsMatch(img.At(x, y), color.RGBA{155, 155, 155, 255}, pixelColorDiffMargin) {
					borderColorPixels++
				}
			}
		}

		if shouldShowBorder && borderColorPixels < borderColorPixelCountThreshold {
			return errors.Errorf("failed to verify that the window border is visible; contains %d border color pixels", borderColorPixels)
		}
		if !shouldShowBorder && borderColorPixels > borderColorPixelCountThreshold {
			return errors.Errorf("failed to verify that the window border is invisible; contains %d border color pixels", borderColorPixels)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// checkVisibility checks whether the node specified by the given class name exists or not.
func checkVisibility(ctx context.Context, tconn *chrome.TestConn, className string, visible bool) error {
	if visible {
		return chromeui.WaitUntilExists(ctx, tconn, chromeui.FindParams{ClassName: className}, 10*time.Second)
	}
	return chromeui.WaitUntilGone(ctx, tconn, chromeui.FindParams{ClassName: className}, 10*time.Second)
}

// checkResizability verifies the given app's resizability.
func checkResizability(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, activity *arc.Activity, mode resizeLockMode) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		window, err := ash.GetARCAppWindowInfo(ctx, tconn, activity.PackageName())
		if err != nil {
			return errors.Wrapf(err, "failed to get the ARC window infomation for package name %s", activity.PackageName())
		}
		shouldBeResizable := getExpectedResizability(activity, mode)
		if window.CanResize != shouldBeResizable {
			return errors.Errorf("failed to verify the resizability; got %t, want %t", window.CanResize, shouldBeResizable)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// reinstallAPK uninstalls and installs an APK. It returns a boolean that represents whether it's already installed or not.
func reinstallAPK(ctx context.Context, a *arc.ARC, packageName, apkName string) (bool, error) {
	alreadyInstalled, err := a.PackageInstalled(ctx, packageName)
	if err != nil {
		return false, errors.Wrap(err, "failed to query installed state")
	}

	if alreadyInstalled {
		testing.ContextLog(ctx, "uninstalling: ", packageName)
		if a.Uninstall(ctx, packageName); err != nil {
			return alreadyInstalled, errors.Wrap(err, "failed to uninstall app")
		}
	}

	if err := a.Install(ctx, arc.APKPath(apkName), adb.InstallOptionFromPlayStore); err != nil {
		return alreadyInstalled, errors.Wrap(err, "failed to install app")
	}

	return alreadyInstalled, nil
}

// showCompatModeMenuViaButtonClick clicks on the compat-mode button and shows the compat-mode menu.
func showCompatModeMenuViaButtonClick(ctx context.Context, tconn *chrome.TestConn) error {
	icon, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: centerButtonClassName}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the compat-mode button")
	}
	defer icon.Release(ctx)

	if err := icon.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click on the compat-mode button")
	}

	return checkVisibility(ctx, tconn, bubbleDialogClassName, true /* visible */)
}

// waitForCompatModeMenuToDisappear waits for the compat-mode menu to disappear.
// After one of the resize lock mode buttons are selected, the compat mode menu disappears after a few seconds of delay.
// Can't use chromeui.WaitUntilGone() for this purpose because this function also checks whether the dialog has the "Phone" button or not to ensure that we are checking the correct dialog.
func waitForCompatModeMenuToDisappear(ctx context.Context, tconn *chrome.TestConn) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		dialog, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: bubbleDialogClassName})
		if err == nil && dialog != nil {
			defer dialog.Release(ctx)

			phoneButton, err := dialog.Descendant(ctx, chromeui.FindParams{Name: phoneButtonName})
			if err == nil && phoneButton != nil {
				phoneButton.Release(ctx)
				return errors.Wrap(err, "compat mode menu is sitll visible")
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// closeSplashViaButtonClick clicks on the close button and closes the splash screen.
func closeSplashViaButtonClick(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		splash, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: bubbleDialogClassName})
		if err != nil {
			return errors.Wrap(err, "failed to find the splash dialog")
		}
		defer splash.Release(ctx)

		button, err := splash.Descendant(ctx, chromeui.FindParams{Name: splashCloseButtonName})
		if err != nil {
			return errors.Wrap(err, "failed to find the close button of the splash dialog")
		}
		defer button.Release(ctx)

		if err := button.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click on the close button of the splash dialog")
		}

		if err := checkVisibility(ctx, tconn, bubbleDialogClassName, false /* visible */); err != nil {
			return errors.Wrap(err, "failed to verify that the splash screen is gone")
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// toggleResizeLockMode shows the compat-mode menu, clicks on one of the resize lock mode buttons on the compat-mode menu, and verifies the post state.
func toggleResizeLockMode(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, currentMode, nextMode resizeLockMode, action confirmationDialogAction) error {
	if err := showCompatModeMenuViaButtonClick(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to show the compat-mode dialog of %s", activity.ActivityName())
	}

	compatModeMenuDialog, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: bubbleDialogClassName}, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find the compat-mode menu dialog of %s", activity.ActivityName())
	}
	defer compatModeMenuDialog.Release(ctx)

	nextResizeLockModeName := getResizeLockModeButtonName(nextMode)
	resizeLockModeButton, err := compatModeMenuDialog.DescendantWithTimeout(ctx, chromeui.FindParams{Name: nextResizeLockModeName}, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find the %s button on the compat mode menu", nextResizeLockModeName)
	}
	defer resizeLockModeButton.Release(ctx)

	if err := resizeLockModeButton.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to click on the %s button on the compat mode menu", nextResizeLockModeName)
	}

	expectedMode := nextMode
	if action != dialogActionNoDialog {
		if err := waitForCompatModeMenuToDisappear(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to wait for the compat-mode menu of %s to disappear", activity.ActivityName())
		}

		confirmationDialog, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: overlayDialogClassName}, 10*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find the resizability confirmation dialog")
		}
		defer confirmationDialog.Release(ctx)

		if action == dialogActionCancel {
			cancelButton, err := confirmationDialog.DescendantWithTimeout(ctx, chromeui.FindParams{Name: cancelButtonName}, 10*time.Second)
			if err != nil {
				return errors.Wrap(err, "failed to find the cancel button on the compat mode menu")
			}

			if err := cancelButton.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to click on the cancel button of the resizability confirmation dialog")
			}
			expectedMode = currentMode
		} else if action == dialogActionConfirm || action == dialogActionConfirmWithDoNotAskMeAgainChecked {
			if action == dialogActionConfirmWithDoNotAskMeAgainChecked {
				checkbox, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: checkBoxClassName}, 10*time.Second)
				if err != nil {
					return errors.Wrap(err, "failed to find the checkbox of the resizability confirmation dialog")
				}
				defer checkbox.Release(ctx)

				if err := checkbox.LeftClick(ctx); err != nil {
					return errors.Wrap(err, "failed to click on the checkbox of the resizability confirmation dialog")
				}
			}

			confirmButton, err := confirmationDialog.DescendantWithTimeout(ctx, chromeui.FindParams{Name: confirmButtonName}, 10*time.Second)
			if err != nil {
				return errors.Wrap(err, "failed to find the confirm button on the compat mode menu")
			}

			if err := confirmButton.LeftClick(ctx); err != nil {
				return errors.Wrap(err, "failed to click on the confirm button of the resizability confirmation dialog")
			}
		}
	}

	if err := checkVisibility(ctx, tconn, bubbleDialogClassName, false /* visible */); err != nil {
		return errors.Wrap(err, "failed to verify that the resizability confirmation dialog is invisible")
	}

	return checkResizeLockState(ctx, tconn, a, d, cr, activity, expectedMode, false /* isSplashVisible */)
}

// getResizeLockModeButtonName converts resizeLockMode to the corresponding button name.
func getResizeLockModeButtonName(mode resizeLockMode) string {
	switch mode {
	case phoneResizeLockMode:
		return phoneButtonName
	case tabletResizeLockMode:
		return tabletButtonName
	case resizableResizeLockMode:
		return resizableButtonName
	default:
		return ""
	}
}
