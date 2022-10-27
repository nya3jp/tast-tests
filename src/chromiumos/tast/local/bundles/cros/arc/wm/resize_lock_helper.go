// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wm provides Window Manager Helper functions.
package wm

import (
	"context"
	"image/color"
	"math"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/wallpaper"
	"chromiumos/tast/local/wallpaper/constants"
	"chromiumos/tast/testing"
)

const (
	// ResizeLockTestPkgName is the package name of the resize lock app.
	ResizeLockTestPkgName = "org.chromium.arc.testapp.resizelock"
	// ResizeLockApkName is the apk name of the resize lock app.
	ResizeLockApkName = "ArcResizeLockTest.apk"
	// ResizeLockMainActivityName is the main activity name of the resize lock app.
	ResizeLockMainActivityName = "org.chromium.arc.testapp.resizelock.MainActivity"
	// ResizeLockUnresizableUnspecifiedActivityName is the name of the unresizable, unspecified activity of the resize lock app.
	ResizeLockUnresizableUnspecifiedActivityName = "org.chromium.arc.testapp.resizelock.UnresizableUnspecifiedActivity"
	// ResizeLockUnresizablePortraitActivityName is the name of the unresizable, portrait-only activity of the resize lock app.
	ResizeLockUnresizablePortraitActivityName = "org.chromium.arc.testapp.resizelock.UnresizablePortraitActivity"
	// ResizeLockResizableUnspecifiedMaximizedActivityName is the name of the resizable, maximized activity of the resize lock app.
	ResizeLockResizableUnspecifiedMaximizedActivityName = "org.chromium.arc.testapp.resizelock.ResizableUnspecifiedMaximizedActivity"
	// ResizeLockPipActivityName is the name of the PIP-able activity of the resize lock app.
	ResizeLockPipActivityName = "org.chromium.arc.testapp.resizelock.PipActivity"

	// ResizeLock2PkgName is the package name of the second resize lock app.
	ResizeLock2PkgName = "org.chromium.arc.testapp.resizelock2"
	// ResizeLock3PkgName is the package name of the third resize lock app.
	ResizeLock3PkgName = "org.chromium.arc.testapp.resizelock3"
	// ResizeLock2ApkName is the apk name of the second resize lock app.
	ResizeLock2ApkName = "ArcResizeLockTest2.apk"
	// ResizeLock3ApkName is the apk name of the third resize lock app.
	ResizeLock3ApkName = "ArcResizeLockTest3.apk"

	// ResizeLockO4CViaA2C2PkgName is the package name of the resize lock app declared as O4C via A2C2.
	ResizeLockO4CViaA2C2PkgName = "org.chromium.arc.testapp.resizelocko4cviaa2c2"
	// ResizeLockO4CViaA2C2ApkName is the apk name of the resize lock app declared as O4C via A2C2.
	ResizeLockO4CViaA2C2ApkName = "ArcResizeLockTestO4CViaA2C2.apk"
	// ResizeLockNonO4CViaA2C2PkgName is the package name of the resize lock app declared as non-O4C (AMAC-e) via A2C2.
	ResizeLockNonO4CViaA2C2PkgName = "org.chromium.arc.testapp.resizelocknono4cviaa2c2"
	// ResizeLockNonO4CViaA2C2ApkName is the apk name of the resize lock app declared as non-O4C (AMAC-e) via A2C2.
	ResizeLockNonO4CViaA2C2ApkName = "ArcResizeLockTestNonO4CViaA2C2.apk"

	// Used to (i) find the resize lock mode buttons on the compat-mode menu and (ii) check the state of the compat-mode button
	phoneButtonName     = "Phone"
	tabletButtonName    = "Tablet"
	resizableButtonName = "Resizable"

	// CenterButtonClassName is the class name of the caption center button.
	CenterButtonClassName = "FrameCenterButton"
	// BubbleDialogClassName is the class name of the bubble dialog.
	BubbleDialogClassName  = "BubbleDialogDelegateView"
	checkBoxClassName      = "Checkbox"
	overlayDialogClassName = "OverlayDialog"
	shelfIconClassName     = "ash/ShelfAppButton"
	menuItemViewClassName  = "MenuItemView"

	// AppManagementSettingToggleName is the a11y name of the app-management setting toggle.
	AppManagementSettingToggleName = "Preset window sizes"
	splashCloseButtonName          = "Got it"
	confirmButtonName              = "Allow"
	cancelButtonName               = "Cancel"
	appInfoMenuItemViewName        = "App info"
	closeMenuItemViewName          = "Close"

	// ResizeLockAppName is the name of the resize lock app. Used to identify the shelf icon of interest.
	ResizeLockAppName = "ArcResizeLockTest"
	settingsAppName   = "Settings"

	// Used in test cases where screenshots are taken.
	pixelColorDiffMargin                    = 15
	clientContentColorPixelPercentThreshold = 95
	// When shadow exists, the percentage will be 70~80%, and otherwise, it will be 0%. Let's use the intermediate value.
	borderColorPixelPercentageThreshold = 35
	borderWidthPX                       = 6
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

// ResizeLockMode represents the high-level state of the app from the resize-lock feature's perspective.
type ResizeLockMode int

const (
	// PhoneResizeLockMode represents the state where an app is locked in a portrait size.
	PhoneResizeLockMode ResizeLockMode = iota
	// TabletResizeLockMode represents the state where an app is locked in a landscape size.
	TabletResizeLockMode
	// ResizableTogglableResizeLockMode represents the state where an app is not resize lock, and the resize lock state is togglable.
	ResizableTogglableResizeLockMode
	// NoneResizeLockMode represents the state where an app is not eligible for resize lock.
	NoneResizeLockMode
)

func (mode ResizeLockMode) String() string {
	switch mode {
	case PhoneResizeLockMode:
		return phoneButtonName
	case TabletResizeLockMode:
		return tabletButtonName
	case ResizableTogglableResizeLockMode:
		return resizableButtonName
	default:
		return ""
	}
}

// ConfirmationDialogAction represents the expected behavior and action to take for the resizability confirmation dialog.
type ConfirmationDialogAction int

const (
	// DialogActionNoDialog represents the behavior where resize confirmation dialog isn't shown when a window is resized.
	DialogActionNoDialog ConfirmationDialogAction = iota
	// DialogActionCancel represents the behavior where resize confirmation dialog is shown, and the cancel button should be selected.
	DialogActionCancel
	// DialogActionConfirm represents the behavior where resize confirmation dialog is shown, and the confirm button should be selected.
	DialogActionConfirm
	// DialogActionConfirmWithDoNotAskMeAgainChecked represents the behavior where resize confirmation dialog is shown, and the confirm button should be selected with the "Don't ask me again" option on.
	DialogActionConfirmWithDoNotAskMeAgainChecked
)

// InputMethodType represents how to interact with UI.
type InputMethodType int

const (
	// InputMethodClick represents the state where UI should be interacted with mouse click.
	InputMethodClick InputMethodType = iota
	// InputMethodKeyEvent represents the state where UI should be interacted with keyboard.
	InputMethodKeyEvent
)

func (mode InputMethodType) String() string {
	switch mode {
	case InputMethodClick:
		return "click"
	case InputMethodKeyEvent:
		return "keyboard"
	default:
		return "unknown"
	}
}

// CheckResizeLockState verifies the various properties that depend on resize lock state.
func CheckResizeLockState(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, mode ResizeLockMode, isSplashVisible bool) error {
	resizeLocked := mode == PhoneResizeLockMode || mode == TabletResizeLockMode

	if err := CheckResizability(ctx, tconn, a, d, activity.PackageName(), getExpectedResizability(activity, mode)); err != nil {
		return errors.Wrapf(err, "failed to verify the resizability of %s", activity.ActivityName())
	}

	if err := CheckVisibility(ctx, tconn, BubbleDialogClassName, isSplashVisible); err != nil {
		return errors.Wrapf(err, "failed to verify the visibility of the splash screen on %s", activity.ActivityName())
	}

	if err := CheckCompatModeButton(ctx, tconn, a, d, cr, activity, mode); err != nil {
		return errors.Wrapf(err, "failed to verify the type of the compat mode button of %s", activity.ActivityName())
	}

	if err := checkBorder(ctx, tconn, a, d, cr, activity, resizeLocked /* shouldShowBorder */); err != nil {
		return errors.Wrapf(err, "failed to verify the visibility of the resize lock window border of %s", activity.ActivityName())
	}

	// There's no orientation rule for non-resize-locked apps, so only check the phone and tablet modes.
	if mode == TabletResizeLockMode {
		if err := checkOrientation(ctx, tconn, a, d, cr, activity, tabletOrientation); err != nil {
			return errors.Wrapf(err, "failed to verify %s has tablet orientation", activity.ActivityName())
		}
	} else if mode == PhoneResizeLockMode {
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
func getExpectedResizability(activity *arc.Activity, mode ResizeLockMode) bool {
	// Resize-locked apps are unresizable.
	if mode == PhoneResizeLockMode || mode == TabletResizeLockMode {
		return false
	}

	// The activity with resizability false in its manifest is unresizable.
	if activity.ActivityName() == ResizeLockUnresizableUnspecifiedActivityName {
		return false
	}

	return true
}

// checkMaximizeRestoreButtonVisibility verifies the visibility of the maximize/restore button of the given app.
func checkMaximizeRestoreButtonVisibility(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, mode ResizeLockMode) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		expected := ash.CaptionButtonBack | ash.CaptionButtonMinimize | ash.CaptionButtonClose
		tabletModeStatus, err := ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get tablet mode status")
		}
		// The visibility of the maximize/restore button matches the resizability of the app.
		if getExpectedResizability(activity, mode) && !tabletModeStatus {
			expected |= ash.CaptionButtonMaximizeAndRestore
		}
		return CompareCaption(ctx, tconn, activity.PackageName(), expected)
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

// CheckCompatModeButton verifies the state of the compat-mode button of the given app.
func CheckCompatModeButton(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, mode ResizeLockMode) error {
	if mode == NoneResizeLockMode {
		return CheckVisibility(ctx, tconn, CenterButtonClassName, false /* visible */)
	}

	uia := uiauto.New(tconn)
	button := nodewith.HasClass(CenterButtonClassName)
	return testing.Poll(ctx, func(ctx context.Context) error {
		info, err := uia.Info(ctx, button)
		if err != nil {
			return errors.Wrap(err, "failed to find the compat-mode button")
		}

		if info.Name != mode.String() {
			return errors.Errorf("failed to verify the name of compat-mode button; got: %s, want: %s", info.Name, mode)
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
		shadowOnBorderPixels := 0
		borderPixels := 0
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				p := coords.Point{X: x, Y: y}
				onBorder := p.In(bounds.WithInset(-borderWidthPX, -borderWidthPX)) && !p.In(bounds)
				if onBorder {
					if colorcmp.ColorsMatch(img.At(x, y), color.RGBA{170, 170, 170, 255}, pixelColorDiffMargin) {
						shadowOnBorderPixels++
					}
					borderPixels++
				}
			}
		}

		// borderPixels is 0 if the window is maximized. There's nothing to verify in this case.
		if borderPixels != 0 {
			shadowOnBorderPercentage := int(float64(shadowOnBorderPixels) / float64(borderPixels) * 100.0)
			if shouldShowBorder && shadowOnBorderPercentage < borderColorPixelPercentageThreshold {
				return errors.Errorf("failed to verify that the window border is visible; Border has %d%% (%d/%d) of shadow pixels (threshold: %d%%)", shadowOnBorderPercentage, shadowOnBorderPixels, borderPixels, borderColorPixelPercentageThreshold)
			}
			if !shouldShowBorder && shadowOnBorderPercentage > borderColorPixelPercentageThreshold {
				return errors.Errorf("failed to verify that the window border is invisible; Border has %d%% (%d/%d) of shadow pixels (threshold: %d%%)", shadowOnBorderPercentage, shadowOnBorderPixels, borderPixels, borderColorPixelPercentageThreshold)
			}
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// CheckVisibility checks whether the node specified by the given class name exists or not.
func CheckVisibility(ctx context.Context, tconn *chrome.TestConn, className string, visible bool) error {
	uia := uiauto.New(tconn)
	finder := nodewith.HasClass(className).First()
	if visible {
		return uia.WithTimeout(10 * time.Second).WaitUntilExists(finder)(ctx)
	}
	return uia.WithTimeout(10 * time.Second).WaitUntilGone(finder)(ctx)
}

// CheckResizability verifies the given app's resizability.
func CheckResizability(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, pkgName string, expected bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		window, err := ash.GetARCAppWindowInfo(ctx, tconn, pkgName)
		if err != nil {
			return errors.Wrapf(err, "failed to get the ARC window infomation for package name %s", pkgName)
		}
		if window.CanResize != expected {
			return errors.Errorf("failed to verify the resizability; got %t, want %t", window.CanResize, expected)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// ToggleCompatModeMenu toggles the compat-mode menu via the given method and verifies the expected visibility of the compat-mode menu.
func ToggleCompatModeMenu(ctx context.Context, tconn *chrome.TestConn, method InputMethodType, keyboard *input.KeyboardEventWriter, isMenuVisible bool) error {
	switch method {
	case InputMethodClick:
		return toggleCompatModeMenuViaButtonClick(ctx, tconn, isMenuVisible)
	case InputMethodKeyEvent:
		return toggleCompatModeMenuViaKeyboard(ctx, tconn, keyboard, isMenuVisible)
	}
	return errors.Errorf("invalid InputMethodType is given: %s", method)
}

// toggleCompatModeMenuViaButtonClick clicks on the compat-mode button and verifies the expected visibility of the compat-mode menu.
func toggleCompatModeMenuViaButtonClick(ctx context.Context, tconn *chrome.TestConn, isMenuVisible bool) error {
	ui := uiauto.New(tconn)
	icon := nodewith.Role(role.Button).HasClass(CenterButtonClassName)
	if err := ui.WithTimeout(10 * time.Second).LeftClick(icon)(ctx); err != nil {
		return errors.Wrap(err, "failed to click on the compat-mode button")
	}

	return CheckVisibility(ctx, tconn, BubbleDialogClassName, isMenuVisible)
}

// toggleCompatModeMenuViaKeyboard injects the keyboard shortcut and verifies the expected visibility of the compat-mode menu.
func toggleCompatModeMenuViaKeyboard(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, isMenuVisible bool) error {
	ui := uiauto.New(tconn)
	accel := func(ctx context.Context) error {
		if err := keyboard.Accel(ctx, "Search+Alt+C"); err != nil {
			return errors.Wrap(err, "failed to inject Search+Alt+C")
		}
		return nil
	}
	dialog := nodewith.Role(role.Window).HasClass(BubbleDialogClassName)
	if isMenuVisible {
		return ui.WithTimeout(10*time.Second).WithInterval(2*time.Second).RetryUntil(accel, ui.Exists(dialog))(ctx)
	}
	return nil
}

// waitForCompatModeMenuToDisappear waits for the compat-mode menu to disappear.
// After one of the resize lock mode buttons are selected, the compat mode menu disappears after a few seconds of delay.
// Can't use chromeui.WaitUntilGone() for this purpose because this function also checks whether the dialog has the "Phone" button or not to ensure that we are checking the correct dialog.
func waitForCompatModeMenuToDisappear(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	dialog := nodewith.ClassName(BubbleDialogClassName).Role(role.Window)
	phoneButton := nodewith.HasClass(phoneButtonName).Ancestor(dialog)
	return ui.WithTimeout(10 * time.Second).WaitUntilGone(phoneButton)(ctx)
}

// CloseSplash closes the splash screen via the given method.
func CloseSplash(ctx context.Context, tconn *chrome.TestConn, method InputMethodType, keyboard *input.KeyboardEventWriter) error {
	ui := uiauto.New(tconn)
	splash := nodewith.ClassName(BubbleDialogClassName).Role(role.Window)
	if err := ui.Exists(splash)(ctx); err != nil {
		return errors.Wrap(err, "failed to find the splash dialog")
	}

	switch method {
	case InputMethodClick:
		return closeSplashViaClick(ctx, tconn, splash)
	case InputMethodKeyEvent:
		return closeSplashViaKeyboard(ctx, tconn, splash, keyboard)
	}
	return nil
}

// closeSplashViaKeyboard presses the Enter key and closes the splash screen.
func closeSplashViaKeyboard(ctx context.Context, tconn *chrome.TestConn, splash *nodewith.Finder, keyboard *input.KeyboardEventWriter) error {
	ui := uiauto.New(tconn)
	enter := func(ctx context.Context) error {
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			return errors.Wrap(err, "failed to press the Enter key")
		}
		return nil
	}
	if err := ui.WithTimeout(10*time.Second).RetryUntil(enter, ui.Gone(splash))(ctx); err != nil {
		return errors.Wrap(err, "failed to close splash via keyboard")
	}
	return nil
}

// closeSplashViaClick clicks on the close button and closes the splash screen.
func closeSplashViaClick(ctx context.Context, tconn *chrome.TestConn, splash *nodewith.Finder) error {
	ui := uiauto.New(tconn)
	button := nodewith.Ancestor(splash).Role(role.Button).Name(splashCloseButtonName)
	if err := ui.WithTimeout(10*time.Second).LeftClickUntil(button, ui.Gone(splash))(ctx); err != nil {
		return errors.Wrap(err, "failed to close splash via click")
	}
	return nil
}

// ToggleResizeLockMode shows the compat-mode menu, selects one of the resize lock mode buttons on the compat-mode menu via the given method, and verifies the post state.
func ToggleResizeLockMode(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, currentMode, nextMode ResizeLockMode, action ConfirmationDialogAction, method InputMethodType, keyboard *input.KeyboardEventWriter) error {
	preToggleOrientation, err := activityOrientation(ctx, tconn, activity)
	if err != nil {
		return errors.Wrapf(err, "failed to get the pre-toggle orientation of %s", activity.PackageName())
	}
	if err := ToggleCompatModeMenu(ctx, tconn, method, keyboard, true /* isMenuVisible */); err != nil {
		return errors.Wrapf(err, "failed to show the compat-mode dialog of %s via %s", activity.ActivityName(), method)
	}

	ui := uiauto.New(tconn)
	compatModeMenuDialog := nodewith.Role(role.Window).HasClass(BubbleDialogClassName)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(compatModeMenuDialog)(ctx); err != nil {
		return errors.Wrapf(err, "failed to find the compat-mode menu dialog of %s", activity.ActivityName())
	}

	switch method {
	case InputMethodClick:
		if err := selectResizeLockModeViaClick(ctx, tconn, nextMode, compatModeMenuDialog); err != nil {
			return errors.Wrapf(err, "failed to click on the compat-mode dialog of %s via click", activity.ActivityName())
		}
	case InputMethodKeyEvent:
		if err := shiftViaTabAndEnter(ctx, tconn, nodewith.Ancestor(compatModeMenuDialog).Role(role.MenuItem).Name(nextMode.String()), keyboard); err != nil {
			return errors.Wrapf(err, "failed to click on the compat-mode dialog of %s via keyboard", activity.ActivityName())
		}
	}

	expectedMode := nextMode
	if action == DialogActionCancel {
		expectedMode = currentMode
	}
	if action != DialogActionNoDialog {
		if err := waitForCompatModeMenuToDisappear(ctx, tconn); err != nil {
			return errors.Wrapf(err, "failed to wait for the compat-mode menu of %s to disappear", activity.ActivityName())
		}

		confirmationDialog := nodewith.HasClass(overlayDialogClassName)
		if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(confirmationDialog)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the resizability confirmation dialog")
		}

		switch method {
		case InputMethodClick:
			if err := handleConfirmationDialogViaClick(ctx, tconn, nextMode, confirmationDialog, action); err != nil {
				return errors.Wrapf(err, "failed to handle the confirmation dialog of %s via click", activity.ActivityName())
			}
		case InputMethodKeyEvent:
			if err := handleConfirmationDialogViaKeyboard(ctx, tconn, nextMode, confirmationDialog, action, keyboard); err != nil {
				return errors.Wrapf(err, "failed to handle the confirmation dialog of %s via keyboard", activity.ActivityName())
			}
		}
	}

	// The compat-mode dialog stays shown for two seconds by default after resize lock mode is toggled.
	// Explicitly close the dialog using the Esc key.
	if err := ui.WithTimeout(5*time.Second).RetryUntil(func(ctx context.Context) error {
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			return errors.Wrap(err, "failed to press the Esc key")
		}
		return nil
	}, ui.Gone(nodewith.Role(role.Window).Name(BubbleDialogClassName)))(ctx); err != nil {
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

	return CheckResizeLockState(ctx, tconn, a, d, cr, activity, expectedMode, false /* isSplashVisible */)
}

// handleConfirmationDialogViaKeyboard does the given action for the confirmation dialog via keyboard.
func handleConfirmationDialogViaKeyboard(ctx context.Context, tconn *chrome.TestConn, mode ResizeLockMode, confirmationDialog *nodewith.Finder, action ConfirmationDialogAction, keyboard *input.KeyboardEventWriter) error {
	if action == DialogActionCancel {
		return shiftViaTabAndEnter(ctx, tconn, nodewith.Ancestor(confirmationDialog).Role(role.Button).Name(cancelButtonName), keyboard)
	} else if action == DialogActionConfirm || action == DialogActionConfirmWithDoNotAskMeAgainChecked {
		if action == DialogActionConfirmWithDoNotAskMeAgainChecked {
			if err := shiftViaTabAndEnter(ctx, tconn, nodewith.Ancestor(confirmationDialog).HasClass(checkBoxClassName), keyboard); err != nil {
				return errors.Wrap(err, "failed to select the checkbox of the resizability confirmation dialog via keyboard")
			}
		}
		return shiftViaTabAndEnter(ctx, tconn, nodewith.Ancestor(confirmationDialog).Role(role.Button).Name(confirmButtonName), keyboard)
	}
	return nil
}

// handleConfirmationDialogViaClick does the given action for the confirmation dialog via click.
func handleConfirmationDialogViaClick(ctx context.Context, tconn *chrome.TestConn, mode ResizeLockMode, confirmationDialog *nodewith.Finder, action ConfirmationDialogAction) error {
	ui := uiauto.New(tconn)
	if action == DialogActionCancel {
		cancelButton := nodewith.Ancestor(confirmationDialog).Role(role.Button).Name(cancelButtonName)
		return ui.WithTimeout(10 * time.Second).LeftClick(cancelButton)(ctx)
	} else if action == DialogActionConfirm || action == DialogActionConfirmWithDoNotAskMeAgainChecked {
		if action == DialogActionConfirmWithDoNotAskMeAgainChecked {
			checkbox := nodewith.HasClass(checkBoxClassName)
			if err := ui.WithTimeout(10 * time.Second).LeftClick(checkbox)(ctx); err != nil {
				return errors.Wrap(err, "failed to click on the checkbox of the resizability confirmation dialog")
			}
		}

		confirmButton := nodewith.Ancestor(confirmationDialog).Role(role.Button).Name(confirmButtonName)
		return ui.WithTimeout(10 * time.Second).LeftClick(confirmButton)(ctx)
	}
	return nil
}

// selectResizeLockModeViaClick clicks on the given resize lock mode button.
func selectResizeLockModeViaClick(ctx context.Context, tconn *chrome.TestConn, mode ResizeLockMode, compatModeMenuDialog *nodewith.Finder) error {
	ui := uiauto.New(tconn)
	resizeLockModeButton := nodewith.Ancestor(compatModeMenuDialog).Role(role.MenuItem).Name(mode.String())
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(resizeLockModeButton)(ctx); err != nil {
		return errors.Wrapf(err, "failed to find the %s button on the compat mode menu", mode)
	}
	return ui.LeftClick(resizeLockModeButton)(ctx)
}

// shiftViaTabAndEnter keeps pressing the Tab key until the UI element of interest gets focus, and press the Enter key.
func shiftViaTabAndEnter(ctx context.Context, tconn *chrome.TestConn, target *nodewith.Finder, keyboard *input.KeyboardEventWriter) error {
	ui := uiauto.New(tconn)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := keyboard.Accel(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to press the Tab key")
		}
		if err := ui.Exists(target)(ctx); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to find the node seeking focus"))
		}
		return ui.Exists(target.Focused())(ctx)
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to shift focus to the node to click on")
	}
	return keyboard.Accel(ctx, "Enter")
}

// ToggleAppManagementSettingToggle opens the app-management page for the given app via the shelf icon, toggles the resize lock setting, and verifies the states of the app and the setting toggle.
func ToggleAppManagementSettingToggle(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, appName string, currentMode, nextMode ResizeLockMode, method InputMethodType, keyboard *input.KeyboardEventWriter) error {
	// This check must be done before opening the ChromeOS settings page so it won't affect the screenshot taken in one of the checks.
	if err := CheckResizeLockState(ctx, tconn, a, d, cr, activity, currentMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify resize lock state of %s", appName)
	}

	if err := OpenAppManagementSetting(ctx, tconn, appName); err != nil {
		return errors.Wrapf(err, "failed to open the app management page of %s", appName)
	}

	if err := checkAppManagementSettingToggleState(ctx, tconn, currentMode); err != nil {
		return errors.Wrap(err, "failed to verify the state of the setting toggle before toggling the setting")
	}

	switch method {
	case InputMethodClick:
		if err := toggleAppManagementSettingToggleViaClick(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to toggle the resize-lock setting toggle on the ChromeOS settings via click")
		}
	case InputMethodKeyEvent:
		if err := shiftViaTabAndEnter(ctx, tconn, nodewith.Name(AppManagementSettingToggleName).Role(role.ToggleButton), keyboard); err != nil {
			return errors.Wrap(err, "failed to toggle the resize-lock setting toggle on the ChromeOS settings via keyboard")
		}
	}

	if err := checkAppManagementSettingToggleState(ctx, tconn, nextMode); err != nil {
		return errors.Wrap(err, "failed to verify the state of the setting toggle after toggling the setting")
	}

	if err := CloseAppManagementSetting(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to close the app management page of %s", appName)
	}

	// This check must be done after closing the ChromeOS settings page so it won't affect the screenshot taken in one of the checks.
	if err := CheckResizeLockState(ctx, tconn, a, d, cr, activity, nextMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify resize lock state of %s", activity.ActivityName())
	}

	return nil
}

// toggleAppManagementSettingToggleViaClick toggles the resize-lock setting toggle via click.
func toggleAppManagementSettingToggleViaClick(ctx context.Context, tconn *chrome.TestConn) error {
	return uiauto.New(tconn).WithTimeout(10 * time.Second).LeftClick(nodewith.Name(AppManagementSettingToggleName))(ctx)
}

// OpenAppManagementSetting opens the app management page if the given app.
func OpenAppManagementSetting(ctx context.Context, tconn *chrome.TestConn, appName string) error {
	uia := uiauto.New(tconn)
	resizeLockShelfIcon := nodewith.Name(appName).HasClass(shelfIconClassName)
	if err := uia.WithTimeout(10 * time.Second).RightClick(resizeLockShelfIcon)(ctx); err != nil {
		return errors.Wrapf(err, "failed to click on the shelf icon of %s", appName)
	}

	appInfoMenuItem := nodewith.Name(appInfoMenuItemViewName).HasClass(menuItemViewClassName)
	if err := uia.WithTimeout(10 * time.Second).LeftClick(appInfoMenuItem)(ctx); err != nil {
		return errors.Wrap(err, "failed to find and click on the menu item for the app-management page")
	}
	return nil
}

// CloseAppManagementSetting closes any open app management page.
func CloseAppManagementSetting(ctx context.Context, tconn *chrome.TestConn) error {
	uia := uiauto.New(tconn)
	settingShelfIcon := nodewith.Name(settingsAppName).HasClass(shelfIconClassName)
	if err := uia.WithTimeout(10 * time.Second).RightClick(settingShelfIcon)(ctx); err != nil {
		return errors.Wrap(err, "failed to find and right click on the shelf icon of the settings app")
	}

	closeMenuItem := nodewith.Name(closeMenuItemViewName).HasClass(menuItemViewClassName)
	if err := uia.WithTimeout(10 * time.Second).LeftClick(closeMenuItem)(ctx); err != nil {
		return errors.Wrap(err, "failed to find and click on the menu item for closing the settings app")
	}
	return nil
}

// checkAppManagementSettingToggleState verifies the resize lock setting state on the app-management page.
// The app management page must be open when this function is called.
func checkAppManagementSettingToggleState(ctx context.Context, tconn *chrome.TestConn, mode ResizeLockMode) error {
	uia := uiauto.New(tconn)
	return testing.Poll(ctx, func(ctx context.Context) error {
		settingToggle, err := uia.WithTimeout(2*time.Second).Info(ctx, nodewith.Name(AppManagementSettingToggleName))
		if err != nil {
			return errors.Wrap(err, "failed to find the resize lock setting toggle on the app-management page")
		}

		if ((mode == PhoneResizeLockMode || mode == TabletResizeLockMode) && settingToggle.Checked == checked.False) ||
			(mode == ResizableTogglableResizeLockMode && settingToggle.Checked == checked.True) {
			return errors.Errorf("the app-management resize lock setting value (%v) doesn't match the expected curent state (%s)", settingToggle.Checked, mode)
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

func openLegacyWallpaperPicker(ui *uiauto.Context) uiauto.Action {
	setWallpaperMenu := nodewith.Name("Set wallpaper").Role(role.MenuItem)
	return ui.RetryUntil(uiauto.Combine("open wallpaper picker",
		ui.LeftClick(nodewith.HasClass("WallpaperView")),
		ui.RightClick(nodewith.HasClass("WallpaperView")),
		ui.WithInterval(300*time.Millisecond).LeftClickUntil(setWallpaperMenu, ui.Gone(setWallpaperMenu))),
		ui.Exists(nodewith.NameContaining("Wallpaper").Role(role.Window).First()))
}

func scrollDownUntilSucceeds(ctx context.Context, action uiauto.Action, mew *input.MouseEventWriter) error {
	const (
		maxNumSelectRetries = 4
		numScrolls          = 100
	)
	var actionErr error
	for i := 0; i < maxNumSelectRetries; i++ {
		if actionErr = action(ctx); actionErr == nil {
			return nil
		}
		for j := 0; j < numScrolls; j++ {
			if err := mew.ScrollDown(); err != nil {
				return errors.Wrap(err, "failed to scroll down")
			}
		}
	}

	return actionErr
}

// SetSolidWhiteWallpaper sets the wallpaper to the solid white.
func SetSolidWhiteWallpaper(ctx context.Context, ui *uiauto.Context) error {
	mew, err := input.Mouse(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup the mouse")
	}
	defer mew.Close()

	if err := wallpaper.OpenWallpaperPicker(ui)(ctx); err != nil {
		return errors.Wrap(err, "failed to open wallpaper picker")
	}

	// Move the cursor to the active photo container before scrolling.
	if err := ui.MouseMoveTo(nodewith.Role(role.ListBoxOption).HasClass("photo-inner-container").First(), time.Second)(ctx); err != nil {
		return errors.Wrap(err, "failed to move mouse to the photo container")
	}

	// "Solid" collection is at the end of the collection list so we need to scroll down to make it visible on a small display.
	if err := scrollDownUntilSucceeds(ctx, wallpaper.SelectCollection(ui, constants.SolidColorsCollection), mew); err != nil {
		return errors.Wrap(err, "failed to select wallpaper collection")
	}

	// "White" wallpaper is at the end of the wallpaper list so we need to scroll down to make it visible on a small display.
	if err := scrollDownUntilSucceeds(ctx, wallpaper.SelectImage(ui.WithTimeout(5*time.Second), constants.WhiteWallpaperName), mew); err != nil {
		return errors.Wrap(err, "failed to select wallpaper image")
	}

	if err := wallpaper.CloseWallpaperPicker()(ctx); err != nil {
		return errors.Wrap(err, "failed to close wallpaper picker")
	}

	return nil
}
