// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"chromiumos/tast/errors"
)

// A ServoControl contains the name of a gettable/settable control.
type servoControl string

// These are the Servo controls which can be get/set.
const (
	CtrlActiveChgPort servoControl = "active_chg_port"
	CtrlDutVoltageMv  servoControl = "dut_voltage_mv"
	CtrlFwWpState     servoControl = "fw_wp_state"
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
	err := s.run(ctx, newCall("set", CtrlActiveChgPort, port), &val)
	return err
}

// DutVoltageMV reads the voltage present on the DUT port on fluffy.
func (s *Servo) DutVoltageMV(ctx context.Context) (string, error) {
	var voltageMV string
	err := s.run(ctx, newCall("get", CtrlDutVoltageMv), &voltageMV)
	return voltageMV, err
}

// Get returns the value of a specified control.
func (s *Servo) Get(ctx context.Context, control servoControl) (string, error) {
	var value string
	err := s.run(ctx, newCall("get", string(control)), &value)
	return value, err
}

// Set sets a specified control to a specified value.
func (s *Servo) Set(ctx context.Context, control servoControl, value string) error {
	var resp bool
	err := s.run(ctx, newCall("set", string(control), value), &resp)
	return err
}

// SetAndCheck sets a control to a specified value, and then verifies that it was set correctly.
func (s *Servo) SetAndCheck(ctx context.Context, control servoControl, value string) error {
	if err := s.Set(ctx, control, value); err != nil {
		return errors.Wrapf(err, "failed to set %s to %s", control, value)
	}
	if checkedValue, err := s.Get(ctx, control); err != nil {
		return errors.Wrapf(err, "failed to check %s", control)
	} else if checkedValue != value {
		return errors.Errorf("after attempting to set %s to %s, checked value was %s", control, value, checkedValue)
	}
	return nil
}
