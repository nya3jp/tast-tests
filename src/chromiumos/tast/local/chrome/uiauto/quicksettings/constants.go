// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"regexp"

	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// RootFinder is the finder to find the Quick Settings area in the UI.
var RootFinder = nodewith.ClassName("UnifiedSystemTrayView")

// SystemTray is to distinguish it from calendar view in cases that use the calendar, it is also the finder to find the Quick Settings area in the UI.
var SystemTray = nodewith.ClassName("UnifiedSystemTray")

// StatusAreaWidget is the finder to find the control widgets.
var StatusAreaWidget = nodewith.Role(role.Pane).HasClass("ash/StatusAreaWidgetDelegate")

// CollapseButton is the finder for the collapse button, which collapses Quick Settings.
var CollapseButton = nodewith.Role(role.Button).ClassName("CollapseButton").Name("Collapse menu")

// ExpandButton is the finder for the expand button, which expands Quick Settings.
var ExpandButton = nodewith.Role(role.Button).ClassName("CollapseButton").Name("Expand menu")

// LockButton is the finder for Quick Settings' lock button.
var LockButton = nodewith.Name("Lock").ClassName("IconButton")

// SettingsButton is the finder for the Quick Settings' setting button.
var SettingsButton = nodewith.Name("Settings").ClassName("IconButton")

// ShutdownButton is the finder for the shutdown button in Quick Settings.
var ShutdownButton = nodewith.Name("Shut down").ClassName("IconButton")

// SignoutButton is the finder for the 'Sign out' Quick Settings button.
var SignoutButton = nodewith.Role(role.Button).Name("Sign out").ClassName("PillButton")

// VPNButton is the finder for the 'VPN' Quick Settings button.
var VPNButton = nodewith.Role(role.Button).Name("VPN").ClassName("PillButton")

// SliderType represents the Quick Settings slider elements.
type SliderType string

// List of descriptive slider names. These don't correspond to any UI node attributes,
// but will be used as keys to map descriptive names to the finders defined below.
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

// BrightnessSlider is the finder for the Quick Settings brightness slider.
var BrightnessSlider = nodewith.Name("Brightness").ClassName("Slider").Role(role.Slider)

// VolumeSlider is the finder for the Quick Settings volume slider.
var VolumeSlider = nodewith.Name("Volume").ClassName("Slider").Role(role.Slider)

// VolumeToggle is the finder for the button that toggles the volume's mute status.
var VolumeToggle = nodewith.Role(role.ToggleButton).NameStartingWith("Toggle Volume")

// MicGainSlider is the finder for the Quick Settings mic gain slider.
// The Finder is identical to the volume slider, but it's located on a different
// page of Quick Settings.
var MicGainSlider = nodewith.Name("Volume").ClassName("Slider").Role(role.Slider)

// MicToggle is the finder for the button that toggles the microphone's mute status.
var MicToggle = nodewith.Role(role.ToggleButton).Attribute("name", regexp.MustCompile("Toggle Mic"))

// ManagedInfoView is the finder for the Quick Settings management information display.
var ManagedInfoView = nodewith.Role(role.Button).ClassName("EnterpriseManagedView")

// BatteryView is the finder for the Quick Settings date/time display.
var BatteryView = nodewith.Role(role.LabelText).ClassName("BatteryLabelView")

// DateView is the finder for the Quick Settings date/time display.
var DateView = nodewith.Role(role.Button).ClassName("DateView")

// SettingPod represents the name of a setting pod in Quick Settings.
// These names are contained in the Name attribute of the automation node
// for the corresponding pod icon button, so they can be used to find the
// buttons in the UI.
type SettingPod string

// List of quick setting names, derived from the corresponding pod icon button node names.
// Character case in the names should exactly match the pod icon button node Name attribute.
const (
	SettingPodAccessibility     SettingPod = "accessibility"
	SettingPodBluetooth         SettingPod = "Bluetooth"
	SettingPodDoNotDisturb      SettingPod = "Do not disturb"
	SettingPodNetwork           SettingPod = "network"
	SettingPodNightLight        SettingPod = "Night Light"
	SettingPodNearbyShare       SettingPod = "Nearby Share"
	SettingPodKeyboard          SettingPod = "keyboard"
	SettingPodScreenCapture     SettingPod = "Screen capture"
	SettingPodVPN               SettingPod = "VPN"
	SettingPodDarkTheme         SettingPod = "Toggle Dark theme"
	SettingPodDarkThemeSettings SettingPod = "dark theme settings"
	SettingPodCast              SettingPod = "cast"
	SettingPodCameraFraming     SettingPod = "Camera framing"
)
