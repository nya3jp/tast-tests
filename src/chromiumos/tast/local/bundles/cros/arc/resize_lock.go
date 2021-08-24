// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

const (
	resizeLockTestPkgName                               = "org.chromium.arc.testapp.resizelock"
	resizeLockApkName                                   = "ArcResizeLockTest.apk"
	resizeLockMainActivityName                          = "org.chromium.arc.testapp.resizelock.MainActivity"
	resizeLockUnresizableUnspecifiedActivityName        = "org.chromium.arc.testapp.resizelock.UnresizableUnspecifiedActivity"
	resizeLockUnresizablePortraitActivityName           = "org.chromium.arc.testapp.resizelock.UnresizablePortraitActivity"
	resizeLockResizableUnspecifiedMaximizedActivityName = "org.chromium.arc.testapp.resizelock.ResizableUnspecifiedMaximizedActivity"

	// Verifying splash visbility requires 3 different resize-locked apps.
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
	shelfIconClassName     = "ash/ShelfAppButton"
	menuItemViewClassName  = "MenuItemView"

	// A11y names are available for some UI elements
	splashCloseButtonName          = "Got it"
	confirmButtonName              = "Allow"
	cancelButtonName               = "Cancel"
	appManagementSettingToggleName = "Preset window sizes"
	appInfoMenuItemViewName        = "App info"
	closeMenuItemViewName          = "Close"

	// Used to identify the shelf icon of interest.
	resizeLockAppName = "ArcResizeLockTest"
	settingsAppName   = "Settings"

	// Used in test cases where screenshots are taken.
	pixelColorDiffMargin                    = 5
	clientContentColorPixelPercentThreshold = 95
	borderColorPixelCountThreshold          = 1000
	borderWidthPX                           = 6
)

// Represents the size of a window.
type orientation int

const (
	phoneOrientation orientation = iota
	tabletOrientation
	maximizedOrientation
)

func (mode orientation) String() string {
	switch mode {
	case phoneOrientation:
		return "phone"
	case tabletOrientation:
		return "tablet"
	case maximizedOrientation:
		return "maximized"
	default:
		return "unknown"
	}
}

// Represents the high-level state of the app from the resize-lock feature's perspective.
type resizeLockMode int

const (
	phoneResizeLockMode resizeLockMode = iota
	tabletResizeLockMode
	resizableResizeLockMode
	nonEligibleResizeLockMode
)

func (mode resizeLockMode) String() string {
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

// Represents the expected behavior and action to take for the resizability confirmation dialog.
type confirmationDialogAction int

const (
	dialogActionNoDialog confirmationDialogAction = iota
	dialogActionCancel
	dialogActionConfirm
	dialogActionConfirmWithDoNotAskMeAgainChecked
)

// Represents how to interact with UI.
type inputMethodType int

const (
	inputMethodClick inputMethodType = iota
	inputMethodKeyEvent
)

func (mode inputMethodType) String() string {
	switch mode {
	case inputMethodClick:
		return "click"
	case inputMethodKeyEvent:
		return "keyboard"
	default:
		return "unknown"
	}
}

type resizeLockTestFunc func(context.Context, *chrome.TestConn, *arc.ARC, *ui.Device, *chrome.Chrome, *input.KeyboardEventWriter) error

type resizeLockTestCase struct {
	name string
	fn   resizeLockTestFunc
}

// The order of the test cases matters since some persistent chrome-side properties are tested.
// - "Splash" must come first as any launch of the apps affect the test case.
var testCases = []resizeLockTestCase{
	resizeLockTestCase{
		name: "Splash",
		fn:   testSplash,
	},
	resizeLockTestCase{
		name: "Resize Locked App - CUJ",
		fn:   testResizeLockedAppCUJ,
	},
	resizeLockTestCase{
		name: "Resize Locked App - Fully Locked",
		fn:   testFullyLockedApp,
	},
	resizeLockTestCase{
		name: "O4C App",
		fn:   testO4CApp,
	},
	resizeLockTestCase{
		name: "Unresizable Maximized App",
		fn:   testUnresizableMaximizedApp,
	},
	resizeLockTestCase{
		name: "Resizable Maximized App",
		fn:   testResizableMaximizedApp,
	},
	resizeLockTestCase{
		name: "Install from outside of PlayStore",
		fn:   testAppFromOutsideOfPlayStore,
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

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer keyboard.Close()

	// Place a maximized activity below to ensure that the display has a white background.
	// This is necessary because currently checking the visibility of the translucent window border relies on taking a screenshot.
	// The WM23 app is used here as the WM24 app is used for testing O4C (Optimized for Chromebook).
	if alreadyInstalled, err := reinstallAPK(ctx, a, wm.Pkg23, wm.APKNameArcWMTestApp23, true /* fromPlayStore */); err != nil {
		s.Fatal("Failed to reinstall the WM23 app: ", err)
	} else if !alreadyInstalled {
		defer a.Uninstall(ctx, wm.Pkg23)
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

		if err := test.fn(ctx, tconn, a, dev, cr, keyboard); err != nil {
			path := fmt.Sprintf("%s/screenshot-resize-lock-failed-test-%s.png", s.OutDir(), test.name)
			if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
				s.Log("Failed to capture screenshot: ", err)
			}
			s.Errorf("Failed to run test %s: %v", test.name, err)
		}
	}
}

// testO4CApp verifies that an O4C app is not resize locked even if it's newly-installed.
func testO4CApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	return testNonResizeLocked(ctx, tconn, a, d, cr, keyboard, wm.Pkg24, wm.APKNameArcWMTestApp24, wm.ResizableUnspecifiedActivity, true /* fromPlayStore */, false /* checkRestoreMaximize */)
}

