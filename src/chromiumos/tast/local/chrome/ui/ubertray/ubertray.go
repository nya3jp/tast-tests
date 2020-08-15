// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ubertray is for controlling the Ubertray directly from the UI.
package ubertray

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

var ubertrayParams ui.FindParams = ui.FindParams{
	ClassName: "BubbleFrameView",
}

const uiTimeout = 10 * time.Second

func findStatusArea(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	return ui.FindWithTimeout(ctx, tconn, params, uiTimeout)
}

// Rect returns Chrome OS's Ubertray rect, in DPs.
// Returns error if the Ubertray is not present.
func Rect(ctx context.Context, tconn *chrome.TestConn) (coords.Rect, error) {
	ubertray, err := ui.FindWithTimeout(ctx, tconn, ubertrayParams, uiTimeout)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to find the ubertray")
	}
	defer ubertray.Release(ctx)
	return ubertray.Location, nil
}

// ClickStatusArea clicks the status area, which is the area on the shelf where info
// such as time and battery level are shown.
func ClickStatusArea(ctx context.Context, tconn *chrome.TestConn) error {
	statusArea, err := findStatusArea(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find status area widget")
	}
	defer statusArea.Release(ctx)

	if err := statusArea.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the status area")
	}
	return nil
}

// IsShown checks if the Ubertray exists in the UI.
func IsShown(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	return ui.Exists(ctx, tconn, ubertrayParams)
}

// Show will click the status area to show the Ubertray and wait for it to appear.
// If the Ubertray is already open, it does nothing.
func Show(ctx context.Context, tconn *chrome.TestConn) error {
	if shown, err := IsShown(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed initial ubertray visibility check")
	} else if shown {
		return nil
	}

	if err := ClickStatusArea(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the status area")
	}

	if err := ui.WaitUntilExists(ctx, tconn, ubertrayParams, uiTimeout); err != nil {
		return errors.Wrap(err, "failed waiting for ubertray to appear")
	}
	return nil
}

// Hide will click the status area to hide the Ubertray if it's currently shown.
// It will then wait for it to be hidden for the duration specified by timeout.
func Hide(ctx context.Context, tconn *chrome.TestConn) error {
	if shown, err := IsShown(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed initial ubertray visibility check")
	} else if !shown {
		return nil
	}

	if err := ClickStatusArea(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the status area")
	}

	if err := ui.WaitUntilGone(ctx, tconn, ubertrayParams, uiTimeout); err != nil {
		return errors.Wrap(err, "failed waiting for ubertray to be hidden")
	}
	return nil
}

// ShowWithRetry will continuously click the status area until the Ubertray is shown,
// for the duration specified by timeout. The Ubertray sometimes does not open if the status area
// is clicked very early in the test, so this function can be used to ensure it will be opened.
// TODO(crbug/1099502): remove this once there's a better indicator for when the status area
// is ready to receive clicks.
func ShowWithRetry(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	statusArea, err := findStatusArea(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the status area widget")
	}
	defer statusArea.Release(ctx)

	f := func(ctx context.Context) (bool, error) {
		return ui.Exists(ctx, tconn, ubertrayParams)
	}
	if err := statusArea.LeftClickUntil(ctx, f, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "ubertray not shown")
	}
	return nil
}

// QuickSetting represents the name of a quick setting in the Ubertray.
// These names are contained in the Name attribute of the automation node
// for the corresponding pod icon button, so they can be used to find the
// buttons in the UI.
type QuickSetting string

// List of quick setting names, derived from the corresponding pod icon button node names.
// Character case in the names should exactly match the pod icon button node Name attribute.
const (
	QuickSettingBluetooth     QuickSetting = "Bluetooth"
	QuickSettingDoNotDisturb  QuickSetting = "Do not disturb"
	QuickSettingNightLight    QuickSetting = "Night Light"
	QuickSettingNetwork       QuickSetting = "network"
	QuickSettingAccessibility QuickSetting = "accessibility"
)

