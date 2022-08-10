// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import "strings"

// DeviceCapability is a name of a device capability, matching keys of the
// capabilities map returned from calling PeripheralKit.GetCapabilities().
type DeviceCapability string

const (
	// DeviceCapabilityTransports is the DeviceCapability for supported transport
	// methods.
	DeviceCapabilityTransports DeviceCapability = "CAP_TRANSPORTS"

	// DeviceCapabilityHasPin is the DeviceCapability denoting whether the device
	// has a pin code.
	DeviceCapabilityHasPin DeviceCapability = "CAP_HAS_PIN"

	// DeviceCapabilityInitConnect is the DeviceCapability denoting whether the
	// device can initiate a bluetooth connection.
	DeviceCapabilityInitConnect DeviceCapability = "CAP_INIT_CONNECT"
)

// String returns DeviceCapability as a string.
func (dc DeviceCapability) String() string {
	return string(dc)
}

// TransportMethod refers to a type of transport method that a bluetooth
// device may support.
type TransportMethod string

const (
	// TransportMethodLE refers to the LE TransportMethod.
	TransportMethodLE TransportMethod = "TRANSPORT_LE"

	// TransportMethodBREDR refers to the BREDR TransportMethod.
	TransportMethodBREDR TransportMethod = "TRANSPORT_BREDR"
)

// String returns TransportMethod as a string.
func (tm TransportMethod) String() string {
	return string(tm)
}

// DeviceType refers to the type of bluetooth device, as returned by
// Note that PeripheralKit.GetDeviceType() returns a device type without a
// "BLE_" prefix, but you need the "BLE_" prefix when calling Preipherial
type DeviceType string

const (
	// DeviceTypeKeyboard is the DeviceType for keyboard devices.
	DeviceTypeKeyboard DeviceType = "KEYBOARD"

	// DeviceTypeLEKeyboard is the DeviceType for LE keyboard devices.
	DeviceTypeLEKeyboard DeviceType = "BLE_KEYBOARD"

	// DeviceTypeGamepad is the DeviceType for gamepad devices.
	DeviceTypeGamepad DeviceType = "GAMEPAD"

	// DeviceTypeMouse is the DeviceType for mouse devices.
	DeviceTypeMouse DeviceType = "MOUSE"

	// DeviceTypeLEMouse is the DeviceType for LE mouse devices.
	DeviceTypeLEMouse DeviceType = "BLE_MOUSE"

	// DeviceTypeCombo is the DeviceType for combo devices.
	DeviceTypeCombo DeviceType = "COMBO"

	// DeviceTypeJoystick is the DeviceType for joystick devices.
	DeviceTypeJoystick DeviceType = "JOYSTICK"

	// DeviceTypeA2DPSink is the DeviceType for A2DP sink devices.
	DeviceTypeA2DPSink DeviceType = "A2DP_SINK"

	// DeviceTypePhone is the DeviceType for phone devices.
	DeviceTypePhone DeviceType = "PHONE"

	// DeviceTypeLEPhone is the DeviceType for LE phone devices.
	DeviceTypeLEPhone DeviceType = "BLE_PHONE"

	// DeviceTypeBluetoothAudio is the DeviceType for audio devices.
	DeviceTypeBluetoothAudio DeviceType = "BLUETOOTH_AUDIO"

	// DeviceTypeFastPair is the DeviceType for fast pair devices.
	DeviceTypeFastPair DeviceType = "FAST_PAIR"

	// DeviceTypeLEFastPair is the DeviceType for LE fast pair devices.
	DeviceTypeLEFastPair DeviceType = "BLE_FAST_PAIR"
)

// String returns DeviceType as a string.
func (dt DeviceType) String() string {
	return string(dt)
}

// IsLE returns true if the DeviceType is LE (low-energy).
func (dt DeviceType) IsLE() bool {
	return strings.HasPrefix("BLE_", dt.String())
}

// BaseDeviceType returns the base DeviceType without the LE prefix.
func (dt DeviceType) BaseDeviceType() DeviceType {
	return DeviceType(strings.TrimPrefix("BLE_", dt.String()))
}

// AuthenticationMode refers to the bluetooth authentication mode of a device, as
// returned by calling PeripheralKit.GetAuthenticationMode().
type AuthenticationMode string

const (
	// AuthenticationModeOpen is the "OPEN" AuthenticationMode.
	AuthenticationModeOpen AuthenticationMode = "OPEN"

	// AuthenticationModeSSPKeyboard is the "SSP_KEYBOARD" AuthenticationMode.
	AuthenticationModeSSPKeyboard AuthenticationMode = "SSP_KEYBOARD"

	// AuthenticationModeSSPJustWork is the "SSP_JUST_WORK" AuthenticationMode.
	AuthenticationModeSSPJustWork AuthenticationMode = "SSP_JUST_WORK"

	// AuthenticationModePinCode is the "PIN_CODE" AuthenticationMode.
	AuthenticationModePinCode AuthenticationMode = "PIN_CODE"
)

// String returns AuthenticationMode as a string.
func (am AuthenticationMode) String() string {
	return string(am)
}

// PairingAgentCapability refers to the capability options of the bluez pairing
// agent that resides on the btpeer.
//
// Note: Not to be confused to with bluez agent on the DUT.
type PairingAgentCapability string

const (
	// PairingAgentCapabilityDisplayOnly is the "DisplayOnly"
	// PairingAgentCapability.
	PairingAgentCapabilityDisplayOnly PairingAgentCapability = "DisplayOnly"

	// PairingAgentCapabilityDisplayYesNo is the "DisplayYesNo"
	// PairingAgentCapability.
	PairingAgentCapabilityDisplayYesNo PairingAgentCapability = "DisplayYesNo"

	// PairingAgentCapabilityKeyboardOnly is the "KeyboardOnly"
	// PairingAgentCapability.
	PairingAgentCapabilityKeyboardOnly PairingAgentCapability = "KeyboardOnly"

	// PairingAgentCapabilityNoInputNoOutput is the "NoInputNoOutput"
	// PairingAgentCapability.
	PairingAgentCapabilityNoInputNoOutput PairingAgentCapability = "NoInputNoOutput"

	// PairingAgentCapabilityKeyboardDisplay is the "KeyboardDisplay"
	// PairingAgentCapability.
	PairingAgentCapabilityKeyboardDisplay PairingAgentCapability = "KeyboardDisplay"
)

// String returns PairingAgentCapability as a string.
func (pac PairingAgentCapability) String() string {
	return string(pac)
}
