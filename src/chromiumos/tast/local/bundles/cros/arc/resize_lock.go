// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/wm"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

type resizeLockTestFunc func(context.Context, *chrome.TestConn, *arc.ARC, *ui.Device, *chrome.Chrome, *input.KeyboardEventWriter) error

type resizeLockTestCase struct {
	name string
	fn   resizeLockTestFunc
}

// The order of the test cases matters since some persistent chrome-side properties are tested.
// - "Splash" must come first as any launch of the apps affect the test case.
// - CUJ must come next as it tests the behavior of resize confirmation dialog, and once tested the dialog isn't shown in other tests.
var testCases = []resizeLockTestCase{
	{
		name: "Splash",
		fn:   testSplash,
	},
	{
		name: "Resize Locked App - CUJ",
		fn:   testResizeLockedAppCUJ,
	},
	{
		name: "Resize Locked App - Fully Locked",
		fn:   testFullyLockedApp,
	},
	{
		name: "Resize Locked App - Toggle immersive mode",
		fn:   testToggleImmersiveMode,
	},
	{
		name: "Resize Locked App - Toggle PIP",
		fn:   testPIP,
	},
	{
		name: "O4C App",
		fn:   testO4CApp,
	},
	{
		name: "Unresizable Maximized App",
		fn:   testUnresizableMaximizedApp,
	},
	{
		name: "Resizable Maximized App",
		fn:   testResizableMaximizedApp,
	},
	{
		name: "Install from outside of PlayStore",
		fn:   testAppFromOutsideOfPlayStore,
	},
	{
		name: "Tablet mode",
		fn:   testTablet,
	},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ResizeLock,
		LacrosStatus: testing.LacrosVariantUnneeded,
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
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		s.Fatal("Failed to set device to clamshell mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeStatus)

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer keyboard.Close()

	ctxDefer := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	for _, app := range []struct {
		apkName       string
		pkgName       string
		fromPlayStore bool
	}{
		{wm.APKNameArcWMTestApp24, wm.Pkg24, true},
		{wm.APKNameArcWMTestApp24Maximized, wm.Pkg24InMaximizedList, true},
		{wm.ResizeLockApkName, wm.ResizeLockTestPkgName, true},
		{wm.ResizeLock2ApkName, wm.ResizeLock2PkgName, true},
		{wm.ResizeLock3ApkName, wm.ResizeLock3PkgName, true},
	} {
		if app.fromPlayStore {
			if err := a.Install(ctx, arc.APKPath(app.apkName), adb.InstallOptionFromPlayStore); err != nil {
				s.Fatal("Failed to install app from PlayStore: ", err)
			}
		} else {
			if err := a.Install(ctx, arc.APKPath(app.apkName)); err != nil {
				s.Fatal("Failed to install app from outside of PlayStore: ", err)
			}
		}
		defer a.Uninstall(ctxDefer, app.pkgName)
	}

	// Set a pure white wallpaper to reduce the noises on a screenshot because currently checking the visibility of the translucent window border relies on a screenshot.
	// The Wallpaper will exist continuous if the Chrome session gets reused.
	ui := uiauto.New(tconn)
	if err := wm.SetSolidWhiteWallpaper(ctx, ui); err != nil {
		s.Fatal("Failed to set the white wallpaper: ", err)
	}

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

// testToggleImmersiveMode verifies that a resize locked app rejects a fullscreen event.
func testToggleImmersiveMode(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	return testChangeWindowState(ctx, tconn, a, d, cr, keyboard, ash.WMEventFullscreen, ash.WindowStateNormal)
}

// testChangeWindowState verifies that the given WM event transitions a resize-locked app to the expected state.
func testChangeWindowState(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter, event ash.WMEventType, expectedState ash.WindowStateType) error {
	const (
		packageName  = wm.ResizeLockTestPkgName
		activityName = wm.ResizeLockMainActivityName
	)
	activity, err := arc.NewActivity(a, packageName, activityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", activityName)
	}
	defer activity.Close()
	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start %s", activityName)
	}
	defer activity.Stop(ctx, tconn)

	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.PhoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify the resize lock state of %s", activityName)
	}
	window, err := ash.GetARCAppWindowInfo(ctx, tconn, packageName)
	if err != nil {
		return errors.Wrapf(err, "failed to get ARC window infomation for package name %s", packageName)
	}

	if _, err := ash.SetWindowState(ctx, tconn, window.ID, event, false /* waitForStateChange */); err != nil {
		return errors.Wrapf(err, "failed to send window event %v to %s", event, activityName)
	}
	defer ash.SetARCAppWindowStateAndWait(ctx, tconn, packageName, ash.WindowStateNormal)

	if expectedState == ash.WindowStateNormal {
		if err := testing.Sleep(ctx, 3*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep waiting for window state change event to be completed")
		}
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, expectedState); err != nil {
		return errors.Wrapf(err, "failed to wait for %s to be expected state %v", activityName, expectedState)
	}
	return nil
}