// PodIconParams generates ui.FindParams for the specified quick setting pod.
func PodIconParams(setting QuickSetting) (ui.FindParams, error) {
	// The pod icon names change based on their state, but a substring containing the setting name stays
	// the same regardless of state, so we can match that in the name attribute.
	r, err := regexp.Compile(string(setting))
	if err != nil {
		return ui.FindParams{}, errors.Wrapf(err, "failed to compile regexp for %v pod icon name attribute", setting)
	}
	podParams := ui.FindParams{
		ClassName:  "FeaturePodIconButton",
		Attributes: map[string]interface{}{"name": r},
	}

	return podParams, nil
}

// findQuickSettingPod finds the UI node corresponding to the specified quick setting pod icon.
func findQuickSettingPod(ctx context.Context, tconn *chrome.TestConn, setting QuickSetting) (*ui.Node, error) {
	podParams, err := PodIconParams(setting)
	if err != nil {
		return nil, err
	}

	pod, err := ui.FindWithTimeout(ctx, tconn, podParams, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find the pod icon button")
	}
	return pod, nil
}

// ubertrayCleanup returns a function that will restore the Ubertray to its original state (shown or hidden).
func ubertrayCleanup(ctx context.Context, tconn *chrome.TestConn) (func(ctx context.Context) error, error) {
	initiallyShown, err := IsShown(ctx, tconn)
	if err != nil {
		return nil, err
	}
	if !initiallyShown {
		return func(ctx context.Context) error {
			return Hide(ctx, tconn)
		}, nil
	}
	return func(ctx context.Context) error {
		return Show(ctx, tconn)
	}, nil
}

// QuickSettingEnabled checks if the specified Ubertray quick setting is on or off.
// In order to check the setting, the Ubertray will be shown if it's not already,
// but the original state will be restored once the check is complete.
func QuickSettingEnabled(ctx context.Context, tconn *chrome.TestConn, setting QuickSetting) (bool, error) {
	cleanup, err := ubertrayCleanup(ctx, tconn)
	if err != nil {
		return false, err
	}
	defer cleanup(ctx)

	if err := Show(ctx, tconn); err != nil {
		return false, err
	}

	pod, err := findQuickSettingPod(ctx, tconn, setting)
	if err != nil {
		return false, errors.Wrap(err, "failed to find the pod icon button")
	}
	defer pod.Release(ctx)

	switch status := pod.Checked; status {
	case ui.CheckedStateTrue:
		return true, nil
	case ui.CheckedStateFalse:
		return false, nil
	default:
		return false, errors.New("invalid checked state for pod icon button; quick setting may not be toggleable")
	}
}

// ToggleQuickSetting toggles quick settings by clicking the corresponding pod icon in the Ubertray.
// The Ubertray will be shown if it's not already, but the original state will be restored after toggling the setting.
func ToggleQuickSetting(ctx context.Context, tconn *chrome.TestConn, setting QuickSetting, enable bool) (func(ctx context.Context) error, error) {
	cleanup, err := ubertrayCleanup(ctx, tconn)
	if err != nil {
		return nil, err
	}
	defer cleanup(ctx)

	if err := Show(ctx, tconn); err != nil {
		return nil, err
	}

	currentState, err := QuickSettingEnabled(ctx, tconn, setting)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get initial setting state")
	} else if currentState == enable {
		return nil, nil
	}

	pod, err := findQuickSettingPod(ctx, tconn, setting)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find the pod icon button")
	}
	defer pod.Release(ctx)

	if err := pod.LeftClick(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click the pod icon button")
	}
	return func(ctx context.Context) error {
		_, err := ToggleQuickSetting(ctx, tconn, setting, currentState)
		return err
	}, nil
}

