// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"chromiumos/tast/errors"
)

const (
	// Host to switching USB to host machine
	Host = "servo_sees_usbkey"
	// DUT to switching USB to DUT
	DUT = "dut_sees_usbkey"
	// PowerOn to turn on USB_KEY
	PowerOn = "on"
	// PowerOff to turn off USB_KEY
	PowerOff = "off"
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

// SetCheck calls Servo get and set functions.
// First will check the default value then proceeds with setting value.
func (s *Servo) SetCheck(ctx context.Context, name, value string) error {
	// Checking the value is set already.
	controlValue, err := s.Get(ctx, name)
	if err != nil {
		return err
	}
	if value == controlValue {
		return nil
	}
	// Setting the value if not set.
	if err := s.SetNoCheck(ctx, name, value); err != nil {
		return err
	}

	// Verifying the value is set correctly.
	controlValue, err = s.Get(ctx, name)
	if err != nil {
		return err
	}
	if value != controlValue {
		return errors.Wrapf(err, "failed to set %q -> %q", name, value)
	}
	return nil

}

// SetNoCheck calls Servo set function.
// Method sets the vlaue without any check.
func (s *Servo) SetNoCheck(ctx context.Context, name, value string) error {
	var val interface{}
	if err := s.run(ctx, newCall("set", name, value), &val); err != nil {
		return errors.Wrapf(err, "failed to set %q -> %q: %v", name, value)
	}
	return nil

}

// Get calls servo get function.
func (s *Servo) Get(ctx context.Context, name string) (string, error) {
	var val string
	if err := s.run(ctx, newCall("get", name), &val); err != nil {
		return val, errors.Wrapf(err, "failed to get %q value: %v", name)
	}
	return val, nil
}

// SwitchUSBKey switches the usb connected servo USB_KEY port between DUT and HOST.
func (s *Servo) SwitchUSBKey(ctx context.Context, usbState string) error {
	var muxDirection string
	if usbState == "host" {
		muxDirection = Host
	} else if usbState == "dut" {
		muxDirection = DUT
	} else {
		return errors.Errorf("%q usbstate is not supported", usbState)
	}
	if err := s.SetNoCheck(ctx, "image_usbkey_direction", muxDirection); err != nil {
		return err
	}
	return nil
}

// SwitchUSBKeyPower turns off the servo USB_KEY port.
func (s *Servo) SwitchUSBKeyPower(ctx context.Context, powerState string) error {
	if powerState != PowerOff && powerState != PowerOn {
		return errors.Errorf("%q powerstate is not supported", powerState)
	}
	if err := s.SetCheck(ctx, "image_usbkey_pwr", powerState); err != nil {
		return err
	}
	return nil
}
