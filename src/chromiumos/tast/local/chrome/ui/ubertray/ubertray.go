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

// IsShown checks if the Ubertray exists in the UI.
func IsShown(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	return ui.Exists(ctx, tconn, ubertrayParams)
}

// Show will click the status area to show the Ubertray and wait for it to appear.
// If the Ubertray is already open, do nothing.
func Show(ctx context.Context, tconn *chrome.TestConn) error {
	if shown, err := IsShown(ctx, tconn); err != nil {
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

// Hide will click the status area to hide the Ubertray if it's currently shown.
// It will then wait for it to be hidden for the duration specified by timeout.
func Hide(ctx context.Context, tconn *chrome.TestConn) error {
	if shown, err := IsShown(ctx, tconn); err != nil {
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

// ShowWithRetry will continuously click the status area until the Ubertray is shown,
// for the duration specified by timeout. The Ubertray sometimes does not open if the status area
// is clicked very early in the test, so this function can be used to ensure it will be opened.
// TODO(crbug/1099502): remove this once there's a better indicator for when the status area
// is ready to receive clicks.
func ShowWithRetry(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
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

// QuickSetting represents the name of a quick setting in the Ubertray.
// These names are contained in the Name attribute of the automation node
// for the corresponding pod icon button, so they can be used to find the
// buttons in the UI.
type QuickSetting string

// List of quick setting names, derived from the corresponding pod icon button node names.
// Character case in the names should exactly match the pod icon button node Name attribute.
const (
	QuickSettingBluetooth    QuickSetting = "Bluetooth"
	QuickSettingDoNotDisturb QuickSetting = "Do not disturb"
	QuickSettingNightLight   QuickSetting = "Night Light"
	QuickSettingNetwork      QuickSetting = "network"
)

// getQuickSettingPod finds the UI node corresponding to the specified quick setting pod icon.
func getQuickSettingPod(ctx context.Context, tconn *chrome.TestConn, setting QuickSetting) (*ui.Node, error) {
	// The pod icon names change based on their state, but a substring containing the setting name stays
	// the same regardless of state, so we can match that in the name attribute.
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
func QuickSettingEnabled(ctx context.Context, tconn *chrome.TestConn, setting QuickSetting) (bool, error) {
	if err := Show(ctx, tconn); err != nil {
		return false, err
	}

	pod, err := getQuickSettingPod(ctx, tconn, setting)
	if err != nil {
		return false, errors.Wrap(err, "failed to find the pod icon button")
	}
	defer pod.Release(ctx)

	if status := pod.Checked; status == ui.CheckedStateTrue {
		return true, nil
	} else if status == ui.CheckedStateFalse {
		return false, nil
	} else {
		return false, errors.New("invalid checked state for pod icon button; quick setting may not be toggleable")
	}

}

// ToggleQuickSetting toggles quick settings by clicking the corresponding pod icon in the Ubertray.
// The Ubertray will be shown if it's not already.
func ToggleQuickSetting(ctx context.Context, tconn *chrome.TestConn, setting QuickSetting, enable bool) error {
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

// OpenSettings will launch the Settings app by clicking on the Settings
// icon in the Ubertray and wait for its icon to appear in the shelf.
// The Ubertray will be opened if not already shown.
func OpenSettings(ctx context.Context, tconn *chrome.TestConn) error {
	if err := Show(ctx, tconn); err != nil {
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

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID); err != nil {
		return errors.Wrapf(err, "settings app did not open within %v seconds", uiTimeout)
	}

	return nil
}
