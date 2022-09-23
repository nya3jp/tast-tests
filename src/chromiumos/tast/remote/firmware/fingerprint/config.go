// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

// Possible fingerprint cros-config enum values can be seen at the following:
// source.chromium.org/chromium/chromiumos/platform2/+/main:chromeos-config/cros_config_host/cros_config_schema.yaml

// FPSensorLoc represents the cros-config fingerprint sensor-location.
type FPSensorLoc string

// Possible values for cros-config fingerprint sensor-location.
const (
	FPSensorLocNone                      FPSensorLoc = "none"
	FPSensorLocPowerButtonTopLeft        FPSensorLoc = "power-button-top-left"
	FPSensorLocKeyboardBottomLeft        FPSensorLoc = "keyboard-bottom-left"
	FPSensorLocKeyboardBottomRight       FPSensorLoc = "keyboard-bottom-right"
	FPSensorLocKeyboardTopRight          FPSensorLoc = "keyboard-top-right"
	FPSensorLocRightSide                 FPSensorLoc = "right-side"
	FPSensorLocLightSide                 FPSensorLoc = "left-side"
	FPSensorLocLeftOfPowerButtonTopRight FPSensorLoc = "left-of-power-button-top-right"
)

// IsValid checks if FPSensorLoc is a valid fingerprint sensor-location.
func (l FPSensorLoc) IsValid() bool {
	switch l {
	case FPSensorLocNone,
		FPSensorLocPowerButtonTopLeft,
		FPSensorLocKeyboardBottomLeft,
		FPSensorLocKeyboardBottomRight,
		FPSensorLocKeyboardTopRight,
		FPSensorLocRightSide,
		FPSensorLocLightSide,
		FPSensorLocLeftOfPowerButtonTopRight:
		return true
	default:
		return false
	}
}

// IsSupported determine if fingerprint is supported on the device based on
// the FPSensorLoc. It allows parsing the "" shortcut for not-defined.
func (l FPSensorLoc) IsSupported() bool {
	if l != FPSensorLoc("") && l != FPSensorLocNone {
		return true
	}
	return false
}

// FPSensorType represents the cros-config fingerprint fingerprint-sensor-type.
type FPSensorType string

// Possible values for cros-config fingerprint fingerprint-sensor-type.
const (
	FPSensorTypeStandAlone    FPSensorType = "stand-alone"
	FPSensorTypeOnPowerButton FPSensorType = "on-power-button"
)

// IsValid checks if FPSensorType is a valid fingerprint sensor-type.
func (t FPSensorType) IsValid() bool {
	switch t {
	case FPSensorTypeStandAlone, FPSensorTypeOnPowerButton:
		return true
	default:
		return false
	}
}
