// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servo is used to communicate with servo devices connected to DUTs.
// It communicates with servod over XML-RPC.
// More details on servo: https://www.chromium.org/chromium-os/servo
package servo

import (
	"context"
)

// Echo calls the Servo echo method.
func (s *Servo) Echo(ctx context.Context, message string) (string, error) {
	req := methodCall{"echo", []param{param{value{String: message}}}}
	res, err := s.call(ctx, req)
	if err != nil {
		return "", err
	}
	return res.Params[0].Value.String, err
}

// PowerNormalPress calls the Servo power_normal_press method.
func (s *Servo) PowerNormalPress(ctx context.Context) (bool, error) {
	req := methodCall{"power_normal_press", []param{}}
	res, err := s.call(ctx, req)
	if err != nil {
		return false, err
	}
	return xmlBooleanToBoolean(res.Params[0].Value.Boolean)
}
