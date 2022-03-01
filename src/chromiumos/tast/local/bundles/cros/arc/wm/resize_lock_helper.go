// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/imgcmp"
	"chromiumos/tast/local/screenshot"
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
	borderColorPixelCountThreshold          = 3000
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

	return testing.Poll(ctx, func(ctx context.Context) error {
		button, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: CenterButtonClassName})
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
				onLeftBorder := bounds.Left-borderWidthPX <= x && x < bounds.Left && bounds.Top <= y && y < bounds.Bottom()
				onTopBorder := bounds.Top-borderWidthPX <= y && y < bounds.Top && bounds.Left <= x && x < bounds.Right()
				onRightBorder := bounds.Right() < x && x <= bounds.Right()+borderWidthPX && bounds.Top <= y && y < bounds.Bottom()
				onBottomBorder := bounds.Bottom() < y && y <= bounds.Bottom()+borderWidthPX && bounds.Left <= x && x < bounds.Right()
				onBorder := onLeftBorder || onTopBorder || onRightBorder || onBottomBorder
				if onBorder && colorcmp.ColorsMatch(img.At(x, y), color.RGBA{170, 170, 170, 255}, pixelColorDiffMargin) {
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

// CheckVisibility checks whether the node specified by the given class name exists or not.
func CheckVisibility(ctx context.Context, tconn *chrome.TestConn, className string, visible bool) error {
	if visible {
		return chromeui.WaitUntilExists(ctx, tconn, chromeui.FindParams{ClassName: className}, 10*time.Second)
	}
	return chromeui.WaitUntilGone(ctx, tconn, chromeui.FindParams{ClassName: className}, 10*time.Second)
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

// showCompatModeMenu shows the compat-mode menu via the given method.
func showCompatModeMenu(ctx context.Context, tconn *chrome.TestConn, method InputMethodType, keyboard *input.KeyboardEventWriter) error {
	switch method {
	case InputMethodClick:
		return showCompatModeMenuViaButtonClick(ctx, tconn)
	case InputMethodKeyEvent:
		return showCompatModeMenuViaKeyboard(ctx, tconn, keyboard)
	}
	return errors.Errorf("invalid InputMethodType is given: %s", method)
}

// showCompatModeMenuViaButtonClick clicks on the compat-mode button and shows the compat-mode menu.
func showCompatModeMenuViaButtonClick(ctx context.Context, tconn *chrome.TestConn) error {
	icon, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: CenterButtonClassName}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the compat-mode button")
	}
	defer icon.Release(ctx)

	if err := icon.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click on the compat-mode button")
	}

	return CheckVisibility(ctx, tconn, BubbleDialogClassName, true /* visible */)
}

// showCompatModeMenuViaKeyboard injects the keyboard shortcut and shows the compat-mode menu.
func showCompatModeMenuViaKeyboard(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := keyboard.Accel(ctx, "Search+Alt+C"); err != nil {
			return errors.Wrap(err, "failed to inject Search+Alt+C")
		}

		if err := chromeui.WaitUntilExists(ctx, tconn, chromeui.FindParams{ClassName: BubbleDialogClassName}, 2*time.Second); err != nil {
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
		dialog, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: BubbleDialogClassName})
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

