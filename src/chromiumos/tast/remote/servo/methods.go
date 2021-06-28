// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"strconv"
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
	ActiveDUTController  StringControl = "active_dut_controller"
	DUTVoltageMV         StringControl = "dut_voltage_mv"
	FWWPState            StringControl = "fw_wp_state"
	ImageUSBKeyDirection StringControl = "image_usbkey_direction"
	ImageUSBKeyPwr       StringControl = "image_usbkey_pwr"
	PowerState           StringControl = "power_state"
	Type                 StringControl = "servo_type"
	UARTCmd              StringControl = "servo_v4_uart_cmd"
	Watchdog             StringControl = "watchdog"
	WatchdogAdd          StringControl = "watchdog_add"
	WatchdogRemove       StringControl = "watchdog_remove"

	// DUTConnectionType was previously known as V4Type ("servo_v4_type")
	DUTConnectionType StringControl = "root.dut_connection_type"

	// PDRole was previously known as V4Role ("servo_v4_role")
	PDRole StringControl = "servo_pd_role"
)

// A BoolControl contains the name of a gettable/settable Control which takes a boolean value.
type BoolControl string

// These are the Servo controls which can be get/set with a boolean value.
const (
	ChargerAttached BoolControl = "charger_attached"
)

// An IntControl contains the name of a gettable/settable Control which takes an integer value.
type IntControl string

// These are the Servo controls which can be get/set with an integer value.
const (
	BatteryChargeMAH     IntControl = "battery_charge_mah"
	BatteryFullChargeMAH IntControl = "battery_full_charge_mah"
	VolumeDownHold       IntControl = "volume_down_hold"    // Integer represents a number of milliseconds.
	VolumeUpHold         IntControl = "volume_up_hold"      // Integer represents a number of milliseconds.
	VolumeUpDownHold     IntControl = "volume_up_down_hold" // Integer represents a number of milliseconds.
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
	CCDKeepaliveEn OnOffControl = "ccd_keepalive_en"
	CCDState       OnOffControl = "ccd_state"
	DTSMode        OnOffControl = "servo_dts_mode"
	RecMode        OnOffControl = "rec_mode"
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
	CtrlS        KeypressControl = "ctrl_s"
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
	USBEnter     KeypressControl = "usb_keyboard_enter_key"
)

// A KeypressDuration is a string accepted by a KeypressControl.
type KeypressDuration string

// These are string values that can be passed to a KeypressControl.
const (
	DurTab       KeypressDuration = "tab"
	DurPress     KeypressDuration = "press"
	DurLongPress KeypressDuration = "long_press"
)

// Dur returns a custom duration that can be passed to KeypressWithDuration
func Dur(dur time.Duration) KeypressDuration {
	return KeypressDuration(fmt.Sprintf("%f", dur.Seconds()))
}

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

// A PDRoleValue is a string that would be accepted by the PDRole control.
type PDRoleValue string

// These are the string values that can be passed to PDRole.
const (
	PDRoleSnk PDRoleValue = "snk"
	PDRoleSrc PDRoleValue = "src"

	// PDRoleNA indicates a non-v4 servo.
	PDRoleNA PDRoleValue = "n/a"
)

// A DUTConnTypeValue is a string that would be returned by the DUTConnectionType control.
type DUTConnTypeValue string

// These are the string values that can be returned by DUTConnectionType
const (
	DUTConnTypeA DUTConnTypeValue = "type-a"
	DUTConnTypeC DUTConnTypeValue = "type-c"

	// DUTConnTypeNA indicates a non-v4 servo.
	DUTConnTypeNA DUTConnTypeValue = "n/a"
)

// A WatchdogValue is a string that would be accepted by WatchdogAdd & WatchdogRemove control.
type WatchdogValue string

// These are the string watchdog type values that can be passed to WatchdogAdd & WatchdogRemove.
const (
	WatchdogCCD  WatchdogValue = "ccd"
	WatchdogMain WatchdogValue = "main"
)

// DUTController is the active controller on a dual mode servo.
type DUTController string