// testPIP verifies that a resize locked app can enter PIP and becomes resizable in PIP mode.
func testPIP(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	const (
		packageName  = wm.ResizeLockTestPkgName
		activityName = wm.ResizeLockPipActivityName
	)
	activity, err := arc.NewActivity(a, packageName, activityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", activityName)
	}
	defer activity.Close()
	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start %s", activityName)
	}
	defer activity.Stop(ctx, tconn)

	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.PhoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify the resize lock state of %s", activityName)
	}
	window, err := ash.GetARCAppWindowInfo(ctx, tconn, packageName)
	if err != nil {
		return errors.Wrapf(err, "failed to get ARC window infomation for package name %s", packageName)
	}

	if _, err := ash.SetWindowState(ctx, tconn, window.ID, ash.WMEventMinimize, false /* waitForStateChange */); err != nil {
		return errors.Wrapf(err, "failed to minimize %s", activityName)
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, ash.WindowStatePIP); err != nil {
		return errors.Wrapf(err, "failed to wait for %s to enter PIP", activityName)
	}
	// Verify that the app is resizable in PIP mode.
	if err := wm.CheckResizability(ctx, tconn, a, d, packageName, true); err != nil {
		return errors.Wrapf(err, "failed to verify the resizability of %s", activityName)
	}
	return nil
}

// testO4CApp verifies that an O4C app is not resize locked even if it's newly-installed.
func testO4CApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	return testNonResizeLocked(ctx, tconn, a, d, cr, keyboard, wm.Pkg24, wm.APKNameArcWMTestApp24, wm.ResizableUnspecifiedActivity, false /* checkRestoreMaximize */)
}

// testUnresizableMaximizedApp verifies that an unresizable, maximized app is not resize locked even if it's newly-installed.
func testUnresizableMaximizedApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	return testNonResizeLocked(ctx, tconn, a, d, cr, keyboard, wm.ResizeLockTestPkgName, wm.ResizeLockApkName, wm.ResizeLockUnresizableUnspecifiedActivityName, false /* checkRestoreMaximize */)
}

// testResizableMaximizedApp verifies that a resizable, maximized app is not resize locked even if it's newly-installed.
func testResizableMaximizedApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	return testNonResizeLocked(ctx, tconn, a, d, cr, keyboard, wm.ResizeLockTestPkgName, wm.ResizeLockApkName, wm.ResizeLockResizableUnspecifiedMaximizedActivityName, true /* checkRestoreMaximize */)
}

// testAppFromOutsideOfPlayStore verifies that an resize-lock-eligible app installed from outside of PlayStore is not resize locked even if it's newly-installed.
func testAppFromOutsideOfPlayStore(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	return testNonResizeLocked(ctx, tconn, a, d, cr, keyboard, wm.Pkg24InMaximizedList, wm.APKNameArcWMTestApp24Maximized, wm.ResizableUnspecifiedActivity, false /* checkRestoreMaximize */)
}

