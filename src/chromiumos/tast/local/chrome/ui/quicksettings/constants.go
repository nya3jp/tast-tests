// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"regexp"

	"chromiumos/tast/local/chrome/ui"
)

// quickSettingsParams are the parameters to find the Quick Settings area in the UI.
var quickSettingsParams ui.FindParams = ui.FindParams{
	ClassName: "BubbleFrameView",
}

// CollapseBtnParams are the UI params for the collapse button, which collapses and expands Quick Settings.
var CollapseBtnParams ui.FindParams = ui.FindParams{
	Role:      ui.RoleTypeButton,
	ClassName: "CollapseButton",
}

// LockBtnParams are the UI params for Quick Settings' lock button.
var LockBtnParams ui.FindParams = ui.FindParams{
	Name:      "Lock",
	ClassName: "TopShortcutButton",
}

// SettingsBtnParams are the UI params for the Quick Settings' setting button.
var SettingsBtnParams ui.FindParams = ui.FindParams{
	Name:      "Settings",
	ClassName: "TopShortcutButton",
}

// ShutdownBtnParams are the UI params for the shutdown button in Quick Settings.
var ShutdownBtnParams ui.FindParams = ui.FindParams{
	Name:      "Shut down",
	ClassName: "TopShortcutButton",
}

// SignoutBtnParams are the UI params for the 'Sign out' Quick Settings button.
var SignoutBtnParams ui.FindParams = ui.FindParams{
	Role:      ui.RoleTypeButton,
	Name:      "Sign out",
	ClassName: "SignOutButton",
}

// SliderType represents the Quick Settings slider elements.
type SliderType string

// List of descriptive slider names. These don't correspond to any UI node attributes,
// but will be used as keys to map descriptive names to the UI params defined below.
const (
	SliderTypeVolume     SliderType = "Volume"
	SliderTypeBrightness SliderType = "Brightness"
	SliderTypeMicGain    SliderType = "Mic gain"
)

// SliderParamMap maps slider names (SliderType) to the params to find the sliders in the UI.
var SliderParamMap = map[SliderType]ui.FindParams{
	SliderTypeVolume:     VolumeSliderParams,
	SliderTypeBrightness: BrightnessSliderParams,
	SliderTypeMicGain:    MicGainSliderParams,
}

// BrightnessSliderParams are the UI params for the Quick Settings brightness slider.
var BrightnessSliderParams ui.FindParams = ui.FindParams{
	Name:      "Brightness",
	ClassName: "Slider",
	Role:      ui.RoleTypeSlider,
}

// VolumeSliderParams are the UI params for the Quick Settings volume slider.
var VolumeSliderParams ui.FindParams = ui.FindParams{
	Name:      "Volume",
	ClassName: "Slider",
	Role:      ui.RoleTypeSlider,
}

// MicGainSliderParams are the UI params for the Quick Settings mic gain slider.
// The params are identical to the volume slider, but it's located on a different
// page of Quick Settings.
var MicGainSliderParams ui.FindParams = ui.FindParams{
	Name:      "Volume",
	ClassName: "Slider",
	Role:      ui.RoleTypeSlider,
}

// MicToggleParams are the UI params for the button that toggles the microphone's mute status.
var MicToggleParams ui.FindParams = ui.FindParams{
	Role:       ui.RoleTypeToggleButton,
	Attributes: map[string]interface{}{"name": regexp.MustCompile("Toggle Mic")},
}

// ManagedInfoViewParams are the UI params for the Quick Settings management information display.
var ManagedInfoViewParams ui.FindParams = ui.FindParams{
	Role:      ui.RoleTypeButton,
	ClassName: "EnterpriseManagedView",
}

// BatteryViewParams are the UI params for the Quick Settings date/time display.
var BatteryViewParams ui.FindParams = ui.FindParams{
	Role:      ui.RoleTypeLabelText,
	ClassName: "BatteryView",
}

// DateViewParams are the UI params for the Quick Settings date/time display.
var DateViewParams ui.FindParams = ui.FindParams{
	Role:      ui.RoleTypeButton,
	ClassName: "DateView",
}

// SettingPod represents the name of a setting pod in Quick Settings.
// These names are contained in the Name attribute of the automation node
// for the corresponding pod icon button, so they can be used to find the
// buttons in the UI.
type SettingPod string

// List of quick setting names, derived from the corresponding pod icon button node names.
// Character case in the names should exactly match the pod icon button node Name attribute.
const (
	SettingPodAccessibility SettingPod = "accessibility"
	SettingPodBluetooth     SettingPod = "Bluetooth"
	SettingPodDoNotDisturb  SettingPod = "Do not disturb"
	SettingPodNetwork       SettingPod = "network"
	SettingPodNightLight    SettingPod = "Night Light"
	SettingPodNearbyShare   SettingPod = "Nearby Share"
	SettingPodKeyboard      SettingPod = "keyboard"
)