// testUnresizableMaximizedApp verifies that an unresizable, maximized app is not resize locked even if it's newly-installed.
func testUnresizableMaximizedApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	return testNonResizeLocked(ctx, tconn, a, d, cr, keyboard, resizeLockTestPkgName, resizeLockApkName, resizeLockUnresizableUnspecifiedActivityName, true /* fromPlayStore */, false /* checkRestoreMaximize */)
}

// testResizableMaximizedApp verifies that a resizable, maximized app is not resize locked even if it's newly-installed.
func testResizableMaximizedApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	return testNonResizeLocked(ctx, tconn, a, d, cr, keyboard, resizeLockTestPkgName, resizeLockApkName, resizeLockResizableUnspecifiedMaximizedActivityName, true /* fromPlayStore */, true /* checkRestoreMaximize */)
}

// testAppFromOutsideOfPlayStore verifies that an resize-lock-eligible app installed from outside of PlayStore is not resize locked even if it's newly-installed.
func testAppFromOutsideOfPlayStore(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	return testNonResizeLocked(ctx, tconn, a, d, cr, keyboard, resizeLockTestPkgName, resizeLockApkName, resizeLockMainActivityName, false /* fromPlayStore */, false /* checkRestoreMaximize */)
}

// testNonResizeLocked verifies that the given app is not resize locked.
func testNonResizeLocked(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter, packageName, apkName, activityName string, fromPlayStore, checkRestoreMaximize bool) error {
	if alreadyInstalled, err := reinstallAPK(ctx, a, packageName, apkName, fromPlayStore); err != nil {
		return errors.Wrap(err, "failed to reinstall APK")
	} else if !alreadyInstalled {
		defer a.Uninstall(ctx, packageName)
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

	if checkRestoreMaximize {
		// Restore the app and verify the app is resize-locked.
		if _, err := ash.SetARCAppWindowState(ctx, tconn, packageName, ash.WMEventNormal); err != nil {
			return errors.Wrapf(err, "failed to restore %s", activityName)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, ash.WindowStateNormal); err != nil {
			return errors.Wrapf(err, "failed to wait for %s to be restored", activityName)
		}
		if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, tabletResizeLockMode, false /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", activityName)
		}
		// Make the app resizable to enable maximization.
		if err := toggleResizeLockMode(ctx, tconn, a, d, cr, activity, tabletResizeLockMode, resizableResizeLockMode, dialogActionConfirm, inputMethodClick, keyboard); err != nil {
			return errors.Wrapf(err, "failed to change the resize lock mode of %s from tablet to resizable", apkName)
		}
		// Maximize the app and verify the app is not resize-locked.
		if _, err := ash.SetARCAppWindowState(ctx, tconn, packageName, ash.WMEventMaximize); err != nil {
			return errors.Wrapf(err, "failed to maximize %s", activityName)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, ash.WindowStateMaximized); err != nil {
			return errors.Wrapf(err, "failed to wait for %s to be maximized", activityName)
		}
		if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, nonEligibleResizeLockMode, false /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", activityName)
		}
	}
	return nil
}

