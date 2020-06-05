// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"strings"
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
	PowerState           StringControl = "power_state"
	V4Role               StringControl = "servo_v4_role"
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

// A PowerStateValue is a string accepted by the PowerState control.
type PowerStateValue string

// These are the string values that can be passed to the PowerState control.
const (
	PowerStateCR50Reset   PowerStateValue = "cr50_reset"
	PowerStateOff         PowerStateValue = "off"
	PowerStateOn          PowerStateValue = "on"
	PowerStateRec         PowerStateValue = "rec"
	PowerStateRecForceMRC PowerStateValue = "rec_force_mrc"
	PowerStateReset       PowerStateValue = "reset"
	PowerStateWarmReset   PowerStateValue = "warm_reset"
)

// A USBMuxState indicates whether the servo's USB mux is on, and if so, which direction it is powering.
type USBMuxState string

// These are the possible states of the USB mux.
const (
	USBMuxOff  USBMuxState = "off"
	USBMuxDUT  USBMuxState = "dut_sees_usbkey"
	USBMuxHost USBMuxState = "servo_sees_usbkey"
)

// A V4RoleValue is a string that would be accepted by the V4Role control.
type V4RoleValue string

// These are the string values that can be passed to V4Role.
const (
	V4RoleSnk V4RoleValue = "snk"
	V4RoleSrc V4RoleValue = "src"
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
	// Servo's Set method returns a bool stating whether the call succeeded or not.
	// This is redundant, because a failed call will return an error anyway.
	// So, we can skip unpacking the output.
	err := s.run(ctx, newCall("set", ActiveChgPort, port))
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
	// Servo's Set method returns a bool stating whether the call succeeded or not.
	// This is redundant, because a failed call will return an error anyway.
	// So, we can skip unpacking the output.
	if err := s.run(ctx, newCall("set", string(control), value)); err != nil {
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

// GetUSBMuxState determines whether the servo USB mux is on, and if so, which direction it is pointed.
func (s *Servo) GetUSBMuxState(ctx context.Context) (USBMuxState, error) {
	pwr, err := s.GetString(ctx, ImageUSBKeyPwr)
	if err != nil {
		return "", err
	}
	if pwr == string(USBMuxOff) {
		return USBMuxOff, nil
	}
	direction, err := s.GetString(ctx, ImageUSBKeyDirection)
	if err != nil {
		return "", err
	}
	if direction != string(USBMuxDUT) && direction != string(USBMuxHost) {
		return "", errors.Errorf("%q has an unknown value: %q", ImageUSBKeyDirection, direction)
	}
	return USBMuxState(direction), nil
}

// SetUSBMuxState switches the servo's USB mux to the specified power/direction state.
func (s *Servo) SetUSBMuxState(ctx context.Context, value USBMuxState) error {
	curr, err := s.GetUSBMuxState(ctx)
	if err != nil {
		return errors.Wrap(err, "getting servo usb state")
	}
	if curr == value {
		return nil
	}
	if value == USBMuxOff {
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
	// Polling until GetUSBMuxState returns USBMuxDUT is not sufficient, because
	// servod will return USBMuxDUT immediately. We need to wait to ensure that the DUT
	// has had time to enumerate the USB.
	// TODO(b/157751281): Clean this up once servo_v3 has been removed.
	if value == USBMuxDUT {
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			return errors.Wrap(err, "sleeping after switching usbkey direction")
		}
	}
	return nil
}

// SetPowerState sets the PowerState control.
// Because this is particularly disruptive, it is always logged.
func (s *Servo) SetPowerState(ctx context.Context, value PowerStateValue) error {
	testing.ContextLogf(ctx, "Setting %q to %q", PowerState, value)
	if value == PowerStateOff {
		// HTTP request is expected to time out while awaiting headers. So we expect an error.
		// So to reduce wait time, call SetString with a short timeout, and ignore the error.
		shortCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := s.SetString(shortCtx, PowerState, string(value)); err == nil {
			return errors.Errorf("expected a timeout error from setting servo %q to %q; got nil", PowerState, value)
		}
		return nil
	}
	return s.SetString(ctx, PowerState, string(value))
}

// SetV4Role sets the V4Role control for a servo v4.
// On a Servo version other than v4, this does nothing.
func (s *Servo) SetV4Role(ctx context.Context, value V4RoleValue) error {
	version, err := s.GetServoVersion(ctx)
	if err != nil {
		return errors.Wrap(err, "getting servo version")
	}
	if !strings.HasPrefix(version, "servo_v4") {
		testing.ContextLogf(ctx, "Skipping setting %q to %q on servo with version %q", V4Role, value, version)
		return nil
	}
	curr, err := s.GetString(ctx, V4Role)
	if err != nil {
		return err
	}
	if s.initialV4Role == "" {
		s.initialV4Role = V4RoleValue(curr)
	}
	if curr == string(value) {
		testing.ContextLogf(ctx, "Skipping setting %q to %q, because that is the current value", V4Role, value)
		return nil
	}
	return s.SetString(ctx, V4Role, string(value))
}