// CloseSplash closes the splash screen via the given method.
func CloseSplash(ctx context.Context, tconn *chrome.TestConn, method InputMethodType, keyboard *input.KeyboardEventWriter) error {
	splash, err := chromeui.Find(ctx, tconn, chromeui.FindParams{ClassName: BubbleDialogClassName})
	if err != nil {
		return errors.Wrap(err, "failed to find the splash dialog")
	}
	defer splash.Release(ctx)

	switch method {
	case InputMethodClick:
		return closeSplashViaClick(ctx, tconn, splash)
	case InputMethodKeyEvent:
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
		return CheckVisibility(ctx, tconn, BubbleDialogClassName, false /* visible */)
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

		return CheckVisibility(ctx, tconn, BubbleDialogClassName, false /* visible */)
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// ToggleResizeLockMode shows the compat-mode menu, selects one of the resize lock mode buttons on the compat-mode menu via the given method, and verifies the post state.
func ToggleResizeLockMode(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, currentMode, nextMode ResizeLockMode, action ConfirmationDialogAction, method InputMethodType, keyboard *input.KeyboardEventWriter) error {
	preToggleOrientation, err := activityOrientation(ctx, tconn, activity)
	if err != nil {
		return errors.Wrapf(err, "failed to get the pre-toggle orientation of %s", activity.PackageName())
	}
	if err := showCompatModeMenu(ctx, tconn, method, keyboard); err != nil {
		return errors.Wrapf(err, "failed to show the compat-mode dialog of %s via %s", activity.ActivityName(), method)
	}

	compatModeMenuDialog, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: BubbleDialogClassName}, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find the compat-mode menu dialog of %s", activity.ActivityName())
	}
	defer compatModeMenuDialog.Release(ctx)

	switch method {
	case InputMethodClick:
		if err := selectResizeLockModeViaClick(ctx, nextMode, compatModeMenuDialog); err != nil {
			return errors.Wrapf(err, "failed to click on the compat-mode dialog of %s via click", activity.ActivityName())
		}
	case InputMethodKeyEvent:
		if err := shiftViaTabAndEnter(ctx, tconn, compatModeMenuDialog, chromeui.FindParams{Name: nextMode.String()}, keyboard); err != nil {
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

		confirmationDialog, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{ClassName: overlayDialogClassName}, 10*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find the resizability confirmation dialog")
		}
		defer confirmationDialog.Release(ctx)

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
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			return errors.Wrap(err, "failed to press the Esc key")
		}

		return CheckVisibility(ctx, tconn, BubbleDialogClassName, false /* visible */)
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

	return CheckResizeLockState(ctx, tconn, a, d, cr, activity, expectedMode, false /* isSplashVisible */)
}

// handleConfirmationDialogViaKeyboard does the given action for the confirmation dialog via keyboard.
func handleConfirmationDialogViaKeyboard(ctx context.Context, tconn *chrome.TestConn, mode ResizeLockMode, confirmationDialog *chromeui.Node, action ConfirmationDialogAction, keyboard *input.KeyboardEventWriter) error {
	if action == DialogActionCancel {
		return shiftViaTabAndEnter(ctx, tconn, confirmationDialog, chromeui.FindParams{Name: cancelButtonName}, keyboard)
	} else if action == DialogActionConfirm || action == DialogActionConfirmWithDoNotAskMeAgainChecked {
		if action == DialogActionConfirmWithDoNotAskMeAgainChecked {
			if err := shiftViaTabAndEnter(ctx, tconn, confirmationDialog, chromeui.FindParams{ClassName: checkBoxClassName}, keyboard); err != nil {
				return errors.Wrap(err, "failed to select the checkbox of the resizability confirmation dialog via keyboard")
			}
		}
		return shiftViaTabAndEnter(ctx, tconn, confirmationDialog, chromeui.FindParams{Name: confirmButtonName}, keyboard)
	}
	return nil
}

