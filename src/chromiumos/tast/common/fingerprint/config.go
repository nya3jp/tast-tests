// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

// Possible fingerprint cros-config enum values can be seen at the following:
// crsrc.org/o/src/platform2/chromeos-config/cros_config_host/cros_config_schema.yaml?q=content:%22fingerprint:%22

// BoardName is the board name of the FPMCU. This is also the cros-config
// fingerprint board value.
type BoardName string

// Possible names for FPMCUs.
const (
	BoardNameBloonchipper BoardName = "bloonchipper"
	BoardNameDartmonkey   BoardName = "dartmonkey"
	BoardNameNocturne     BoardName = "nocturne_fp"
	BoardNameNami         BoardName = "nami_fp"
)

// IsValid checks if the BoardName is a valid fingerprint board name.
func (b BoardName) IsValid() bool {
	switch b {
	case BoardNameBloonchipper,
		BoardNameDartmonkey,
		BoardNameNocturne,
		BoardNameNami:
		return true
	default:
		return false
	}
}

// SensorLoc represents the cros-config fingerprint sensor-location.
type SensorLoc string

// Possible values for cros-config fingerprint sensor-location.
//
// See the following for valid fields:
// crsrc.org/o/src/platform2/chromeos-config/cros_config_host/cros_config_schema.yaml?q=content:%22sensor-location:%22
const (
	SensorLocNone                      SensorLoc = "none"
	SensorLocPowerButtonTopLeft        SensorLoc = "power-button-top-left"
	SensorLocKeyboardBottomLeft        SensorLoc = "keyboard-bottom-left"
	SensorLocKeyboardBottomRight       SensorLoc = "keyboard-bottom-right"
	SensorLocKeyboardTopRight          SensorLoc = "keyboard-top-right"
	SensorLocRightSide                 SensorLoc = "right-side"
	SensorLocLightSide                 SensorLoc = "left-side"
	SensorLocLeftOfPowerButtonTopRight SensorLoc = "left-of-power-button-top-right"
)

// IsValid checks if SensorLoc is a valid fingerprint sensor-location.
func (l SensorLoc) IsValid() bool {
	switch l {
	case SensorLocNone,
		SensorLocPowerButtonTopLeft,
		SensorLocKeyboardBottomLeft,
		SensorLocKeyboardBottomRight,
		SensorLocKeyboardTopRight,
		SensorLocRightSide,
		SensorLocLightSide,
		SensorLocLeftOfPowerButtonTopRight:
		return true
	default:
		return false
	}
}

// IsSupported determine if fingerprint is supported on the device based on
// the SensorLoc. It allows parsing the "" shortcut for not-defined.
func (l SensorLoc) IsSupported() bool {
	if l != SensorLoc("") && l != SensorLocNone {
		return true
	}
	return false
}

// SensorType represents the cros-config fingerprint fingerprint-sensor-type.
type SensorType string

// Possible values for cros-config fingerprint fingerprint-sensor-type.
//
// See the following for valid fields:
// crsrc.org/o/src/platform2/chromeos-config/cros_config_host/cros_config_schema.yaml?q=content:%22fingerprint-sensor-type:%22
const (
	SensorTypeStandAlone    SensorType = "stand-alone"
	SensorTypeOnPowerButton SensorType = "on-power-button"
)

// IsValid checks if SensorType is a valid fingerprint sensor-type.
func (t SensorType) IsValid() bool {
	switch t {
	case SensorTypeStandAlone, SensorTypeOnPowerButton:
		return true
	default:
		return false
	}
}