// testFullyLockedApp verifies that the given app is fully locked.
func testFullyLockedApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	if alreadyInstalled, err := reinstallAPK(ctx, a, resizeLockTestPkgName, resizeLockApkName, true /* fromPlayStore */); err != nil {
		return errors.Wrap(err, "failed to reinstall APK")
	} else if !alreadyInstalled {
		defer a.Uninstall(ctx, resizeLockTestPkgName)
	}

	activity, err := arc.NewActivity(a, resizeLockTestPkgName, resizeLockUnresizablePortraitActivityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", resizeLockUnresizablePortraitActivityName)
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start %s", resizeLockUnresizablePortraitActivityName)
	}
	defer activity.Stop(ctx, tconn)

	// Verify the initial state of the given non-resize-locked app.
	if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, phoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify the resize lock state of %s", resizeLockUnresizablePortraitActivityName)
	}

	// The compat-mode button of a fully-locked app is disabled.
	icon, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: centerButtonClassName}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the compat-mode button")
	}
	defer icon.Release(ctx)

	if err := icon.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click on the compat-mode button")
	}

	// Need some sleep here as we verify that nothing changes.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep after clicking on the compat-mode button")
	}

	if err := checkVisibility(ctx, tconn, bubbleDialogClassName, false); err != nil {
		return errors.Wrapf(err, "failed to verify the visibility of the compat-mode menu of %s", activity.ActivityName())
	}

	// The setting toggle of a fully-locked app is invisible in the app-management page.
	if err := openAppManagementSetting(ctx, tconn, resizeLockAppName); err != nil {
		return errors.Wrapf(err, "failed to open the app management page of %s", resizeLockAppName)
	}
	defer closeAppManagementSetting(ctx, tconn)
	return chromeui.WaitUntilGone(ctx, tconn, chromeui.FindParams{Name: appManagementSettingToggleName}, 10*time.Second)
}

// testSplash installs 3 different resize-locked app, launches an activity twice, and verifies that the splash screen works as expected.
// The spec of visibility: The splash must be shown twice per user, once per app at most.
func testSplash(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	const (
		// The splash must be shown twice per user at most.
		showSplashLimit = 2
	)

	for i, test := range []struct {
		apkName      string
		pkgName      string
		activityName string
		method       inputMethodType
	}{
		{resizeLockApkName, resizeLockTestPkgName, resizeLockMainActivityName, inputMethodClick},
		{resizeLock2ApkName, resizeLock2PkgName, resizeLockMainActivityName, inputMethodKeyEvent},
		{resizeLock3ApkName, resizeLock3PkgName, resizeLockMainActivityName, inputMethodClick},
	} {
		ctxDefer := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()
		if alreadyInstalled, err := reinstallAPK(ctx, a, test.pkgName, test.apkName, true /* fromPlayStore */); err != nil {
			return errors.Wrap(err, "failed to reinstall APK")
		} else if !alreadyInstalled {
			defer a.Uninstall(ctxDefer, test.pkgName)
		}

		activity, err := arc.NewActivity(a, test.pkgName, test.activityName)
		if err != nil {
			return errors.Wrapf(err, "failed to create %s", test.activityName)
		}
		defer activity.Close()

		if err := activity.Start(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to start %s", test.activityName)
		}
		defer activity.Stop(ctx, tconn)

		if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, phoneResizeLockMode, i < showSplashLimit /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", resizeLockMainActivityName)
		}

		if i < showSplashLimit {
			if err := closeSplash(ctx, tconn, test.method, keyboard); err != nil {
				return errors.Wrapf(err, "failed to close the splash screen of %s via %s", resizeLockMainActivityName, test.method)
			}
		}

		// Close and reopen the activity, and verify that the splash is not shown on the same app more than once.
		if err := activity.Stop(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to stop %s", test.activityName)
		}

		if err := activity.Start(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to start %s", test.activityName)
		}
		defer activity.Stop(ctx, tconn)

		if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, phoneResizeLockMode, false /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", resizeLockMainActivityName)
		}

		if err := activity.Stop(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to stop %s", test.activityName)
		}
	}
	return nil
}

