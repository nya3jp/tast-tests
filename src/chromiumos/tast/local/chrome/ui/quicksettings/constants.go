// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"regexp"

	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// quickSettingsFinder are the parameters to find the Quick Settings area in the UI.
var quickSettingsFinder = nodewith.ClassName("UnifiedSystemTrayView")

// CollapseBtn are the finder for the collapse button, which collapses and expands Quick Settings.
var CollapseBtn = nodewith.Role(role.Button).ClassName("CollapseButton")

// LockBtn are the finder for Quick Settings' lock button.
var LockBtn = nodewith.Name("Lock").ClassName("TopShortcutButton")

// SettingsBtn are the finder for the Quick Settings' setting button.
var SettingsBtn = nodewith.Name("Settings").ClassName("TopShortcutButton")

// ShutdownBtn are the finder for the shutdown button in Quick Settings.
var ShutdownBtn = nodewith.Name("Shut down").ClassName("TopShortcutButton")

// SignoutBtn are the finder for the 'Sign out' Quick Settings button.
var SignoutBtn = nodewith.Role(role.Button).Name("Sign out").ClassName("SignOutButton")

// SliderType represents the Quick Settings slider elements.
type SliderType string

// List of descriptive slider names. These don't correspond to any UI node attributes,
// but will be used as keys to map descriptive names to the finder defined below.
const (
	SliderTypeVolume     SliderType = "Volume"
	SliderTypeBrightness SliderType = "Brightness"
	SliderTypeMicGain    SliderType = "Mic gain"
)

// SliderParamMap maps slider names (SliderType) to the  to find the sliders in the UI.
var SliderParamMap = map[SliderType]*nodewith.Finder{
	SliderTypeVolume:     VolumeSlider,
	SliderTypeBrightness: BrightnessSlider,
	SliderTypeMicGain:    MicGainSlider,
}

// BrightnessSlider are the finder for the Quick Settings brightness slider.
var BrightnessSlider = nodewith.Name("Brightness").ClassName("Slider").Role(role.Slider)

// VolumeSlider are the finder for the Quick Settings volume slider.
var VolumeSlider = nodewith.Name("Volume").ClassName("Slider").Role(role.Slider)

// MicGainSlider are the finder for the Quick Settings mic gain slider.
// The  are identical to the volume slider, but it's located on a different
// page of Quick Settings.
var MicGainSlider = nodewith.Name("Volume").ClassName("Slider").Role(role.Slider)

// MicToggle are the finder for the button that toggles the microphone's mute status.
var MicToggle = nodewith.Role(role.ToggleButton).Attribute("name", regexp.MustCompile("Toggle Mic"))

// ManagedInfoView are the finder for the Quick Settings management information display.
var ManagedInfoView = nodewith.Role(role.Button).ClassName("EnterpriseManagedView")

// BatteryView are the finder for the Quick Settings date/time display.
var BatteryView = nodewith.Role(role.LabelText).ClassName("BatteryView")

// DateView are the finder for the Quick Settings date/time display.
var DateView = nodewith.Role(role.Button).ClassName("DateView")

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