// testTablet verifies that tablet conversion properly updates the resize lock state of an app.
func testTablet(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	const (
		packageName  = wm.ResizeLockTestPkgName
		activityName = wm.ResizeLockMainActivityName
	)

	tabletModeStatus, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get tablet mode status")
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeStatus)

	if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to change device to tablet mode")
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

	// Verify that resize lock isn't enabled in tablet mode.
	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.NoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify the resize lock state of %s", activityName)
	}

	// Convert the device to clamshell and verify that resize lock is enabled.
	if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to change device to clamshell mode")
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, ash.WindowStateNormal); err != nil {
		return errors.Wrapf(err, "failed to wait for %s to be restored", activityName)
	}
	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.PhoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify the resize lock state of %s", activityName)
	}

	// Convert the device back to clamshell and verify that resize lock is disabled again.
	if err := ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to change device to clamshell mode")
	}
	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.NoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify the resize lock state of %s", activityName)
	}
	return nil
}

// testNonResizeLocked verifies that the given app is not resize locked.
func testNonResizeLocked(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter, packageName, apkName, activityName string, checkRestoreMaximize bool) error {
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
	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.NoneResizeLockMode, false /* isSplashVisible */); err != nil {
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
		if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.TabletResizeLockMode, false /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", activityName)
		}
		// Make the app resizable to enable maximization.
		if err := wm.ToggleResizeLockMode(ctx, tconn, a, d, cr, activity, wm.TabletResizeLockMode, wm.ResizableTogglableResizeLockMode, wm.DialogActionNoDialog, wm.InputMethodClick, keyboard); err != nil {
			return errors.Wrapf(err, "failed to change the resize lock mode of %s from tablet to resizable", apkName)
		}
		defer func() error {
			if err := wm.ToggleResizeLockMode(ctx, tconn, a, d, cr, activity, wm.ResizableTogglableResizeLockMode, wm.PhoneResizeLockMode, wm.DialogActionNoDialog, wm.InputMethodClick, keyboard); err != nil {
				return errors.Wrapf(err, "failed to change the resize lock mode of %s from resizable to phone", apkName)
			}
			return nil
		}()
		// Maximize the app and verify the app is not resize-locked.
		if _, err := ash.SetARCAppWindowState(ctx, tconn, packageName, ash.WMEventMaximize); err != nil {
			return errors.Wrapf(err, "failed to maximize %s", activityName)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, ash.WindowStateMaximized); err != nil {
			return errors.Wrapf(err, "failed to wait for %s to be maximized", activityName)
		}
		if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.NoneResizeLockMode, false /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", activityName)
		}
		// Restore the app again and verify the app is in resizable state with compat-mode button shown.
		// We need this because (launch in maximize->restore) and (restored->maximized->restored) go through different code paths internally.
		if _, err := ash.SetARCAppWindowState(ctx, tconn, packageName, ash.WMEventNormal); err != nil {
			return errors.Wrapf(err, "failed to restore %s", activityName)
		}
		if err := ash.WaitForARCAppWindowState(ctx, tconn, packageName, ash.WindowStateNormal); err != nil {
			return errors.Wrapf(err, "failed to wait for %s to be restored", activityName)
		}
		if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.ResizableTogglableResizeLockMode, false /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", activityName)
		}
	}
	return nil
}

