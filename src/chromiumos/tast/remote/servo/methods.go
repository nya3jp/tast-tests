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

// ActChgPort
func (s *Servo) ActChgPort(ctx context.Context, port string) (bool, error) {
// func (s *Servo) Echo(ctx context.Context, message string) (string, error) {
	var val bool
	err := s.run(ctx, newCall("set", "active_chg_port", port), &val)
	return val, err
	// return err
}
	// var val string
	// var val string
	// // var val bool
	// err := s.run(ctx, newCall("active_chg_port", port))
	// return  err
// }
