// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"chromiumos/tast/errors"
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

// Set calls Servo get and set functions
// First will check the default value then proceeds with setting value
func (s *Servo) Set(ctx context.Context, args ...interface{}) error {
	var val interface{}
	// Checking the value is set already
	value, err := s.Get(ctx, args[0])
	if err != nil {
		return errors.Errorf("Failed to get %q value", args[0])
	}
	if value == args[1] {
		return nil
	}

	// Setting the value if not set
	if err := s.run(ctx, newCall("set", args...), &val); err != nil {
		return errors.Errorf("Failed to set %q", args)
	}

	// Verifying the value is set correctly
	value, err = s.Get(ctx, args[0])
	if err != nil {
		return errors.Errorf("Failed to get %q value", args[0])
	}
	if value != args[1].(string) {
		return errors.Errorf("%q is not set", args)
	}
	return nil

}

// SetNoCheck calls Servo set function
// Method sets the vlaue without any check
func (s *Servo) SetNoCheck(ctx context.Context, args ...interface{}) error {
	var val interface{}
	if err := s.run(ctx, newCall("set", args...), &val); err != nil {
		return errors.Errorf("Failed to set %q", args)
	}
	return nil

}

// Get calls servo get function
func (s *Servo) Get(ctx context.Context, args ...interface{}) (string, error) {
	var val string
	if err := s.run(ctx, newCall("get", args...), &val); err != nil {
		return val, errors.Errorf("Failed to get %q value", args)
	}
	return val, nil
}

// SwitchUSBKey switches the usb connected servo USB_KEY port between DUT and HOST
func (s *Servo) SwitchUSBKey(ctx context.Context, usbState string) error {
	var muxDirection string
	if usbState == "host" {
		muxDirection = "servo_sees_usbkey"
	} else if usbState == "dut" {
		muxDirection = "dut_sees_usbkey"
	} else {
		return errors.Errorf("%q usbstate is not supported", usbState)
	}
	if err := s.SetNoCheck(ctx, "image_usbkey_direction", muxDirection); err != nil {
		return err
	}
	return nil
}

// SwitchUSBKeyPower turns off the servo USB_KEY port
func (s *Servo) SwitchUSBKeyPower(ctx context.Context, powerState string) error {
	if powerState != "off" && powerState != "on" {
		return errors.Errorf("%q powerstate is not supported", powerState)
	}
	if err := s.Set(ctx, "image_usbkey_pwr", powerState); err != nil {
		return err
	}
	return nil
}