// IsPodRestricted checks if a pod icon is restricted and unable to be used on the lock screen.
func IsPodRestricted(ctx context.Context, tconn *chrome.TestConn, setting QuickSetting) (bool, error) {
	if err := Show(ctx, tconn); err != nil {
		return false, err
	}

	var pod *ui.Node
	var err error

	// Network is a special case, its usual name is not included in the Name attribute when it's
	// restricted on the lock screen. Its parent element has a distinct class name we can use to find it instead.
	if setting == QuickSettingNetwork {
		networkPod, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{ClassName: "NetworkFeaturePodButton"}, uiTimeout)
		if err != nil {
			return false, errors.Wrap(err, "failed to find parent element of network button")
		}
		defer networkPod.Release(ctx)
		pod, err = networkPod.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeToggleButton}, uiTimeout)
	} else {
		pod, err = findQuickSettingPod(ctx, tconn, setting)
	}

	if err != nil {
		return false, errors.Wrapf(err, "failed to find the %v pod icon node", setting)
	}
	defer pod.Release(ctx)

	if pod.Restriction == ui.RestrictionDisabled {
		return true, nil
	}
	return false, nil
}

// SignoutBtnParams are the UI params for the 'Sign out' ubertray button.
var SignoutBtnParams ui.FindParams = ui.FindParams{
	Role:      ui.RoleTypeButton,
	Name:      "Sign out",
	ClassName: "SignOutButton",
}

// ShutdownBtnParams are the UI params for the shutdown button in the ubertray.
var ShutdownBtnParams ui.FindParams = ui.FindParams{
	Name:      "Shut down",
	ClassName: "TopShortcutButton",
}

// LockBtnParams are the UI params for the ubertray's lock button.
var LockBtnParams ui.FindParams = ui.FindParams{
	Name:      "Lock",
	ClassName: "TopShortcutButton",
}

// SettingsBtnParams are the UI params for the ubertray's setting button.
var SettingsBtnParams ui.FindParams = ui.FindParams{
	Name:      "Settings",
	ClassName: "TopShortcutButton",
}

// CollapseBtnParams are the UI params for the collapse button, which collapses and expands the Ubertray.
var CollapseBtnParams ui.FindParams = ui.FindParams{
	Role:      ui.RoleTypeButton,
	ClassName: "CollapseButton",
}

// clickUntilGone clicks the UI node with the given params until it goes away,
// indicating that it has successfully been clicked. Used here for clicking the
// ubertray's top shortcut buttons, since they are not always clickable
// immediately after opening the ubertray.
// TODO(crbug/1099502): once top shortcut button clickability can be determined
// from the UI node, replace this method with single click actions.
func clickUntilGone(ctx context.Context, tconn *chrome.TestConn, params ui.FindParams) error {
	node, err := ui.FindWithTimeout(ctx, tconn, params, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find the specified node")
	}

	condition := func(ctx context.Context) (bool, error) {
		exists, err := ui.Exists(ctx, tconn, params)
		return !exists, err
	}
	opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}
	if err := node.LeftClickUntil(ctx, condition, &opts); err != nil {
		return errors.Wrap(err, "node still present after clicking")
	}

	return nil
}

// OpenSettings will launch the Settings app by clicking on the Settings
// icon in the Ubertray and wait for its icon to appear in the shelf.
// The Ubertray will be opened if not already shown.
func OpenSettings(ctx context.Context, tconn *chrome.TestConn) error {
	if err := Show(ctx, tconn); err != nil {
		return err
	}

	if err := clickUntilGone(ctx, tconn, SettingsBtnParams); err != nil {
		return err
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID); err != nil {
		return errors.Wrapf(err, "settings app did not open within %v seconds", uiTimeout)
	}

	return nil
}

// LockScreen locks the screen.
func LockScreen(ctx context.Context, tconn *chrome.TestConn) error {
	if err := Show(ctx, tconn); err != nil {
		return err
	}

	if err := clickUntilGone(ctx, tconn, LockBtnParams); err != nil {
		return err
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, uiTimeout); err != nil {
		return errors.Wrapf(err, "waiting for screen to be locked failed (last status %+v)", st)
	}

	return nil

}