// testResizeLockedAppCUJ goes though the critical user journey of a resize-locked app via both click and keyboard, and verifies the app behaves expectedly.
func testResizeLockedAppCUJ(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	for _, test := range []struct {
		packageName  string
		apkName      string
		activityName string
		method       inputMethodType
	}{
		{resizeLockTestPkgName, resizeLockApkName, resizeLockMainActivityName, inputMethodClick},
		{resizeLock2PkgName, resizeLock2ApkName, resizeLockMainActivityName, inputMethodKeyEvent},
	} {
		if err := testResizeLockedAppCUJInternal(ctx, tconn, a, d, cr, test.packageName, test.apkName, test.activityName, test.method, keyboard); err != nil {
			return errors.Wrapf(err, "failed to run the critical user journey for %s via %s", test.apkName, test.method)
		}
	}
	return nil
}

// testResizeLockedAppCUJInternal goes though the critical user journey of the given resize-locked app via the given input method.
func testResizeLockedAppCUJInternal(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, packageName, apkName, activityName string, method inputMethodType, keyboard *input.KeyboardEventWriter) error {
	if alreadyInstalled, err := reinstallAPK(ctx, a, resizeLockTestPkgName, resizeLockApkName, true /* fromPlayStore */); err != nil {
		return errors.Wrap(err, "failed to reinstall APK")
	} else if !alreadyInstalled {
		defer a.Uninstall(ctx, resizeLockTestPkgName)
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

	for _, test := range []struct {
		currentMode resizeLockMode
		nextMode    resizeLockMode
		action      confirmationDialogAction
	}{
		// Check the cancel button does nothing.
		{phoneResizeLockMode, resizableResizeLockMode, dialogActionCancel},
		// Toggle between Phone and Tablet.
		{phoneResizeLockMode, tabletResizeLockMode, dialogActionNoDialog},
		{tabletResizeLockMode, phoneResizeLockMode, dialogActionNoDialog},
		// Toggle between Phone and Resizable without "Don't ask me again" checked.
		{phoneResizeLockMode, resizableResizeLockMode, dialogActionConfirm},
		{resizableResizeLockMode, phoneResizeLockMode, dialogActionNoDialog},
		// Toggle between Phone and Resizable with "Don't ask me again" checked.
		{phoneResizeLockMode, resizableResizeLockMode, dialogActionConfirmWithDoNotAskMeAgainChecked},
		{resizableResizeLockMode, phoneResizeLockMode, dialogActionNoDialog},
		{phoneResizeLockMode, resizableResizeLockMode, dialogActionNoDialog},
	} {
		if err := toggleResizeLockMode(ctx, tconn, a, d, cr, activity, test.currentMode, test.nextMode, test.action, method, keyboard); err != nil {
			return errors.Wrapf(err, "failed to change the resize lock mode of %s from %s to %s", resizeLockApkName, test.currentMode, test.nextMode)
		}

		// Verify that relaunching an app doesn't cause any inconsistency.
		if err := activity.Stop(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to stop %s", resizeLockMainActivityName)
		}
		if err := activity.Start(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to start %s", resizeLockMainActivityName)
		}
		expectedMode := test.nextMode
		if test.action == dialogActionCancel {
			expectedMode = test.currentMode
		}
		if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, expectedMode, false /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", resizeLockMainActivityName)
		}
	}

	for _, test := range []struct {
		currentMode resizeLockMode
		nextMode    resizeLockMode
	}{
		{resizableResizeLockMode, phoneResizeLockMode},
		{phoneResizeLockMode, resizableResizeLockMode},
	} {
		// Toggle the resizability state via the Chrome OS setting toggle.
		if err := toggleAppManagementSettingToggle(ctx, tconn, a, d, cr, activity, resizeLockAppName, test.currentMode, test.nextMode, method, keyboard); err != nil {
			return errors.Wrapf(err, "failed to toggle the resizability state from %s to %s on the Chrome OS settings", test.currentMode, test.nextMode)
		}
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
	if activity.ActivityName() == resizeLockUnresizableUnspecifiedActivityName {
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
func checkOrientation(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, expectedOrientation orientation) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		actualOrientation, err := activityOrientation(ctx, tconn, activity)
		if err != nil {
			return errors.Wrapf(err, "failed to get the current orientation of %s", activity.PackageName())
		}
		if actualOrientation != expectedOrientation {
			errors.Errorf("failed to verify the orientation; want: %s, got: %s", expectedOrientation, actualOrientation)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// activityOrientation returns the current orientation of the given app.
func activityOrientation(ctx context.Context, tconn *chrome.TestConn, activity *arc.Activity) (orientation, error) {
	window, err := ash.GetARCAppWindowInfo(ctx, tconn, activity.PackageName())
	if err != nil {
		return maximizedOrientation, errors.Wrapf(err, "failed to ARC window infomation for package name %s", activity.PackageName())
	}
	if window.State == ash.WindowStateMaximized || window.State == ash.WindowStateFullscreen {
		return maximizedOrientation, nil
	}
	if window.BoundsInRoot.Width < window.BoundsInRoot.Height {
		return phoneOrientation, nil
	}
	return tabletOrientation, nil
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

		if button.Name != mode.String() {
			return errors.Errorf("failed to verify the name of compat-mode button; got: %s, want: %s", button.Name, mode)
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// checkClientContent verifies the client content fills the entire window.
// This is useful to check if switching between phone and tablet modes causes any UI glich.
func checkClientContent(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, activity *arc.Activity) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		bounds, err := activity.WindowBounds(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get the window bounds of %s", activity.ActivityName())
		}

		img, err := screenshot.GrabScreenshot(ctx, cr)
		if err != nil {
			return testing.PollBreak(err)
		}

		dispMode, err := ash.PrimaryDisplayMode(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get display mode of the primary display")
		}
		windowInfo, err := ash.GetARCAppWindowInfo(ctx, tconn, activity.PackageName())
		if err != nil {
			return errors.Wrapf(err, "failed to get arc app window info for %s", activity.PackageName())
		}
		captionHeight := int(math.Round(float64(windowInfo.CaptionHeight) * dispMode.DeviceScaleFactor))

		totalPixels := (bounds.Height - captionHeight) * bounds.Width
		bluePixels := imgcmp.CountPixels(img, color.RGBA{0, 0, 255, 255})
		bluePercent := bluePixels * 100 / totalPixels

		if bluePercent < clientContentColorPixelPercentThreshold {
			return errors.Errorf("failed to verify the number of the blue pixels exceeds the threshold (%d%%); contains %d / %d (%d%%) blue pixels", clientContentColorPixelPercentThreshold, bluePixels, totalPixels, bluePercent)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second})
}

// checkBorder checks whether the special window border for compatibility mode is shown or not.
// This functions takes a screenshot of the display, and counts the number of pixels that are dark gray around the window border.
func checkBorder(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, shouldShowBorder bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
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
func reinstallAPK(ctx context.Context, a *arc.ARC, packageName, apkName string, fromPlayStore bool) (bool, error) {
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

	if fromPlayStore {
		if err := a.Install(ctx, arc.APKPath(apkName), adb.InstallOptionFromPlayStore); err != nil {
			return alreadyInstalled, errors.Wrap(err, "failed to install app from PlayStore")
		}
	} else {
		if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
			return alreadyInstalled, errors.Wrap(err, "failed to install app from outside of PlayStore")
		}
	}

	return alreadyInstalled, nil
}

// showCompatModeMenu shows the compat-mode menu via the given method.
func showCompatModeMenu(ctx context.Context, tconn *chrome.TestConn, method inputMethodType, keyboard *input.KeyboardEventWriter) error {
	switch method {
	case inputMethodClick:
		return showCompatModeMenuViaButtonClick(ctx, tconn)
	case inputMethodKeyEvent:
		return showCompatModeMenuViaKeyboard(ctx, tconn, keyboard)
	}
	return errors.Errorf("invalid inputMethodType is given: %s", method)
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

// showCompatModeMenuViaKeyboard injects the keyboard shortcut and shows the compat-mode menu.
func showCompatModeMenuViaKeyboard(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := keyboard.Accel(ctx, "Search+Alt+C"); err != nil {
			return errors.Wrap(err, "failed to inject Search+Alt+C")
		}

		if err := chromeui.WaitUntilExists(ctx, tconn, chromeui.FindParams{ClassName: bubbleDialogClassName}, 2*time.Second); err != nil {
			return errors.Wrap(err, "failed to find the compat-mode dialog")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
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

// closeSplash closes the splash screen via the given method.
func closeSplash(ctx context.Context, tconn *chrome.TestConn, method inputMethodType, keyboard *input.KeyboardEventWriter) error {
	splash, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: bubbleDialogClassName})
	if err != nil {
		return errors.Wrap(err, "failed to find the splash dialog")
	}
	defer splash.Release(ctx)

	switch method {
	case inputMethodClick:
		return closeSplashViaClick(ctx, tconn, splash)
	case inputMethodKeyEvent:
		return closeSplashViaKeyboard(ctx, tconn, splash, keyboard)
	}
	return nil
}

// closeSplashViaKeyboard presses the Enter key and closes the splash screen.
func closeSplashViaKeyboard(ctx context.Context, tconn *chrome.TestConn, splash *chromeui.Node, keyboard *input.KeyboardEventWriter) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "failed to press the Enter key")
		}
		return checkVisibility(ctx, tconn, bubbleDialogClassName, false /* visible */)
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// closeSplashViaClick clicks on the close button and closes the splash screen.
func closeSplashViaClick(ctx context.Context, tconn *chrome.TestConn, splash *chromeui.Node) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		button, err := splash.Descendant(ctx, chromeui.FindParams{Name: splashCloseButtonName})
		if err != nil {
			return errors.Wrap(err, "failed to find the close button of the splash dialog")
		}
		defer button.Release(ctx)

		if err := button.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click on the close button of the splash dialog")
		}

		return checkVisibility(ctx, tconn, bubbleDialogClassName, false /* visible */)
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// toggleResizeLockMode shows the compat-mode menu, selects one of the resize lock mode buttons on the compat-mode menu via the given method, and verifies the post state.
func toggleResizeLockMode(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, currentMode, nextMode resizeLockMode, action confirmationDialogAction, method inputMethodType, keyboard *input.KeyboardEventWriter) error {
	preToggleOrientation, err := activityOrientation(ctx, tconn, activity)
	if err != nil {
		return errors.Wrapf(err, "failed to get the pre-toggle orientation of %s", activity.PackageName())
	}
	if err := showCompatModeMenu(ctx, tconn, method, keyboard); err != nil {
		return errors.Wrapf(err, "failed to show the compat-mode dialog of %s via %s", activity.ActivityName(), method)
	}

	compatModeMenuDialog, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: bubbleDialogClassName}, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find the compat-mode menu dialog of %s", activity.ActivityName())
	}
	defer compatModeMenuDialog.Release(ctx)

	switch method {
	case inputMethodClick:
		if err := selectResizeLockModeViaClick(ctx, nextMode, compatModeMenuDialog); err != nil {
			return errors.Wrapf(err, "failed to click on the compat-mode dialog of %s via click", activity.ActivityName())
		}
	case inputMethodKeyEvent:
		if err := shiftViaTabAndEnter(ctx, tconn, compatModeMenuDialog, chromeui.FindParams{Name: nextMode.String()}, keyboard); err != nil {
			return errors.Wrapf(err, "failed to click on the compat-mode dialog of %s via keyboard", activity.ActivityName())
		}
	}

	expectedMode := nextMode
	if action == dialogActionCancel {
		expectedMode = currentMode
	}
	if action != dialogActionNoDialog {
		if err := waitForCompatModeMenuToDisappear(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to wait for the compat-mode menu of %s to disappear", activity.ActivityName())
		}

		confirmationDialog, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: overlayDialogClassName}, 10*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find the resizability confirmation dialog")
		}
		defer confirmationDialog.Release(ctx)

		switch method {
		case inputMethodClick:
			if err := handleConfirmationDialogViaClick(ctx, tconn, nextMode, confirmationDialog, action); err != nil {
				return errors.Wrapf(err, "failed to handle the confirmation dialog of %s via click", activity.ActivityName())
			}
		case inputMethodKeyEvent:
			if err := handleConfirmationDialogViaKeyboard(ctx, tconn, nextMode, confirmationDialog, action, keyboard); err != nil {
				return errors.Wrapf(err, "failed to handle the confirmation dialog of %s via keyboard", activity.ActivityName())
			}
		}
	}

	// The compat-mode dialog stays shown for two seconds by default after resize lock mode is toggled.
	// Explicitly close the dialog using the Esc key.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			return errors.Wrap(err, "failed to press the Esc key")
		}

		return checkVisibility(ctx, tconn, bubbleDialogClassName, false /* visible */)
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to verify that the resizability confirmation dialog is invisible")
	}

	postToggleOrientation, err := activityOrientation(ctx, tconn, activity)
	if err != nil {
		return errors.Wrapf(err, "failed to get the post-toggle orientation of %s", activity.PackageName())
	}

	if preToggleOrientation != postToggleOrientation {
		if err := checkClientContent(ctx, tconn, cr, activity); err != nil {
			return errors.Wrapf(err, "failed to verify the client content fills the window of %s", activity.ActivityName())
		}
	}

	return checkResizeLockState(ctx, tconn, a, d, cr, activity, expectedMode, false /* isSplashVisible */)
}

