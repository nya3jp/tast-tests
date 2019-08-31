// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
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
func (s *Servo) SetActChgPort(ctx context.Context, port string) (bool, error) {
	var val bool
	err := s.run(ctx, newCall("set", "active_chg_port", port), &val)
	return val, err
}

// DutVoltageMV reads the voltage present on the DUT port on fluffy.
func (s *Servo) DutVoltageMV(ctx context.Context) (string, error) {
	var voltageMV string
	err := s.run(ctx, newCall("get", "dut_voltage_mv"), &voltageMV)
	return voltageMV, err
}