// handleConfirmationDialogViaClick does the given action for the confirmation dialog via click.
func handleConfirmationDialogViaClick(ctx context.Context, tconn *chrome.TestConn, mode ResizeLockMode, confirmationDialog *chromeui.Node, action ConfirmationDialogAction) error {
	if action == DialogActionCancel {
		cancelButton, err := confirmationDialog.DescendantWithTimeout(ctx, chromeui.FindParams{Name: cancelButtonName}, 10*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find the cancel button on the compat mode menu")
		}
		return cancelButton.LeftClick(ctx)
	} else if action == DialogActionConfirm || action == DialogActionConfirmWithDoNotAskMeAgainChecked {
		if action == DialogActionConfirmWithDoNotAskMeAgainChecked {
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
func selectResizeLockModeViaClick(ctx context.Context, mode ResizeLockMode, compatModeMenuDialog *chromeui.Node) error {
	ResizeLockModeButton, err := compatModeMenuDialog.DescendantWithTimeout(ctx, chromeui.FindParams{Name: mode.String()}, 10*time.Second)
	if err != nil {
		return errors.Wrapf(err, "failed to find the %s button on the compat mode menu", mode)
	}
	defer ResizeLockModeButton.Release(ctx)

	return ResizeLockModeButton.LeftClick(ctx)
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

// ToggleAppManagementSettingToggle opens the app-management page for the given app via the shelf icon, toggles the resize lock setting, and verifies the states of the app and the setting toggle.
func ToggleAppManagementSettingToggle(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC, d *ui.Device, cr *chrome.Chrome, activity *arc.Activity, appName string, currentMode, nextMode ResizeLockMode, method InputMethodType, keyboard *input.KeyboardEventWriter) error {
	// This check must be done before opening the Chrome OS settings page so it won't affect the screenshot taken in one of the checks.
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
			return errors.Wrap(err, "failed to toggle the resize-lock setting toggle on the Chrome OS settings via click")
		}
	case InputMethodKeyEvent:
		if err := shiftViaTabAndEnter(ctx, tconn, nil, chromeui.FindParams{Name: AppManagementSettingToggleName}, keyboard); err != nil {
			return errors.Wrap(err, "failed to toggle the resize-lock setting toggle on the Chrome OS settings via keyboard")
		}
	}

	if err := checkAppManagementSettingToggleState(ctx, tconn, nextMode); err != nil {
		return errors.Wrap(err, "failed to verify the state of the setting toggle after toggling the setting")
	}

	if err := CloseAppManagementSetting(ctx, tconn); err != nil {
		return errors.Wrapf(err, "failed to close the app management page of %s", appName)
	}

	// This check must be done after closing the Chrome OS settings page so it won't affect the screenshot taken in one of the checks.
	if err := CheckResizeLockState(ctx, tconn, a, d, cr, activity, nextMode, false /* isSplashVisible */); err != nil {
		return errors.Wrapf(err, "failed to verify resize lock state of %s", activity.ActivityName())
	}

	return nil
}

// toggleAppManagementSettingToggleViaClick toggles the resize-lock setting toggle via click.
func toggleAppManagementSettingToggleViaClick(ctx context.Context, tconn *chrome.TestConn) error {
	settingToggle, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{Name: AppManagementSettingToggleName}, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to find the setting toggle")
	}
	defer settingToggle.Release(ctx)

	return settingToggle.LeftClick(ctx)
}

// OpenAppManagementSetting opens the app management page if the given app.
func OpenAppManagementSetting(ctx context.Context, tconn *chrome.TestConn, appName string) error {
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

// CloseAppManagementSetting closes any open app management page.
func CloseAppManagementSetting(ctx context.Context, tconn *chrome.TestConn) error {
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
func checkAppManagementSettingToggleState(ctx context.Context, tconn *chrome.TestConn, mode ResizeLockMode) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		settingToggle, err := chromeui.FindWithTimeout(ctx, tconn, chromeui.FindParams{Name: AppManagementSettingToggleName}, 2*time.Second)
		if err != nil {
			return errors.Wrap(err, "failed to find the resize lock setting toggle on the app-management page")
		}
		defer settingToggle.Release(ctx)

		if ((mode == PhoneResizeLockMode || mode == TabletResizeLockMode) && settingToggle.Checked == chromeui.CheckedStateFalse) ||
			(mode == ResizableTogglableResizeLockMode && settingToggle.Checked == chromeui.CheckedStateTrue) {
			return errors.Errorf("the app-management resize lock setting value (%v) doesn't match the expected curent state (%s)", settingToggle.Checked, mode)
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