// handleConfirmationDialogViaKeyboard does the given action for the confirmation dialog via keyboard.
func handleConfirmationDialogViaKeyboard(ctx context.Context, tconn *chrome.TestConn, mode resizeLockMode, confirmationDialog *chromeui.Node, action confirmationDialogAction, keyboard *input.KeyboardEventWriter) error {
	if action == dialogActionCancel {
		return shiftViaTabAndEnter(ctx, tconn, confirmationDialog, chromeui.FindParams{Name: cancelButtonName}, keyboard)
	} else if action == dialogActionConfirm || action == dialogActionConfirmWithDoNotAskMeAgainChecked {
		if action == dialogActionConfirmWithDoNotAskMeAgainChecked {
			if err := shiftViaTabAndEnter(ctx, tconn, confirmationDialog, chromeui.FindParams{ClassName: checkBoxClassName}, keyboard); err != nil {
				return errors.Wrap(err, "failed to select the checkbox of the resizability confirmation dialog via keyboard")
			}
		}
		return shiftViaTabAndEnter(ctx, tconn, confirmationDialog, chromeui.FindParams{Name: confirmButtonName}, keyboard)
	}
	return nil
}

// handleConfirmationDialogViaClick does the given action for the confirmation dialog via click.
func handleConfirmationDialogViaClick(ctx context.Context, tconn *chrome.TestConn, mode resizeLockMode, confirmationDialog *chromeui.Node, action confirmationDialogAction) error {
	if action == dialogActionCancel {
		cancelButton, err := confirmationDialog.DescendantWithTimeout(ctx, chromeui.FindParams{Name: cancelButtonName}, 10*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find the cancel button on the compat mode menu")
		}
		return cancelButton.LeftClick(ctx)
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

		return confirmButton.LeftClick(ctx)
	}
	return nil
}

