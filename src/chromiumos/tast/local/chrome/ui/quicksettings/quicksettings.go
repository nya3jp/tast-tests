// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package quicksettings is for controlling the Quick Settings directly from the UI.
package quicksettings

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const uiTimeout = 10 * time.Second

// findStatusArea finds the status area UI node.
func findStatusArea(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	params := ui.FindParams{
		ClassName: "UnifiedSystemTray",
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

// Shown checks if Quick Settings exists in the UI.
func Shown(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	return ui.Exists(ctx, tconn, quickSettingsParams)
}

// Show will click the status area to show Quick Settings and wait for it to appear.
// If Quick Settings is already open, it does nothing. Quick Settings will remain
// open between tests if it's not closed explicitly, so this should be accompanied
// by a deferred call to Hide to clean up the UI before starting other tests.
func Show(ctx context.Context, tconn *chrome.TestConn) error {
	if shown, err := Shown(ctx, tconn); err != nil {
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
	if shown, err := Shown(ctx, tconn); err != nil {
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

// PodIconParams generates ui.FindParams for the specified quick setting pod.
func PodIconParams(setting SettingPod) (ui.FindParams, error) {
	// The network pod cannot be easily found by its Name attribute in both logged-in and lock screen states.
	// Instead, find it by its unique ClassName.
	if setting == SettingPodNetwork {
		return ui.FindParams{ClassName: "NetworkFeaturePodButton"}, nil
	}

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

// findPodButton finds the UI node corresponding to the specified quick setting pod icon button.
func findPodButton(ctx context.Context, tconn *chrome.TestConn, setting SettingPod) (*ui.Node, error) {
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

// ensureVisible ensures that Quick Settings is shown. If it's not visible, this function will
// show Quick Settings and return a cleanup function to hide it. If it is already visible,
// this function will do nothing and the returned function will do nothing, since no cleanup is required.
func ensureVisible(ctx context.Context, tconn *chrome.TestConn) (func(ctx context.Context) error, error) {
	shown, err := Shown(ctx, tconn)
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

// PodRestricted checks if a pod icon is restricted and unable to be used on the lock screen.
func PodRestricted(ctx context.Context, tconn *chrome.TestConn, setting SettingPod) (bool, error) {
	cleanup, err := ensureVisible(ctx, tconn)
	if err != nil {
		return false, err
	}
	defer cleanup(ctx)

	pod, err := findPodButton(ctx, tconn, setting)
	if err != nil {
		return false, errors.Wrapf(err, "failed to find the %v pod icon node", setting)
	}
	defer pod.Release(ctx)

	return pod.Restriction == ui.RestrictionDisabled, nil
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

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID, time.Minute); err != nil {
		return errors.Wrapf(err, "settings app did not open within %v seconds", uiTimeout)
	}

	return nil
}

// LockScreen locks the screen.
func LockScreen(ctx context.Context, tconn *chrome.TestConn) error {
	cleanup, err := ensureVisible(ctx, tconn)
	if err != nil {
		return err
	}
	defer cleanup(ctx)

	lockBtn, err := ui.FindWithTimeout(ctx, tconn, LockBtnParams, uiTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to find lock button")
	}
	defer lockBtn.Release(ctx)

	if err := lockBtn.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click lock button")
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, uiTimeout); err != nil {
		return errors.Wrapf(err, "waiting for screen to be locked failed (last status %+v)", st)
	}

	return nil

}

// NotificationsHidden checks that the 'Notifications are hidden' label appears and that no notifications are visible.
func NotificationsHidden(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	cleanup, err := ensureVisible(ctx, tconn)
	if err != nil {
		return false, err
	}
	defer cleanup(ctx)

	// Wait for the 'Notifications are hidden' label at the top of Quick Settings.
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

// findSlider finds the UI node for the specified slider. Callers should defer releasing the returned node.
func findSlider(ctx context.Context, tconn *chrome.TestConn, slider SliderType) (*ui.Node, error) {
	// The mic gain slider is on the audio settings page of Quick Settings, so we need to navigate there first.
	if slider == SliderTypeMicGain {
		if err := OpenAudioSettings(ctx, tconn); err != nil {
			return nil, err
		}
	}

	s, err := ui.FindWithTimeout(ctx, tconn, SliderParamMap[slider], uiTimeout)
	if err != nil {
		return nil, errors.Wrapf(err, "failed finding the %v slider", slider)
	}
	return s, nil
}

// OpenAudioSettings opens Quick Settings' audio settings page. It does nothing if the page is already open.
func OpenAudioSettings(ctx context.Context, tconn *chrome.TestConn) error {
	cleanup, err := ensureVisible(ctx, tconn)
	if err != nil {
		return err
	}
	defer cleanup(ctx)

	audioParams := ui.FindParams{Role: ui.RoleTypeButton, Name: "Audio settings"}

	if exists, err := ui.Exists(ctx, tconn, audioParams); err != nil {
		return errors.Wrap(err, "failed to check if audio settings button exists")
	} else if exists {
		audioBtn, err := ui.Find(ctx, tconn, audioParams)
		if err != nil {
			errors.Wrap(err, "failed to find audio settings button")
		}
		defer audioBtn.Release(ctx)

		if err := audioBtn.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click audio settings button")
		}
	}

	return ui.WaitUntilExists(ctx, tconn, ui.FindParams{ClassName: "AudioDetailedView"}, uiTimeout)
}

// SliderValue returns the slider value as an integer.
// The slider node's value taken directly is a string expressing a percentage, like "50%".
func SliderValue(ctx context.Context, tconn *chrome.TestConn, slider SliderType) (int, error) {
	cleanup, err := ensureVisible(ctx, tconn)
	if err != nil {
		return 0, err
	}
	defer cleanup(ctx)

	s, err := findSlider(ctx, tconn, slider)
	if err != nil {
		return 0, err
	}
	defer s.Release(ctx)

	percent := strings.Replace(s.Value, "%", "", 1)
	level, err := strconv.Atoi(percent)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to convert %v to int", percent)
	}
	return level, nil
}

// focusSlider puts the keyboard focus on the slider. The keyboard can then be used to change the slider level.
// TODO(crbug/1123231): use better slider automation controls if possible, instead of keyboard controls.
func focusSlider(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, slider SliderType) error {
	s, err := findSlider(ctx, tconn, slider)
	if err != nil {
		return err
	}
	defer s.Release(ctx)

	// Return if already focused.
	if s.State[ui.StateTypeFocused] == true {
		return nil
	}

	// Press tab to ensure keyboard focus is already in Quick Settings, otherwise it may not receive the focus.
	if err := kb.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "failed to press tab key")
	}

	if err := s.FocusAndWait(ctx, uiTimeout); err != nil {
		return errors.Wrapf(err, "failed to focus the %v slider", slider)
	}
	return nil
}

// changeSlider increments or decrements the slider using the keyboard.
// TODO(crbug/1123231): use better slider automation controls if possible, instead of keyboard controls.
func changeSlider(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, slider SliderType, increase bool) error {
	key := "up"
	if !increase {
		key = "down"
	}

	if err := focusSlider(ctx, tconn, kb, slider); err != nil {
		return err
	}

	initial, err := SliderValue(ctx, tconn, slider)
	if err != nil {
		return err
	}

	if err := kb.Accel(ctx, key); err != nil {
		return errors.Wrapf(err, "failed to press %v arrow key", key)
	}

	// The slider shouldn't move at all if we try to increase/decrease past the min/max value.
	// The polling below will time out in these cases, so just return immediately after pressing the key.
	if (initial == 0 && !increase) || (initial == 100 && increase) {
		return nil
	}

	// The value changes smoothly as the slider animates, so wait for it to finish before returning the final value.
	previous := initial
	slidingDone := func(ctx context.Context) error {
		if current, err := SliderValue(ctx, tconn, slider); err != nil {
			return testing.PollBreak(err)
		} else if current == initial {
			return errors.New("slider hasn't started moving yet")
		} else if current == previous {
			return nil
		} else if (increase && current < previous) || (!increase && current > previous) {
			return testing.PollBreak(errors.Errorf("slider moved opposite of the expected direction from %v to %v", previous, current))
		} else {
			previous = current
			return errors.New("slider still sliding")
		}
	}

	if err := testing.Poll(ctx, slidingDone, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: uiTimeout}); err != nil {
		return errors.Wrap(err, "failed waiting for slider animation to complete")
	}
	return nil
}

// IncreaseSlider increments the slider positively using the keyboard and returns the new level.
func IncreaseSlider(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, slider SliderType) (int, error) {
	cleanup, err := ensureVisible(ctx, tconn)
	if err != nil {
		return 0, err
	}
	defer cleanup(ctx)

	if err := changeSlider(ctx, tconn, kb, slider, true); err != nil {
		return 0, err
	}

	return SliderValue(ctx, tconn, slider)
}

// DecreaseSlider increments the slider positively using the keyboard and returns the new level.
func DecreaseSlider(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, slider SliderType) (int, error) {
	cleanup, err := ensureVisible(ctx, tconn)
	if err != nil {
		return 0, err
	}
	defer cleanup(ctx)

	if err := changeSlider(ctx, tconn, kb, slider, false); err != nil {
		return 0, err
	}

	return SliderValue(ctx, tconn, slider)
}

// micToggleButton navigates to the audio settings page and returns the UI node for the mic toggle (mute/unmute) button. Callers should defer releasing the returned node.
func micToggleButton(ctx context.Context, tconn *chrome.TestConn) (*ui.Node, error) {
	if err := OpenAudioSettings(ctx, tconn); err != nil {
		return nil, err
	}
	btn, err := ui.FindWithTimeout(ctx, tconn, MicToggleParams, uiTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find the mic toggle button")
	}
	return btn, nil
}

// MicEnabled checks if the microphone is enabled (unmuted).
func MicEnabled(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	btn, err := micToggleButton(ctx, tconn)
	if err != nil {
		return false, err
	}
	defer btn.Release(ctx)
	return btn.Checked == ui.CheckedStateTrue, nil
}

// ToggleMic toggles the microphone's enabled state by clicking the microphone icon adjacent to the slider.
// If the microphone is already in the desired state, this will do nothing.
func ToggleMic(ctx context.Context, tconn *chrome.TestConn, enable bool) error {
	if current, err := MicEnabled(ctx, tconn); err != nil {
		return err
	} else if current != enable {
		btn, err := micToggleButton(ctx, tconn)
		if err != nil {
			return err
		}
		defer btn.Release(ctx)
		if err := btn.LeftClick(ctx); err != nil {
			return errors.Wrap(err, "failed to click mic toggle button")
		}
	}
	return nil
}

// SelectAudioOption selects the audio input or output device with the given name from the audio settings page.
func SelectAudioOption(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, device string) error {
	if err := OpenAudioSettings(ctx, tconn); err != nil {
		return err
	}

	option, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeCheckBox, Name: device}, uiTimeout)
	if err != nil {
		return errors.Wrapf(err, "failed finding node for %v audio option", device)
	}

	// If there are several audio options available, the target option may be out of view.
	// In that case we need to scroll to it before we can click it. Calling FocusAndWait on the node will scroll it into view.
	// TODO(crbug/1140196): Remove this Tab press when it's no longer needed for focus to work correctly within Quick Settings.
	if err := kb.Accel(ctx, "Tab"); err != nil {
		return errors.Wrap(err, "failed to press tab key")
	}
	if err := option.FocusAndWait(ctx, uiTimeout); err != nil {
		return errors.Wrapf(err, "failed to focus the %v audio option", device)
	}
	if err := option.LeftClick(ctx); err != nil {
		return errors.Wrapf(err, "failed to click %v audio option", device)
	}

	return nil
}

