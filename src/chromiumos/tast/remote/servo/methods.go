// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"chromiumos/tast/errors"
)

// A StringControl contains the name of a gettable/settable Control which takes a string value.
type StringControl string

// These are the Servo controls which can be get/set.
const (
	CtrlActiveChgPort StringControl = "active_chg_port"
	CtrlDUTVoltageMV  StringControl = "dut_voltage_mv"
	CtrlFWWPState     StringControl = "fw_wp_state"
)

// Echo calls the Servo echo method.
func (s *Servo) Echo(ctx context.Context, message string) (string, error) {
	var val string
	err := s.run(ctx, newCall("echo", message), &val)
	return val, err
}

// PowerNormalPress calls servod's (hdctools repo) power_normal_press method.
func (s *Servo) PowerNormalPress(ctx context.Context) (bool, error) {
	var val bool
	err := s.run(ctx, newCall("power_normal_press"), &val)
	return val, err
}

// CtrlD calls servod's (hdctools repo) ctrl_d method.
func (s *Servo) CtrlD(ctx context.Context) (bool, error) {
	var val bool
	err := s.run(ctx, newCall("ctrl_d"), &val)
	return val, err
}

// EnterKey calls servod's (hdctools repo) enter_key method.
func (s *Servo) EnterKey(ctx context.Context) (bool, error) {
	var val bool
	err := s.run(ctx, newCall("enter_key"), &val)
	return val, err
}

// SetActChgPort enables a charge port on fluffy.
func (s *Servo) SetActChgPort(ctx context.Context, port string) error {
	// This is a little strange...  The run() method needs the arg(val) as the response contains whether the
	// xml-rpc call succeeded or not.  Omitting this variable causes it to fail to unmarshall the xmlrpc
	// response.  However it seems strange because if the call didn't succeed, I believe run() will return an
	// error in err anyways so the variable seems useless.  This is why I'm ignoring val and only returning err.
	var val bool
	err := s.run(ctx, newCall("set", CtrlActiveChgPort, port), &val)
	return err
}

// DUTVoltageMV reads the voltage present on the DUT port on fluffy.
func (s *Servo) DUTVoltageMV(ctx context.Context) (string, error) {
	var voltageMV string
	err := s.run(ctx, newCall("get", CtrlDUTVoltageMV), &voltageMV)
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
	err := s.run(ctx, newCall("get", string(control)), &value)
	return value, err
}

// SetString sets a specified control to a specified value.
func (s *Servo) SetString(ctx context.Context, control StringControl, value string) error {
	// The run() method needs the arg(val), as the response contains whether the xml-rpc call
	// succeeded or not. Omitting this variable causes it to fail to unmarshall the xmlrpc response.
	// However, reading this value is unnecessary, because if the call fails
	// then run() returns an error anyway.
	var val bool
	err := s.run(ctx, newCall("set", string(control), value), &val)
	return err
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