// selectResizeLockModeViaClick clicks on the given resize lock mode button.
func selectResizeLockModeViaClick(ctx context.Context, mode resizeLockMode, compatModeMenuDialog *chromeui.Node) error {
	resizeLockModeButton, err := compatModeMenuDialog.DescendantWithTimeout(ctx, chromeui.FindParams{Name: mode.String()}, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find the %s button on the compat mode menu", mode)
	}
	defer resizeLockModeButton.Release(ctx)

	return resizeLockModeButton.LeftClick(ctx)
}

// shiftViaTabAndEnter keeps pressing the Tab key until the UI element of interest gets focus, and press the Enter key.
func shiftViaTabAndEnter(ctx context.Context, tconn *chrome.TestConn, parent *chromeui.Node, params chromeui.FindParams, keyboard *input.KeyboardEventWriter) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := keyboard.Accel(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to press the Tab key")
		}

		var node *chromeui.Node
		var err error
		if parent != nil {
			node, err = parent.DescendantWithTimeout(ctx, params, 10*time.Second)
		} else {
			node, err = chromeui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
		}
		if err != nil {
			return errors.Wrap(err, "failed to find the node seeking focus")
		}

		if !node.State[chromeui.StateTypeFocused] {
			return errors.New("failed to wait for the node to get focus")
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to shift focus to the node to click on")
	}
	return keyboard.Accel(ctx, "Enter")
}