// Parameters that can be passed to SetActiveDUTController().
const (
	DUTControllerC2D2       DUTController = "c2d2"
	DUTControllerCCD        DUTController = "ccd_cr50"
	DUTControllerServoMicro DUTController = "servo_micro"
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

// GetDUTConnectionType gets the type of connection between the Servo and the DUT.
// If Servo is not V4, returns DUTConnTypeNA.
func (s *Servo) GetDUTConnectionType(ctx context.Context) (DUTConnTypeValue, error) {
	if s.dutConnType != "" {
		return s.dutConnType, nil
	}
	if isV4, err := s.IsServoV4(ctx); err != nil {
		return "", errors.Wrap(err, "determining whether servo is v4")
	} else if !isV4 {
		s.dutConnType = DUTConnTypeNA
		return s.dutConnType, nil
	}
	connType, err := s.GetString(ctx, DUTConnectionType)
	if err != nil {
		return "", err
	}
	s.dutConnType = DUTConnTypeValue(connType)
	return s.dutConnType, nil
}

// GetString returns the value of a specified control.
func (s *Servo) GetString(ctx context.Context, control StringControl) (string, error) {
	var value string
	if err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("get", string(control)), &value); err != nil {
		return "", errors.Wrapf(err, "getting value for servo control %q", control)
	}
	return value, nil
}

// GetServoSerials returns a map of servo serial numbers. Interesting map keys are "ccd", "main", "servo_micro", but there are others also.
func (s *Servo) GetServoSerials(ctx context.Context) (map[string]string, error) {
	value := make(map[string]string)
	if err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("get_servo_serials"), &value); err != nil {
		return map[string]string{}, errors.Wrap(err, "getting servo serials")
	}
	return value, nil
}

// GetCCDSerial returns the serial number of the CCD interface.
func (s *Servo) GetCCDSerial(ctx context.Context) (string, error) {
	value, err := s.GetServoSerials(ctx)
	if err != nil {
		return "", err
	}
	ccdSerial, ok := value["ccd"]
	if ok {
		return ccdSerial, nil
	}
	servoType, err := s.GetServoType(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get servo type")
	}
	// SuzyQ reports as ccd_cr50, and doesn't have an interface named ccd.
	if servoType == "ccd_cr50" {
		ccdSerial, ok := value["main"]
		if ok {
			return ccdSerial, nil
		}
	}
	return "", errors.Errorf("no ccd serial in %q", value)
}

// GetBool returns the boolean value of a specified control.
func (s *Servo) GetBool(ctx context.Context, control BoolControl) (bool, error) {
	var value bool
	if err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("get", string(control)), &value); err != nil {
		return false, errors.Wrapf(err, "getting value for servo control %q", control)
	}
	return value, nil
}

// GetChargerAttached returns the boolean value to indicate whether charger is attached.
func (s *Servo) GetChargerAttached(ctx context.Context) (bool, error) {
	return s.GetBool(ctx, ChargerAttached)
}

// parseUint extracts a hex number from `value` at `*index+1` that is exactly `bits` in length.
// `bits` must be power of 2.
// `*index` will be moved to the end of the extracted runes.
func parseUint(value []rune, index *int, bits int) (rune, error) {
	chars := bits / 4
	endIndex := *index + chars
	if endIndex >= len(value) {
		return 0, errors.Errorf("unparsable escape sequence `\\%s`", string(value[*index:]))
	}
	char, err := strconv.ParseUint(string(value[*index+1:endIndex+1]), 16, bits)
	if err != nil {
		return 0, errors.Wrapf(err, "unparsable escape sequence `\\%s`", string(value[*index:endIndex+1]))
	}
	*index += chars
	return rune(char), nil
}

