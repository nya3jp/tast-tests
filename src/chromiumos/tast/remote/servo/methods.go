// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// A StringControl contains the name of a gettable/settable Control which takes a string value.
type StringControl string

// These are the Servo controls which can be get/set with a string value.
const (
	ActiveChgPort        StringControl = "active_chg_port"
	DUTVoltageMV         StringControl = "dut_voltage_mv"
	FWWPState            StringControl = "fw_wp_state"
	ImageUSBKeyDirection StringControl = "image_usbkey_direction"
	ImageUSBKeyPwr       StringControl = "image_usbkey_pwr"
)

// A KeypressControl is a special type of Control which can take either a numerical value or a KeypressDuration.
type KeypressControl StringControl

// These are the Servo controls which can be set with either a numerical value or a KeypressDuration.
const (
	CtrlD        KeypressControl = "ctrl_d"
	CtrlU        KeypressControl = "ctrl_u"
	CtrlEnter    KeypressControl = "ctrl_enter"
	Ctrl         KeypressControl = "ctrl_key"
	Enter        KeypressControl = "enter_key"
	Refresh      KeypressControl = "refresh_key"
	CtrlRefresh  KeypressControl = "ctrl_refresh_key"
	ImaginaryKey KeypressControl = "imaginary_key"
	SysRQX       KeypressControl = "sysrq_x"
	PowerKey     KeypressControl = "power_key"
	Pwrbutton    KeypressControl = "pwr_button"
)

// A KeypressDuration is a string accepted by a KeypressControl.
type KeypressDuration string

// These are string values that can be passed to a KeypressControl.
const (
	DurTab       KeypressDuration = "tab"
	DurPress     KeypressDuration = "press"
	DurLongPress KeypressDuration = "long_press"
)

// A USBKeyState indicates whether the servo's USB mux is on, and if so, which direction it is powering.
type USBKeyState string

// These are the possible states of the USB mux.
const (
	USBKeyDUT  USBKeyState = "dut_sees_usbkey"
	USBKeyOff  USBKeyState = "off"
	USBKeyHost USBKeyState = "servo_sees_usbkey"
)

// Echo calls the Servo echo method.
func (s *Servo) Echo(ctx context.Context, message string) (string, error) {
	var val string
	err := s.run(ctx, newCall("echo", message), &val)
	return val, err
}

// PowerNormalPress calls the Servo power_normal_press method.
func (s *Servo) PowerNormalPress(ctx context.Context) (bool, error) {
	var val bool
	err := s.run(ctx, newCall("power_normal_press"), &val)
	return val, err
}

// SetActChgPort enables a charge port on fluffy.
func (s *Servo) SetActChgPort(ctx context.Context, port string) error {
	// This is a little strange...  The run() method needs the arg(val) as the response contains whether the
	// xml-rpc call succeeded or not.  Omitting this variable causes it to fail to unmarshall the xmlrpc
	// response.  However it seems strange because if the call didn't succeed, I believe run() will return an
	// error in err anyways so the variable seems useless.  This is why I'm ignoring val and only returning err.
	var val bool
	err := s.run(ctx, newCall("set", ActiveChgPort, port), &val)
	return err
}

// DUTVoltageMV reads the voltage present on the DUT port on fluffy.
func (s *Servo) DUTVoltageMV(ctx context.Context) (string, error) {
	var voltageMV string
	err := s.run(ctx, newCall("get", DUTVoltageMV), &voltageMV)
	return voltageMV, err
}

// GetServoVersion gets the version of Servo being used.
func (s *Servo) GetServoVersion(ctx context.Context) (string, error) {
	var version string
	err := s.run(ctx, newCall("get_version"), &version)
	return version, err
}

// GetString returns the value of a specified control.
func (s *Servo) GetString(ctx context.Context, control StringControl) (string, error) {
	var value string
	if err := s.run(ctx, newCall("get", string(control)), &value); err != nil {
		return "", errors.Wrapf(err, "getting value for servo control %q", control)
	}
	return value, nil
}

// SetString sets a specified control to a specified value.
func (s *Servo) SetString(ctx context.Context, control StringControl, value string) error {
	// The run() method needs the arg(val), as the response contains whether the xml-rpc call
	// succeeded or not. Omitting this variable causes it to fail to unmarshall the xmlrpc response.
	// However, reading this value is unnecessary, because if the call fails
	// then run() returns an error anyway.
	var val bool
	if err := s.run(ctx, newCall("set", string(control), value), &val); err != nil {
		return errors.Wrapf(err, "setting servo control %q to %q", control, value)
	}
	return nil
}

// SetStringAndCheck sets a control to a specified value, and then verifies that it was set correctly.
func (s *Servo) SetStringAndCheck(ctx context.Context, control StringControl, value string) error {
	if err := s.SetString(ctx, control, value); err != nil {
		return err
	}
	if checkedValue, err := s.GetString(ctx, control); err != nil {
		return err
	} else if checkedValue != value {
		return errors.Errorf("after attempting to set %s to %s, checked value was %s", control, value, checkedValue)
	}
	return nil
}

// KeypressWithDuration sets a KeypressControl to a KeypressDuration value.
func (s *Servo) KeypressWithDuration(ctx context.Context, control KeypressControl, value KeypressDuration) error {
	return s.SetString(ctx, StringControl(control), string(value))
}

// GetUSBKeyState determines whether the servo USB mux is on, and if so, which direction it is pointed.
func (s *Servo) GetUSBKeyState(ctx context.Context) (USBKeyState, error) {
	pwr, err := s.GetString(ctx, ImageUSBKeyPwr)
	if err != nil {
		return "", err
	}
	if pwr == string(USBKeyOff) {
		return USBKeyOff, nil
	}
	direction, err := s.GetString(ctx, ImageUSBKeyDirection)
	if err != nil {
		return "", err
	}
	if direction != string(USBKeyDUT) && direction != string(USBKeyHost) {
		return "", errors.Errorf("%q has an unknown value: %q", ImageUSBKeyDirection, direction)
	}
	return USBKeyState(direction), nil
}

// SetUSBKeyState switches the servo's USB mux to the specified power/direction state.
func (s *Servo) SetUSBKeyState(ctx context.Context, value USBKeyState) error {
	curr, err := s.GetUSBKeyState(ctx)
	if err != nil {
		return errors.Wrap(err, "getting servo usb state")
	}
	if curr == value {
		return nil
	}
	if value == USBKeyOff {
		return s.SetString(ctx, ImageUSBKeyPwr, string(value))
	}
	// Servod ensures the following:
	// * The port is power cycled if it is changing directions
	// * The port ends up in a powered state after this call
	// * If facing the host side, the call only returns once a USB device is detected, or after a generous timeout (10s)
	if err := s.SetString(ctx, ImageUSBKeyDirection, string(value)); err != nil {
		return err
	}
	// Because servod makes no guarantees when switching to the DUT side,
	// add a detection delay here when facing the DUT.
	// Polling until GetUSBKeyState returns USBKeyDUT is not sufficient, because
	// servod will return USBKeyDUT immediately. Hence, use testing.Sleep.
	if value == USBKeyDUT {
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			return errors.Wrap(err, "sleeping after switching usbkey direction")
		}
	}
	return nil
}