// toggleAppManagementSettingToggle opens the app-management page for the given app via the shelf icon, toggles the resize lock setting, and verifies the states of the app and the setting toggle.
func toggleAppManagementSettingToggle(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, appName string, currentMode, nextMode resizeLockMode, method inputMethodType, keyboard *input.KeyboardEventWriter) error {
	// This check must be done before opening the Chrome OS settings page so it won't affect the screenshot taken in one of the checks.
	if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, currentMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify resize lock state of %s", appName)
	}

	if err := openAppManagementSetting(ctx, tconn, appName); err != nil {
		return errors.Wrapf(err, "failed to open the app management page of %s", appName)
	}

	if err := checkAppManagementSettingToggleState(ctx, tconn, currentMode); err != nil {
		return errors.Wrap(err, "failed to verify the state of the setting toggle before toggling the setting")
	}

	switch method {
	case inputMethodClick:
		if err := toggleAppManagementSettingToggleViaClick(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to toggle the resize-lock setting toggle on the Chrome OS settings via click")
		}
	case inputMethodKeyEvent:
		if err := shiftViaTabAndEnter(ctx, tconn, nil, chromeui.FindParams{Name: appManagementSettingToggleName}, keyboard); err != nil {
			return errors.Wrap(err, "failed to toggle the resize-lock setting toggle on the Chrome OS settings via keyboard")
		}
	}

	if err := checkAppManagementSettingToggleState(ctx, tconn, nextMode); err != nil {
		return errors.Wrap(err, "failed to verify the state of the setting toggle after toggling the setting")
	}

	if err := closeAppManagementSetting(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to close the app management page of %s", appName)
	}

	// This check must be done after closing the Chrome OS settings page so it won't affect the screenshot taken in one of the checks.
	if err := checkResizeLockState(ctx, tconn, a, d, cr, activity, nextMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify resize lock state of %s", resizeLockMainActivityName)
	}

	return nil
}