// IsCollapsed checks if the ubertray is collapsed or expanded.
func IsCollapsed(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	// Collapsed state can be determined by the presence of sliders and pod icon labels.
	volumeSlider, err := ui.Exists(ctx, tconn, VolumeSliderParams)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if volume slider exists")
	}

	brightnessSlider, err := ui.Exists(ctx, tconn, BrightnessSliderParams)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if brightness slider exists")
	}

	podIconLabels, err := ui.Exists(ctx, tconn, ui.FindParams{ClassName: "FeaturePodLabelButton"})
	if err != nil {
		return false, errors.Wrap(err, "failed checking if pod icon labels exist")
	}

	if volumeSlider && brightnessSlider && podIconLabels {
		return false, nil
	}
	if !volumeSlider && !brightnessSlider && !podIconLabels {
		return true, nil
	}
	return false, errors.Errorf(
		"unable to determine if ubertray is collapsed or expanded; volume shown: %v, brightness shown: %v, pod labels shown: %v",
		volumeSlider, brightnessSlider, podIconLabels,
	)
}

// ToggleCollapsed toggles between the Ubertray's collapsed and expanded states.
func ToggleCollapsed(ctx context.Context, tconn *chrome.TestConn) error {
	initialState, err := IsCollapsed(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed initial collapsed state check")
	}

	resizeBtn, err := ui.FindWithTimeout(ctx, tconn, CollapseBtnParams, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find ubertray collapse/expand button")
	}
	defer resizeBtn.Release(ctx)

	// Wait for the resize to finish by polling for the ubertray rect's height to stop changing.
	ubertraySize, err := Rect(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed getting initial ubertray size")
	}
	initialSize := ubertraySize

	if err := resizeBtn.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the ubertray collapse button")
	}
	resizeDone := func(ctx context.Context) error {
		currentSize, err := Rect(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		if currentSize.Height == initialSize.Height {
			return errors.New("ubertray resizing hasn't started yet")
		}
		if currentSize.Height != ubertraySize.Height {
			ubertraySize = currentSize
			return errors.New("ubertray still resizing")
		}
		return nil
	}

	if err := testing.Poll(ctx, resizeDone, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: uiTimeout}); err != nil {
		return errors.Wrap(err, "failed waiting for expand/collapse animation to complete")
	}

	finalState, err := IsCollapsed(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed collapsed state check after clicking the button")
	}

	if initialState == finalState {
		return errors.Errorf("ubertray collapse state was not toggled; collapsed state remained %v", initialState)
	}

	return nil
}

// AreNotificationsHidden checks that the 'Notifications are hidden' label appears and that no notifications are visible.
func AreNotificationsHidden(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	if err := Show(ctx, tconn); err != nil {
		return false, err
	}

	// Wait for the 'Notifications are hidden' label at the top of the ubertray.
	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{ClassName: "NotificationHiddenView"}, uiTimeout); err != nil {
		return false, errors.Wrap(err, "failed to find notifications hidden view")
	}

	// Also check that no notifications are shown in the UI.
	exists, err := ui.Exists(ctx, tconn, ui.FindParams{Name: "Notification Center", ClassName: "Widget"})
	if err != nil {
		return false, errors.Wrap(err, "failed checking if notification node exists")
	}
	return exists, nil
}

// VolumeSliderParams are the UI params for the ubertray volume slider.
var VolumeSliderParams ui.FindParams = ui.FindParams{
	Name:      "Volume",
	ClassName: "Slider",
}

// BrightnessSliderParams are the UI params for the ubertray brightness slider.
var BrightnessSliderParams ui.FindParams = ui.FindParams{
	Name:      "Brightness",
	ClassName: "Slider",
}

// DateViewParams are the UI params for the ubertray date/time display.
var DateViewParams ui.FindParams = ui.FindParams{
	Role:      ui.RoleTypeButton,
	ClassName: "DateView",
}

// BatteryViewParams are the UI params for the ubertray date/time display.
var BatteryViewParams ui.FindParams = ui.FindParams{
	Role:      ui.RoleTypeLabelText,
	ClassName: "BatteryView",
}