// testFullyLockedApp verifies that the given app is fully locked.
func testFullyLockedApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, keyboard *input.KeyboardEventWriter) error {
	activity, err := arc.NewActivity(a, wm.ResizeLockTestPkgName, wm.ResizeLockUnresizablePortraitActivityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", wm.ResizeLockUnresizablePortraitActivityName)
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start %s", wm.ResizeLockUnresizablePortraitActivityName)
	}
	defer activity.Stop(ctx, tconn)

	// Verify the initial state of the given non-resize-locked app.
	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.PhoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify the resize lock state of %s", wm.ResizeLockUnresizablePortraitActivityName)
	}

	// The compat-mode button of a fully-locked app is disabled.
	ui := uiauto.New(tconn)
	icon := nodewith.ClassName(wm.CenterButtonClassName)
	if err := ui.WithTimeout(10 * time.Second).LeftClick(icon)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on the compat-mode button")
	}

	// Need some sleep here as we verify that nothing changes.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep after clicking on the compat-mode button")
	}

	if err := wm.CheckVisibility(ctx, tconn, wm.BubbleDialogClassName, false); err != nil {
		return errors.Wrapf(err, "failed to verify the visibility of the compat-mode menu of %s", activity.ActivityName())
	}

	// The setting toggle of a fully-locked app is invisible in the app-management page.
	if err := wm.OpenAppManagementSetting(ctx, tconn, wm.ResizeLockAppName); err != nil {
		return errors.Wrapf(err, "failed to open the app management page of %s", wm.ResizeLockAppName)
	}
	defer wm.CloseAppManagementSetting(ctx, tconn)
	return ui.WithTimeout(10 * time.Second).WaitUntilGone(nodewith.Name(wm.AppManagementSettingToggleName))(ctx)
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
		method       wm.InputMethodType
	}{
		{wm.ResizeLockApkName, wm.ResizeLockTestPkgName, wm.ResizeLockMainActivityName, wm.InputMethodClick},
		{wm.ResizeLock2ApkName, wm.ResizeLock2PkgName, wm.ResizeLockMainActivityName, wm.InputMethodKeyEvent},
		{wm.ResizeLock3ApkName, wm.ResizeLock3PkgName, wm.ResizeLockMainActivityName, wm.InputMethodClick},
	} {
		activity, err := arc.NewActivity(a, test.pkgName, test.activityName)
		if err != nil {
			return errors.Wrapf(err, "failed to create %s", test.activityName)
		}
		defer activity.Close()

		if err := activity.Start(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to start %s", test.activityName)
		}
		defer activity.Stop(ctx, tconn)

		if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.PhoneResizeLockMode, i < showSplashLimit /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", wm.ResizeLockMainActivityName)
		}

		if i < showSplashLimit {
			if err := wm.CloseSplash(ctx, tconn, test.method, keyboard); err != nil {
				return errors.Wrapf(err, "failed to close the splash screen of %s via %s", wm.ResizeLockMainActivityName, test.method)
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

		if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.PhoneResizeLockMode, false /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", wm.ResizeLockMainActivityName)
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
		method       wm.InputMethodType
	}{
		{wm.ResizeLockTestPkgName, wm.ResizeLockApkName, wm.ResizeLockMainActivityName, wm.InputMethodClick},
		{wm.ResizeLock2PkgName, wm.ResizeLock2ApkName, wm.ResizeLockMainActivityName, wm.InputMethodKeyEvent},
	} {
		if err := testResizeLockedAppCUJInternal(ctx, tconn, a, d, cr, test.packageName, test.apkName, test.activityName, test.method, keyboard); err != nil {
			return errors.Wrapf(err, "failed to run the critical user journey for %s via %s", test.apkName, test.method)
		}
	}
	return nil
}