// toggleAppManagementSettingToggleViaClick toggles the resize-lock setting toggle via click.
func toggleAppManagementSettingToggleViaClick(ctx context.Context, tconn *chrome.TestConn) error {
	settingToggle, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{Name: appManagementSettingToggleName}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the setting toggle")
	}
	defer settingToggle.Release(ctx)

	return settingToggle.LeftClick(ctx)
}

// openAppManagementSetting opens the app management page if the given app.
func openAppManagementSetting(ctx context.Context, tconn *chrome.TestConn, appName string) error {
	resizeLockShelfIcon, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{Name: appName, ClassName: shelfIconClassName}, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find the shelf icon of %s", appName)
	}
	defer resizeLockShelfIcon.Release(ctx)

	if err := resizeLockShelfIcon.RightClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to click on the shelf icon of %s", appName)
	}

	appInfoMenuItem, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{Name: appInfoMenuItemViewName, ClassName: menuItemViewClassName}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the menu item for the app-management page")
	}
	defer appInfoMenuItem.Release(ctx)

	return appInfoMenuItem.LeftClick(ctx)
}

// closeAppManagementSetting closes any open app management page.
func closeAppManagementSetting(ctx context.Context, tconn *chrome.TestConn) error {
	settingShelfIcon, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{Name: settingsAppName, ClassName: shelfIconClassName}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the shelf icon of the settings app")
	}
	defer settingShelfIcon.Release(ctx)

	if err := settingShelfIcon.RightClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click on the shelf icon of the settings app")
	}

	closeMenuItem, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{Name: closeMenuItemViewName, ClassName: menuItemViewClassName}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the menu item for closing the settings app")
	}
	defer closeMenuItem.Release(ctx)

	return closeMenuItem.LeftClick(ctx)
}

// checkAppManagementSettingToggleState verifies the resize lock setting state on the app-management page.
// The app management page must be open when this function is called.
func checkAppManagementSettingToggleState(ctx context.Context, tconn *chrome.TestConn, mode resizeLockMode) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		settingToggle, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{Name: appManagementSettingToggleName}, 2*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find the resize lock setting toggle on the app-management page")
		}
		defer settingToggle.Release(ctx)

		if ((mode == phoneResizeLockMode || mode == tabletResizeLockMode) && settingToggle.Checked == chromeui.CheckedStateFalse) ||
			(mode == resizableResizeLockMode && settingToggle.Checked == chromeui.CheckedStateTrue) {
			return errors.Errorf("the app-management resize lock setting value (%v) doesn't match the expected curent state (%s)", settingToggle.Checked, mode)
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
