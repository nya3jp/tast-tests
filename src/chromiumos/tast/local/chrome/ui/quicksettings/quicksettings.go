// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package quicksettings is for controlling the Quick Settings directly from the UI.
package quicksettings

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

var quickSettingsParams ui.FindParams = ui.FindParams{
	ClassName: "BubbleFrameView",
}

const uiTimeout = 10 * time.Second

// findStatusArea finds the status area UI node.
func findStatusArea(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	return ui.FindWithTimeout(ctx, tconn, params, uiTimeout)
}

// Rect returns a coords.Rect struct for the Quick Settings area, which contains
// coordinate information about the rectangular region it occupies on the screen.
func Rect(ctx context.Context, tconn *chrome.TestConn) (coords.Rect, error) {
	quickSettings, err := ui.FindWithTimeout(ctx, tconn, quickSettingsParams, uiTimeout)
	if err != nil {
		return coords.Rect{}, errors.Wrap(err, "failed to find quick settings")
	}
	defer quickSettings.Release(ctx)
	return quickSettings.Location, nil
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

// IsShown checks if Quick Settings exists in the UI.
func IsShown(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	return ui.Exists(ctx, tconn, quickSettingsParams)
}

// Show will click the status area to show Quick Settings and wait for it to appear.
// If Quick Settings is already open, it does nothing. Quick Settings will remain
// open between tests if it's not closed explicitly, so this should be accompanied
// by a deferred call to Hide to clean up the UI before starting other tests.
func Show(ctx context.Context, tconn *chrome.TestConn) error {
	if shown, err := IsShown(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed initial quick settings visibility check")
	} else if shown {
		return nil
	}

	if err := ClickStatusArea(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the status area")
	}

	if err := ui.WaitUntilExists(ctx, tconn, quickSettingsParams, uiTimeout); err != nil {
		return errors.Wrap(err, "failed waiting for quick settings to appear")
	}
	return nil
}

// Hide will click the status area to hide Quick Settings if it's currently shown.
// It will then wait for it to be hidden for the duration specified by timeout.
func Hide(ctx context.Context, tconn *chrome.TestConn) error {
	if shown, err := IsShown(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed initial quick settings visibility check")
	} else if !shown {
		return nil
	}

	if err := ClickStatusArea(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to click the status area")
	}

	if err := ui.WaitUntilGone(ctx, tconn, quickSettingsParams, uiTimeout); err != nil {
		return errors.Wrap(err, "failed waiting for quick settings to be hidden")
	}
	return nil
}

// ShowWithRetry will continuously click the status area until Quick Settings is shown,
// for the duration specified by timeout. Quick Settings sometimes does not open if the status area
// is clicked very early in the test, so this function can be used to ensure it will be opened.
// Callers should also defer a call to Hide to ensure Quick Settings is closed between tests.
// TODO(crbug/1099502): remove this once there's a better indicator for when the status area
// is ready to receive clicks.
func ShowWithRetry(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	statusArea, err := findStatusArea(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the status area widget")
	}
	defer statusArea.Release(ctx)

	f := func(ctx context.Context) (bool, error) {
		return ui.Exists(ctx, tconn, quickSettingsParams)
	}
	if err := statusArea.LeftClickUntil(ctx, f, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "quick settings not shown")
	}
	return nil
}

// SettingPod represents the name of a setting pod in Quick Settings.
// These names are contained in the Name attribute of the automation node
// for the corresponding pod icon button, so they can be used to find the
// buttons in the UI.
type SettingPod string

// List of quick setting names, derived from the corresponding pod icon button node names.
// Character case in the names should exactly match the pod icon button node Name attribute.
const (
	SettingPodBluetooth    SettingPod = "Bluetooth"
	SettingPodDoNotDisturb SettingPod = "Do not disturb"
	SettingPodNightLight   SettingPod = "Night Light"
	SettingPodNetwork      SettingPod = "network"
)

// findPodButton finds the UI node corresponding to the specified quick setting pod icon button.
func findPodButton(ctx context.Context, tconn *chrome.TestConn, setting SettingPod) (*ui.Node, error) {
	// The pod icon names change based on their state, but a substring containing the setting name stays
	// the same regardless of state, so we can match that in the name attribute.
	r, err := regexp.Compile(string(setting))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compile regexp for %v pod icon name attribute", setting)
	}
	podParams := ui.FindParams{
		ClassName:  "FeaturePodIconButton",
		Attributes: map[string]interface{}{"name": r},
	}

	pod, err := ui.FindWithTimeout(ctx, tconn, podParams, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find the pod icon button")
	}
	return pod, nil
}

// ensureVisible ensures that Quick Settings is shown. If it's not visible, this function will
// show Quick Settings and return a cleanup function to hide it. If it is already visible,
// this function will do nothing and the returned function will do nothing, since no cleanup is required.
func ensureVisible(ctx context.Context, tconn *chrome.TestConn) (func(ctx context.Context) error, error) {
	shown, err := IsShown(ctx, tconn)
	if err != nil {
		return nil, err
	}

	if !shown {
		if err := Show(ctx, tconn); err != nil {
			return nil, err
		}
		return func(ctx context.Context) error {
			return Hide(ctx, tconn)
		}, nil
	}

	return func(ctx context.Context) error {
		return nil
	}, nil
}

// SettingEnabled checks if the specified quick setting is on or off.
// In order to check the setting, Quick Settings will be shown if it's not already,
// but the original state will be restored once the check is complete.
func SettingEnabled(ctx context.Context, tconn *chrome.TestConn, setting SettingPod) (bool, error) {
	cleanup, err := ensureVisible(ctx, tconn)
	if err != nil {
		return false, err
	}
	defer cleanup(ctx)

	pod, err := findPodButton(ctx, tconn, setting)
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

// ToggleSetting toggles a quick setting by clicking the corresponding pod icon.
// If Quick Settings is not already shown, it will be opened and then closed once the setting is toggled.
func ToggleSetting(ctx context.Context, tconn *chrome.TestConn, setting SettingPod, enable bool) error {
	cleanup, err := ensureVisible(ctx, tconn)
	if err != nil {
		return err
	}
	defer cleanup(ctx)

	if currentState, err := SettingEnabled(ctx, tconn, setting); err != nil {
		return errors.Wrap(err, "failed to get initial setting state")
	} else if currentState == enable {
		return nil
	}

	pod, err := findPodButton(ctx, tconn, setting)
	if err != nil {
		return errors.Wrap(err, "failed to find the pod icon button")
	}
	defer pod.Release(ctx)

	if err := pod.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the pod icon button")
	}
	return nil
}

// OpenSettingsApp will launch the Settings app by clicking on the Settings icon and wait
// for its icon to appear in the shelf. Quick Settings will be opened if not already shown.
func OpenSettingsApp(ctx context.Context, tconn *chrome.TestConn) error {
	cleanup, err := ensureVisible(ctx, tconn)
	if err != nil {
		return err
	}
	defer cleanup(ctx)

	params := ui.FindParams{
		Name:      "Settings",
		ClassName: "TopShortcutButton",
	}

	settingsBtn, err := ui.FindWithTimeout(ctx, tconn, params, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find settings top shortcut button")
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
