// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package useractions

// Attribute keys used to represent DUT environment.
const (
	AttributeDeviceMode   string = "DeviceMode"
	AttributeKeyboardType string = "KeyboardType"
	AttributeBoardName    string = "BoardName"
	AttributeUserMode     string = "UserMode"
	AttributeInputMethod  string = "InputMethod"
	AttributeInputField   string = "InputField"
	AttributeFloatVK      string = "FloatVK"
)

// Available attribute values of device mode.
const (
	DeviceModeClamshell string = "Clamshell"
	DeviceModeTablet    string = "Tablet"
	DeviceModeUnknown   string = "Unknown"
)

// Available attribute values of keyboard type.
const (
	KeyboardTypePhysicalKeyboard string = "Physical Keyboard"
	KeyboardTypeTabletVK         string = "Tablet Virtual Keyboard"
	KeyboardTypeA11yVK           string = "A11y Virtual Keyboard"
	KeyboardTypeUnknown          string = "Unknown"
)

// Available attribute values of user mode.
const (
	UserModeConsumer   string = "Consumer"
	UserModeEnterprise string = "Enterprise"
	UserModeGuest      string = "Guest"
	UserModeIncognito  string = "Incognito"
)
