// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/xmlrpc"
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
	V4Type               StringControl = "servo_v4_type"
	ECBoard              StringControl = "ec_board"
	ECUARTCmd            StringControl = "ec_uart_cmd"
	UARTCmd              StringControl = "servo_v4_uart_cmd"
	WatchdogAdd          StringControl = "watchdog_add"
	WatchdogRemove       StringControl = "watchdog_remove"
)

// An IntControl contains the name of a gettable/settable Control which takes an integer value.
type IntControl string

// These are the Servo controls which can be get/set with an integer value.
const (
	VolumeDownHold   IntControl = "volume_down_hold"    // Integer represents a number of milliseconds.
	VolumeUpHold     IntControl = "volume_up_hold"      // Integer represents a number of milliseconds.
	VolumeUpDownHold IntControl = "volume_up_down_hold" // Integer represents a number of milliseconds.
)

// A FloatControl contains the name of a gettable/settable Control which takes a floating-point value.
type FloatControl string

// These are the Servo controls with floating-point values.
const (
	VBusVoltage FloatControl = "vbus_voltage"
)

// A OnOffControl accepts either "on" or "off" as a value.
type OnOffControl string

// These controls accept only "on" and "off" as values.
const (
	RecMode        OnOffControl = "rec_mode"
	CCDKeepaliveEn OnOffControl = "ccd_keepalive_en"
	DTSMode        OnOffControl = "servo_v4_dts_mode"
)

// An OnOffValue is a string value that would be accepted by an OnOffControl.
type OnOffValue string

// These are the values used by OnOff controls.
const (
	Off OnOffValue = "off"
	On  OnOffValue = "on"
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

// A FWWPStateValue is a string accepted by the FWWPState control.
type FWWPStateValue string

// These are the string values that can be passed to the FWWPState control.
const (
	FWWPStateOff FWWPStateValue = "force_off"
	FWWPStateOn  FWWPStateValue = "force_on"
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

	// V4RoleNA indicates a non-v4 servo.
	V4RoleNA V4RoleValue = "n/a"
)

// A V4TypeValue is a string that would be returned by the V4Type control.
type V4TypeValue string

// These are the string values that can be returned by V4Type.
const (
	V4TypeA V4TypeValue = "type-a"
	V4TypeC V4TypeValue = "type-c"

	// V4TypeNA indicates a non-v4 servo.
	V4TypeNA V4TypeValue = "n/a"
)

// A WatchdogValue is a string that would be accepted by WatchdogAdd & WatchdogRemove control.
type WatchdogValue string

// These are the string watchdog type values that can be passed to WatchdogAdd & WatchdogRemove.
const (
	WatchdogCCD WatchdogValue = "ccd"
)

// ServoKeypressDelay comes from hdctools/servo/drv/keyboard_handlers.py.
// It is the minimum time interval between 'press' and 'release' keyboard events.
const ServoKeypressDelay = 100 * time.Millisecond

// HasControl determines whether the Servo being used supports the given control.
func (s *Servo) HasControl(ctx context.Context, ctrl string) (bool, error) {
	err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("doc", ctrl))
	// If the control exists, doc() should return with no issue.
	if err == nil {
		return true, nil
	}
	// If the control doesn't exist, then doc() should return a fault.
	if _, isFault := err.(xmlrpc.FaultError); isFault {
		return false, nil
	}
	// A non-fault error indicates that something went wrong.
	return false, err
}

// Echo calls the Servo echo method.
func (s *Servo) Echo(ctx context.Context, message string) (string, error) {
	var val string
	err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("echo", message), &val)
	return val, err
}

// PowerNormalPress calls the Servo power_normal_press method.
func (s *Servo) PowerNormalPress(ctx context.Context) (bool, error) {
	var val bool
	err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("power_normal_press"), &val)
	return val, err
}

// SetActChgPort enables a charge port on fluffy.
func (s *Servo) SetActChgPort(ctx context.Context, port string) error {
	return s.SetString(ctx, ActiveChgPort, port)
}

// DUTVoltageMV reads the voltage present on the DUT port on fluffy.
func (s *Servo) DUTVoltageMV(ctx context.Context) (string, error) {
	return s.GetString(ctx, DUTVoltageMV)
}

// GetServoVersion gets the version of Servo being used.
func (s *Servo) GetServoVersion(ctx context.Context) (string, error) {
	if s.version != "" {
		return s.version, nil
	}
	err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("get_version"), &s.version)
	return s.version, err
}

// IsServoV4 determines whether the Servo being used is v4.
func (s *Servo) IsServoV4(ctx context.Context) (bool, error) {
	version, err := s.GetServoVersion(ctx)
	if err != nil {
		return false, errors.Wrap(err, "determining servo version")
	}
	return strings.HasPrefix(version, "servo_v4"), nil
}

// GetServoV4Type gets the version of Servo v4 being used, or V4TypeNA if Servo is not v4.
func (s *Servo) GetServoV4Type(ctx context.Context) (V4TypeValue, error) {
	if s.v4Type != "" {
		return s.v4Type, nil
	}
	if isV4, err := s.IsServoV4(ctx); err != nil {
		return "", errors.Wrap(err, "determining whether servo is v4")
	} else if !isV4 {
		s.v4Type = V4TypeNA
		return s.v4Type, nil
	}
	v4t, err := s.GetString(ctx, V4Type)
	if err != nil {
		return "", err
	}
	s.v4Type = V4TypeValue(v4t)
	return s.v4Type, nil
}