// parseQuotedStringInternal returns a new string with the quotes and escaped chars from `value` removed, moves `*index` to the index of the closing quote rune.
func parseQuotedStringInternal(value []rune, index *int) (string, error) {
	if *index >= len(value) {
		return "", errors.Errorf("unexpected end of string at %d in %s", *index, string(value))
	}
	// The first char should always be a ' or "
	quoteChar := value[*index]
	if quoteChar != '\'' && quoteChar != '"' {
		return "", errors.Errorf("unexpected string char %c at index %d in %s", quoteChar, *index, string(value))
	}
	(*index)++
	var current strings.Builder
	for ; *index < len(value); (*index)++ {
		c := value[*index]
		if c == quoteChar {
			break
		} else if c == '\\' {
			(*index)++
			if *index >= len(value) {
				return "", errors.New("unparsable escape sequence \\")
			}
			switch value[*index] {
			case '"', '\'', '\\':
				current.WriteRune(value[*index])
			case 'r':
				current.WriteRune('\r')
			case 'n':
				current.WriteRune('\n')
			case 't':
				current.WriteRune('\t')
			case 'x':
				r, err := parseUint(value, index, 8)
				if err != nil {
					return "", err
				}
				current.WriteRune(r)
			case 'u':
				r, err := parseUint(value, index, 16)
				if err != nil {
					return "", err
				}
				current.WriteRune(r)
			case 'U':
				r, err := parseUint(value, index, 32)
				if err != nil {
					return "", err
				}
				current.WriteRune(r)
			default:
				return "", errors.Errorf("unexpected escape sequence \\%c at index %d in %s", value[*index], *index, string(value))
			}
		} else {
			current.WriteRune(c)
		}
	}
	return current.String(), nil
}

// parseStringListInternal parses `value` as a possibly nested list of strings, each quoted and separated by commas. Moves `*index` to the index of the closing ] rune.
func parseStringListInternal(value []rune, index *int) ([]interface{}, error) {
	var result []interface{}
	if *index >= len(value) {
		return nil, errors.Errorf("unexpected end of string at %d in %s", *index, string(value))
	}
	// The first char should always be a [
	if value[*index] != '[' {
		return nil, errors.Errorf("unexpected list char %c at index %d in %s", value[*index], *index, string(value))
	}
	(*index)++
	for ; *index < len(value); (*index)++ {
		c := value[*index]
		switch c {
		case '[':
			sublist, err := parseStringListInternal(value, index)
			if err != nil {
				return nil, err
			}
			result = append(result, sublist)
		case '\'', '"':
			substr, err := parseQuotedStringInternal(value, index)
			if err != nil {
				return nil, err
			}
			result = append(result, substr)
		case ',', ' ':
			// Ignore this char
		case ']':
			return result, nil
		default:
			return nil, errors.Errorf("unexpected list char %c at index %d in %s", c, *index, string(value))
		}
	}
	return nil, errors.Errorf("unexpected end of string at %d in %s", *index, string(value))
}

// ParseStringList parses `value` as a possibly nested list of strings, each quoted and separated by commas.
func ParseStringList(value string) ([]interface{}, error) {
	index := 0
	return parseStringListInternal([]rune(value), &index)
}

// ParseQuotedString returns a new string with the quotes and escaped chars from `value` removed.
func ParseQuotedString(value string) (string, error) {
	index := 0
	return parseQuotedStringInternal([]rune(value), &index)
}

// GetStringList parses the value of a control as an encoded list
func (s *Servo) GetStringList(ctx context.Context, control StringControl) ([]interface{}, error) {
	v, err := s.GetString(ctx, control)
	if err != nil {
		return nil, err
	}
	return ParseStringList(v)
}