// testResizeLockedAppCUJInternal goes though the critical user journey of the given resize-locked app via the given input method.
func testResizeLockedAppCUJInternal(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, packageName, apkName, activityName string, method wm.InputMethodType, keyboard *input.KeyboardEventWriter) error {
	activity, err := arc.NewActivity(a, packageName, wm.ResizeLockMainActivityName)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", wm.ResizeLockMainActivityName)
	}
	defer activity.Close()

	if err := activity.Start(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to start %s", wm.ResizeLockMainActivityName)
	}
	defer activity.Stop(ctx, tconn)

	// Verify the initial state of a normal resize-locked app.
	if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, wm.PhoneResizeLockMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify resize lock state of %s", wm.ResizeLockMainActivityName)
	}

	// Verify can show and close the compat-mode menu by toggling the compat-mode button.
	if err := wm.ToggleCompatModeMenu(ctx, tconn, method, keyboard, true /* isMenuVisible */); err != nil {
		return errors.Wrapf(err, "failed to show the compat-mode dialog of %s via %s", activity.ActivityName(), method)
	}
	if err := wm.ToggleCompatModeMenu(ctx, tconn, method, keyboard, false /* isMenuVisible */); err != nil {
		return errors.Wrapf(err, "failed to close the compat-mode dialog of %s via %s", activity.ActivityName(), method)
	}

	for _, test := range []struct {
		currentMode wm.ResizeLockMode
		nextMode    wm.ResizeLockMode
		action      wm.ConfirmationDialogAction
	}{
		// Check the cancel button does nothing.
		{wm.PhoneResizeLockMode, wm.ResizableTogglableResizeLockMode, wm.DialogActionCancel},
		// Toggle between Phone and Tablet.
		{wm.PhoneResizeLockMode, wm.TabletResizeLockMode, wm.DialogActionNoDialog},
		{wm.TabletResizeLockMode, wm.PhoneResizeLockMode, wm.DialogActionNoDialog},
		// Toggle between Phone and Resizable without "Don't ask me again" checked.
		{wm.PhoneResizeLockMode, wm.ResizableTogglableResizeLockMode, wm.DialogActionConfirm},
		{wm.ResizableTogglableResizeLockMode, wm.PhoneResizeLockMode, wm.DialogActionNoDialog},
		// Toggle between Phone and Resizable with "Don't ask me again" checked.
		{wm.PhoneResizeLockMode, wm.ResizableTogglableResizeLockMode, wm.DialogActionConfirmWithDoNotAskMeAgainChecked},
		{wm.ResizableTogglableResizeLockMode, wm.PhoneResizeLockMode, wm.DialogActionNoDialog},
		{wm.PhoneResizeLockMode, wm.ResizableTogglableResizeLockMode, wm.DialogActionNoDialog},
	} {
		if err := wm.ToggleResizeLockMode(ctx, tconn, a, d, cr, activity, test.currentMode, test.nextMode, test.action, method, keyboard); err != nil {
			return errors.Wrapf(err, "failed to change the resize lock mode of %s from %s to %s", wm.ResizeLockApkName, test.currentMode, test.nextMode)
		}

		// Verify that relaunching an app doesn't cause any inconsistency.
		if err := activity.Stop(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to stop %s", wm.ResizeLockMainActivityName)
		}
		if err := activity.Start(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to start %s", wm.ResizeLockMainActivityName)
		}
		expectedMode := test.nextMode
		if test.action == wm.DialogActionCancel {
			expectedMode = test.currentMode
		}
		if err := wm.CheckResizeLockState(ctx, tconn, a, d, cr, activity, expectedMode, false /* isSplashVisible */); err != nil {
			return errors.Wrapf(err, "failed to verify resize lock state of %s", wm.ResizeLockMainActivityName)
		}
	}

	for _, test := range []struct {
		currentMode wm.ResizeLockMode
		nextMode    wm.ResizeLockMode
	}{
		{wm.ResizableTogglableResizeLockMode, wm.PhoneResizeLockMode},
		{wm.PhoneResizeLockMode, wm.ResizableTogglableResizeLockMode},
		{wm.ResizableTogglableResizeLockMode, wm.PhoneResizeLockMode},
	} {
		// Toggle the resizability state via the Chrome OS setting toggle.
		if err := wm.ToggleAppManagementSettingToggle(ctx, tconn, a, d, cr, activity, wm.ResizeLockAppName, test.currentMode, test.nextMode, method, keyboard); err != nil {
			return errors.Wrapf(err, "failed to toggle the resizability state from %s to %s on the Chrome OS settings", test.currentMode, test.nextMode)
		}
	}

	return nil
}