// GetString returns the value of a specified control.
func (s *Servo) GetString(ctx context.Context, control StringControl) (string, error) {
	var value string
	if err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("get", string(control)), &value); err != nil {
		return "", errors.Wrapf(err, "getting value for servo control %q", control)
	}
	return value, nil
}

// SetString sets a Servo control to a string value.
func (s *Servo) SetString(ctx context.Context, control StringControl, value string) error {
	// Servo's Set method returns a bool stating whether the call succeeded or not.
	// This is redundant, because a failed call will return an error anyway.
	// So, we can skip unpacking the output.
	if err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("set", string(control), value)); err != nil {
		return errors.Wrapf(err, "setting servo control %q to %q", control, value)
	}
	return nil
}

// SetInt sets a Servo control to an integer value.
func (s *Servo) SetInt(ctx context.Context, control IntControl, value int) error {
	if err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("set", string(control), value)); err != nil {
		return errors.Wrapf(err, "setting servo control %q to %d", control, value)
	}
	return nil
}

// GetFloat returns the floating-point value of a specified control.
func (s *Servo) GetFloat(ctx context.Context, control FloatControl) (float64, error) {
	var value float64
	if err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("get", string(control)), &value); err != nil {
		return 0, errors.Wrapf(err, "getting value for servo control %q", control)
	}
	return value, nil
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
	shortCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	s.SetString(shortCtx, PowerState, string(value))
	return nil
}

// SetFWWPState sets the FWWPState control.
// Because this is particularly disruptive, it is always logged.
func (s *Servo) SetFWWPState(ctx context.Context, value FWWPStateValue) error {
	testing.ContextLogf(ctx, "Setting %q to %q", FWWPState, value)
	// Don't use SetStringAndCheck because the state can be "on" after we set "force_on".
	shortCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.SetString(shortCtx, FWWPState, string(value))
}

// GetV4Role returns the servo's current V4Role (SNK or SRC), or V4RoleNA if Servo is not V4.
func (s *Servo) GetV4Role(ctx context.Context) (V4RoleValue, error) {
	isV4, err := s.IsServoV4(ctx)
	if err != nil {
		return "", errors.Wrap(err, "determining whether servo is v4")
	}
	if !isV4 {
		return V4RoleNA, nil
	}
	role, err := s.GetString(ctx, V4Role)
	if err != nil {
		return "", err
	}
	return V4RoleValue(role), nil
}

// SetV4Role sets the V4Role control for a servo v4.
// On a Servo version other than v4, this does nothing.
func (s *Servo) SetV4Role(ctx context.Context, newRole V4RoleValue) error {
	// Determine the current V4 Role
	currentRole, err := s.GetV4Role(ctx)
	if err != nil {
		return errors.Wrap(err, "getting current V4 role")
	}

	// Save the initial V4 Role so we can restore it during servo.Close()
	if s.initialV4Role == "" {
		testing.ContextLogf(ctx, "Saving initial V4Role %q for later", currentRole)
		s.initialV4Role = currentRole
	}

	// If not using a servo V4, then we can't set the V4 Role
	if currentRole == V4RoleNA {
		testing.ContextLogf(ctx, "Skipping setting %q to %q on non-v4 servo", V4Role, newRole)
		return nil
	}

	// If the current value is already the intended value,
	// then don't bother resetting.
	if currentRole == newRole {
		testing.ContextLogf(ctx, "Skipping setting %q to %q, because that is the current value", V4Role, newRole)
		return nil
	}

	return s.SetString(ctx, V4Role, string(newRole))
}

// RunECCommand runs the given command on the EC on the device.
func (s *Servo) RunECCommand(ctx context.Context, cmd string) error {
	return s.SetString(ctx, ECUARTCmd, cmd)
}

// SetOnOff sets an OnOffControl setting to the specified value.
func (s *Servo) SetOnOff(ctx context.Context, ctrl OnOffControl, value OnOffValue) error {
	return s.SetString(ctx, StringControl(ctrl), string(value))
}

// ToggleOffOn turns a switch off and on again.
func (s *Servo) ToggleOffOn(ctx context.Context, ctrl OnOffControl) error {
	if err := s.SetString(ctx, StringControl(ctrl), string(Off)); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, ServoKeypressDelay); err != nil {
		return err
	}
	if err := s.SetString(ctx, StringControl(ctrl), string(On)); err != nil {
		return err
	}
	return nil
}

// WatchdogAdd adds the specified watchdog to the servod instance.
func (s *Servo) WatchdogAdd(ctx context.Context, val WatchdogValue) error {
	return s.SetString(ctx, WatchdogAdd, string(val))
}

// WatchdogRemove removes the specified watchdog from the servod instance.
func (s *Servo) WatchdogRemove(ctx context.Context, val WatchdogValue) error {
	return s.SetString(ctx, WatchdogRemove, string(val))
}

// runUARTCommand runs the given command on the servo console.
func (s *Servo) runUARTCommand(ctx context.Context, cmd string) error {
	return s.SetString(ctx, UARTCmd, cmd)
}

// RunUSBCDPConfigCommand executes the "usbc_action dp" command with the specified args on the servo
// console.
func (s *Servo) RunUSBCDPConfigCommand(ctx context.Context, args ...string) error {
	args = append([]string{"usbc_action dp"}, args...)
	cmd := strings.Join(args, " ")
	return s.runUARTCommand(ctx, cmd)
}

// SetCC sets the CC line to the specified value.
func (s *Servo) SetCC(ctx context.Context, val OnOffValue) error {
	cmd := "cc " + string(val)
	return s.runUARTCommand(ctx, cmd)
}