// RestrictedSettingsPods returns the setting pods that are restricted when Quick Settings is opened while a user is not signed in.
func RestrictedSettingsPods(ctx context.Context) ([]SettingPod, error) {

	restrictedPods := []SettingPod{SettingPodNetwork}

	// First check for the bluetooth pod on devices with at least 1 bluetooth adapter.
	// If bluetooth adapters exists, add the bluetooth settingPod in the restrictedPods list.
	if adapters, err := bluetooth.Adapters(ctx); err != nil {
		return restrictedPods, errors.Wrap(err, "unable to get Bluetooth adapters")
	} else if len(adapters) > 0 {
		restrictedPods = append(restrictedPods, SettingPodBluetooth)
	}

	return restrictedPods, nil
}

// CommonElementsInQuickSettings returns a map that contains ui.FindParams for Quick Settings UI elements that are present in all sign-in states (signed in, signed out, screen locked).
// The keys of the map are descriptive names for the UI elements.
func CommonElementsInQuickSettings(ctx context.Context, tconn *chrome.TestConn, battery bool) (map[string]ui.FindParams, error) {

	// Associate the params with a descriptive name for better error reporting.
	getNodes := map[string]ui.FindParams{
		"Shutdown button":   ShutdownBtnParams,
		"Collapse button":   CollapseBtnParams,
		"Volume slider":     VolumeSliderParams,
		"Brightness slider": BrightnessSliderParams,
		"Date/time display": DateViewParams,
	}

	if battery {
		getNodes["Battery display"] = BatteryViewParams
	}

	// Check that the expected accessibility UI element is shown in Quick Settings.
	accessibilityParams, err := PodIconParams(SettingPodAccessibility)
	if err != nil {
		return getNodes, errors.Wrap(err, "failed to get params for accessibility pod icon")
	}

	getNodes["Accessibility pod"] = accessibilityParams

	return getNodes, nil
}
