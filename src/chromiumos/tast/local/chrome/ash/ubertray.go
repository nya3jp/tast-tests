// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

var ubertrayParams ui.FindParams = ui.FindParams{
	ClassName: "BubbleFrameView",
}

const uiTimeout = 10 * time.Second

// ToggleableQuickSetting represents Ubertray quick settings that can be toggled on and off.
type ToggleableQuickSetting string

// Partial names for toggleable quick settings.
// These serve 2 purposes:
// 1. They are sufficient identifiers to find the buttons in the UI.
// 2. They can be used to check the toggle status of the pod icon.
//    The pod icon UI node names will change to include 'on' or 'off' at the
//    end of these strings when the icon is toggled to the respective status.
//    The status does not seem to be discernable in any other way from the node properties.
const (
	Bluetooth    ToggleableQuickSetting = "Toggle Bluetooth. Bluetooth is"
	DoNotDisturb ToggleableQuickSetting = "Toggle Do not disturb. Do not disturb is"
	NightLight   ToggleableQuickSetting = "Toggle Night Light. Night Light is"
)

func findStatusArea(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	return ui.FindWithTimeout(ctx, tconn, params, uiTimeout)
}

// UbertrayRect returns Chrome OS's Ubertray rect, in DPs.
// Returns error if the Ubertray is not present.
func UbertrayRect(ctx context.Context, tconn *chrome.TestConn) (coords.Rect, error) {
	ubertray, err := ui.FindWithTimeout(ctx, tconn, ubertrayParams, uiTimeout)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to get the ubertray")
	}
	defer ubertray.Release(ctx)
	return ubertray.Location, nil
}

// ClickStatusArea clicks the status area, which is the area on the shelf where info
// such as time and battery level are shown.
func ClickStatusArea(ctx context.Context, tconn *chrome.TestConn) error {
	statusArea, err := findStatusArea(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get status area widget")
	}
	defer statusArea.Release(ctx)

	if err := statusArea.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the status area")
	}
	return nil
}

// IsUbertrayShown checks if the Ubertray exists in the UI.
func IsUbertrayShown(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	return ui.Exists(ctx, tconn, ubertrayParams)
}

// ShowUbertray will click the status area to show the Ubertray and wait for it to appear.
// If the Ubertray is already open, do nothing.
func ShowUbertray(ctx context.Context, tconn *chrome.TestConn) error {
	if shown, err := IsUbertrayShown(ctx, tconn); err != nil {
		return errors.Wrap(err, "initial ubertray visibility check failed")
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

// HideUbertray will click the status area to hide the Ubertray if it's currently shown.
// It will then wait for it to be hidden for the duration specified by timeout.
func HideUbertray(ctx context.Context, tconn *chrome.TestConn) error {
	if shown, err := IsUbertrayShown(ctx, tconn); err != nil {
		return errors.Wrap(err, "initial ubertray visibility check failed")
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

// ShowUbertrayWithRetry will continuously click the status area until the Ubertray is shown,
// for the duration specified by timeout. The Ubertray sometimes does not open if the status area
// is clicked very early in the test, so this function can be used to ensure it will be opened.
// TODO(crbug/1099502): remove this once there's a better indicator for when the status area
// is ready to receive clicks.
func ShowUbertrayWithRetry(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	statusArea, err := findStatusArea(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get status area widget")
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

// getQuickSettingPod finds the UI node corresponding to the specified quick setting pod icon.
// The pod icon buttons are distinguished by their name attributes, but the names change based
// on whether the quick setting is toggled on or off. The node is found by partially matching
// the node name. The node name can then be used to check the quick setting toggle status by
// looking for the string "is on" or "is off".
func getQuickSettingPod(ctx context.Context, tconn *chrome.TestConn, setting ToggleableQuickSetting) (*ui.Node, error) {
	r, err := regexp.Compile(string(setting))
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regexp for pod icon name attribute")
	}
	attributes := map[string]interface{}{
		"name": r,
	}
	podParams := ui.FindParams{
		ClassName:  "FeaturePodIconButton",
		Attributes: attributes,
	}

	pod, err := ui.FindWithTimeout(ctx, tconn, podParams, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find the pod icon button")
	}
	return pod, nil
}

// QuickSettingEnabled checks if the specified Ubertray quick setting is on or off.
// In order to check the setting, the Ubertray will be shown if it's not already.
func QuickSettingEnabled(ctx context.Context, tconn *chrome.TestConn, setting ToggleableQuickSetting) (bool, error) {
	if err := ShowUbertray(ctx, tconn); err != nil {
		return false, err
	}

	pod, err := getQuickSettingPod(ctx, tconn, setting)
	if err != nil {
		return false, errors.Wrap(err, "failed to find the pod icon button")
	}
	defer pod.Release(ctx)

	return strings.Contains(pod.Name, "is on"), nil
}

// ToggleQuickSetting toggles quick settings by clicking the corresponding pod icon in the Ubertray.
// The Ubertray will be shown if it's not already.
func ToggleQuickSetting(ctx context.Context, tconn *chrome.TestConn, setting ToggleableQuickSetting, enable bool) error {
	if currentState, err := QuickSettingEnabled(ctx, tconn, setting); err != nil {
		return errors.Wrap(err, "failed to get initial setting state")
	} else if currentState == enable {
		return nil
	}

	pod, err := getQuickSettingPod(ctx, tconn, setting)
	if err != nil {
		return errors.Wrap(err, "failed to find the pod icon button")
	}
	defer pod.Release(ctx)

	if err := pod.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the pod icon button")
	}
	return nil
}

// OpenSettingsFromUbertray will launch the Settings app by clicking on the Settings
// icon in the Ubertray. The Ubertray will be opened if not already shown. This function
// doesn't wait for the Settings app to open before returning, so callers should do so on their own.
func OpenSettingsFromUbertray(ctx context.Context, tconn *chrome.TestConn) error {
	if err := ShowUbertray(ctx, tconn); err != nil {
		return err
	}

	params := ui.FindParams{
		Name:      "Settings",
		ClassName: "TopShortcutButton",
	}

	settingsBtn, err := ui.FindWithTimeout(ctx, tconn, params, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find ubertray settings button")
	}

	// Try clicking the Settings button until it goes away, indicating the click was received.
	// todo(crbug/1099502): determine when this is clickable, and just click it once.
	condition := func(ctx context.Context) (bool, error) {
		exists, err := ui.Exists(ctx, tconn, params)
		return !exists, err
	}
	opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}
	if err := settingsBtn.LeftClickUntil(ctx, condition, &opts); err != nil {
		return errors.Wrap(err, "settings button still present after clicking")
	}
	return nil
}