// GetQuotedString parses the value of a control as a quoted string
func (s *Servo) GetQuotedString(ctx context.Context, control StringControl) (string, error) {
	v, err := s.GetString(ctx, control)
	if err != nil {
		return "", err
	}
	return ParseQuotedString(v)
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

// SetStringTimeout sets a Servo control to a string value.
func (s *Servo) SetStringTimeout(ctx context.Context, control StringControl, value string, timeout time.Duration) error {
	// Servo's Set method returns a bool stating whether the call succeeded or not.
	// This is redundant, because a failed call will return an error anyway.
	// So, we can skip unpacking the output.
	if err := s.xmlrpc.Run(ctx, xmlrpc.NewCallTimeout("set", timeout, string(control), value)); err != nil {
		return errors.Wrapf(err, "setting servo control %q to %q", control, value)
	}
	return nil
}

// SetStringList sets a Servo control to a list of string values.
func (s *Servo) SetStringList(ctx context.Context, control StringControl, values []string) error {
	value := "["
	for i, part := range values {
		if i > 0 {
			value += ", "
		}

		// Escape \ and '
		part = strings.ReplaceAll(part, `\`, `\\`)
		part = strings.ReplaceAll(part, `'`, `\'`)

		// Surround by '
		value += "'" + part + "'"
	}
	value += "]"
	return s.SetString(ctx, control, value)
}

// SetInt sets a Servo control to an integer value.
func (s *Servo) SetInt(ctx context.Context, control IntControl, value int) error {
	if err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("set", string(control), value)); err != nil {
		return errors.Wrapf(err, "setting servo control %q to %d", control, value)
	}
	return nil
}

// GetInt returns the integer value of a specified control.
func (s *Servo) GetInt(ctx context.Context, control IntControl) (int, error) {
	var value int
	if err := s.xmlrpc.Run(ctx, xmlrpc.NewCall("get", string(control)), &value); err != nil {
		return 0, errors.Wrapf(err, "getting value for servo control %q", control)
	}
	return value, nil
}

// GetBatteryChargeMAH returns the battery's charge in mAh.
func (s *Servo) GetBatteryChargeMAH(ctx context.Context) (int, error) {
	return s.GetInt(ctx, BatteryChargeMAH)
}

// GetBatteryFullChargeMAH returns the battery's last full charge in mAh.
func (s *Servo) GetBatteryFullChargeMAH(ctx context.Context) (int, error) {
	return s.GetInt(ctx, BatteryFullChargeMAH)
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
	if err := s.SetStringTimeout(ctx, ImageUSBKeyDirection, string(value), 90*time.Second); err != nil {
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
// It can be slow, because some boards are configured to hold down the power button for 12 seconds.
func (s *Servo) SetPowerState(ctx context.Context, value PowerStateValue) error {
	return s.SetStringTimeout(ctx, PowerState, string(value), 20*time.Second)
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

// GetPDRole returns the servo's current PDRole (SNK or SRC), or PDRoleNA if Servo is not V4.
func (s *Servo) GetPDRole(ctx context.Context) (PDRoleValue, error) {
	isV4, err := s.IsServoV4(ctx)
	if err != nil {
		return "", errors.Wrap(err, "determining whether servo is v4")
	}
	if !isV4 {
		return PDRoleNA, nil
	}
	role, err := s.GetString(ctx, PDRole)
	if err != nil {
		return "", err
	}
	return PDRoleValue(role), nil
}

// SetPDRole sets the PDRole control for a servo v4.
// On a Servo version other than v4, this does nothing.
func (s *Servo) SetPDRole(ctx context.Context, newRole PDRoleValue) error {
	// Determine the current PD role
	currentRole, err := s.GetPDRole(ctx)
	if err != nil {
		return errors.Wrap(err, "getting current PD role")
	}

	// Save the initial PD role so we can restore it during servo.Close()
	if s.initialPDRole == "" {
		testing.ContextLogf(ctx, "Saving initial PDRole %q for later", currentRole)
		s.initialPDRole = currentRole
	}

	// If not using a servo V4, then we can't set the PD Role
	if currentRole == PDRoleNA {
		testing.ContextLogf(ctx, "Skipping setting %q to %q on non-v4 servo", PDRole, newRole)
		return nil
	}

	// If the current value is already the intended value,
	// then don't bother resetting.
	if currentRole == newRole {
		testing.ContextLogf(ctx, "Skipping setting %q to %q, because that is the current value", PDRole, newRole)
		return nil
	}

	return s.SetString(ctx, PDRole, string(newRole))
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

// ToggleOnOff turns a switch on and off again.
func (s *Servo) ToggleOnOff(ctx context.Context, ctrl OnOffControl) error {
	if err := s.SetString(ctx, StringControl(ctrl), string(On)); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, ServoKeypressDelay); err != nil {
		return err
	}
	if err := s.SetString(ctx, StringControl(ctrl), string(Off)); err != nil {
		return err
	}
	return nil
}

// GetOnOff gets an OnOffControl as a bool.
func (s *Servo) GetOnOff(ctx context.Context, ctrl OnOffControl) (bool, error) {
	str, err := s.GetString(ctx, StringControl(ctrl))
	if err != nil {
		return false, err
	}
	switch str {
	case string(On):
		return true, nil
	case string(Off):
		return false, nil
	}
	return false, errors.Errorf("cannot convert %q to boolean", str)
}

// WatchdogAdd adds the specified watchdog to the servod instance.
func (s *Servo) WatchdogAdd(ctx context.Context, val WatchdogValue) error {
	return s.SetString(ctx, WatchdogAdd, string(val))
}

// WatchdogRemove removes the specified watchdog from the servod instance.
// Servo.Close() will restore the watchdog.
func (s *Servo) WatchdogRemove(ctx context.Context, val WatchdogValue) error {
	if err := s.SetString(ctx, WatchdogRemove, string(val)); err != nil {
		return err
	}
	s.removedWatchdogs = append(s.removedWatchdogs, val)
	return nil
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

// SetActiveDUTController sets the active controller on a dual mode v4 servo
func (s *Servo) SetActiveDUTController(ctx context.Context, adc DUTController) error {
	return s.SetString(ctx, ActiveDUTController, string(adc))
}

// GetServoType gets the type of the servo.
func (s *Servo) GetServoType(ctx context.Context) (string, error) {
	if s.servoType != "" {
		return s.servoType, nil
	}
	servoType, err := s.GetString(ctx, Type)
	if err != nil {
		return "", err
	}
	hasCCD := strings.Contains(servoType, string(DUTControllerCCD))
	if !hasCCD {
		if hasCCDState, err := s.HasControl(ctx, string(CCDState)); err != nil {
			return "", errors.Wrap(err, "failed to check ccd_state control")
		} else if hasCCDState {
			ccdState, err := s.GetOnOff(ctx, CCDState)
			if err != nil {
				return "", errors.Wrap(err, "failed to get ccd_state")
			}
			hasCCD = ccdState
		}
	}
	hasServoMicro := strings.Contains(servoType, string(DUTControllerServoMicro))
	hasC2D2 := strings.Contains(servoType, string(DUTControllerC2D2))
	isDualV4 := strings.Contains(servoType, "_and_")

	if !hasCCD && !hasServoMicro && !hasC2D2 {
		testing.ContextLogf(ctx, "Assuming %s is equivalent to servo_micro", servoType)
		hasServoMicro = true
	}
	s.servoType = servoType
	s.hasCCD = hasCCD
	s.hasServoMicro = hasServoMicro
	s.hasC2D2 = hasC2D2
	s.isDualV4 = isDualV4
	return s.servoType, nil
}

// RequireCCD verifies that the servo has a CCD connection, and switches to it for dual v4 servos.
func (s *Servo) RequireCCD(ctx context.Context) error {
	servoType, err := s.GetServoType(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get servo type")
	}

	if !s.hasCCD {
		return errors.Wrapf(err, "servo %s is not CCD", servoType)
	}
	if s.isDualV4 {
		if err = s.SetActiveDUTController(ctx, DUTControllerCCD); err != nil {
			return errors.Wrap(err, "failed to set active dut controller")
		}
	}
	return nil
}

// PreferDebugHeader switches to the servo_micro or C2D2 for dual v4 servos, but doesn't fail on CCD only servos.
// Returns true if the servo has a debug header connection, false if it only has CCD.
func (s *Servo) PreferDebugHeader(ctx context.Context) (bool, error) {
	_, err := s.GetServoType(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get servo type")
	}
	if s.isDualV4 {
		if s.hasServoMicro {
			if err = s.SetActiveDUTController(ctx, DUTControllerServoMicro); err != nil {
				return false, errors.Wrap(err, "failed to set active dut controller")
			}
			return true, nil
		} else if s.hasC2D2 {
			if err = s.SetActiveDUTController(ctx, DUTControllerC2D2); err != nil {
				return false, errors.Wrap(err, "failed to set active dut controller")
			}
			return true, nil
		}
	}
	return s.hasServoMicro || s.hasC2D2, nil
}

// RequireDebugHeader verifies that the servo has a servo_micro or C2D2 connection, and switches to it for dual v4 servos.
func (s *Servo) RequireDebugHeader(ctx context.Context) error {
	servoType, err := s.GetServoType(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get servo type")
	}
	if !s.hasServoMicro && !s.hasC2D2 {
		return errors.Wrapf(err, "servo %s doesn't have debug header", servoType)
	}
	if s.isDualV4 {
		if s.hasServoMicro {
			if err = s.SetActiveDUTController(ctx, DUTControllerServoMicro); err != nil {
				return errors.Wrap(err, "failed to set active dut controller")
			}
		} else if s.hasC2D2 {
			if err = s.SetActiveDUTController(ctx, DUTControllerC2D2); err != nil {
				return errors.Wrap(err, "failed to set active dut controller")
			}
		}
	}
	return nil
}
