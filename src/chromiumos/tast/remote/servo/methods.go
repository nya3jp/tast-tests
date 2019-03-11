// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package servo is used to communicate with servo devices connected to DUTs.
// It communicates with servod over XML-RPC.
// More details on servo: https://www.chromium.org/chromium-os/servo
package servo

import (
	"context"

	"chromiumos/tast/errors"
)

// Echo calls the Servo echo method.
func (s *Servo) Echo(ctx context.Context, message string) (string, error) {
	vals, err := s.call(ctx, "echo", message)
	if err != nil {
		return "", err
	}
	if len(vals) != 1 {
		return "", errors.Errorf("Echo got unexpected param len. Params: %q; expected len(1)", vals)
	}
	val := vals[0]
	if v, ok := val.(string); ok {
		return v, nil
	}
	return "", errors.Errorf("Echo got unexpected non-string param: %q", val)
}

// PowerNormalPress calls the Servo power_normal_press method.
func (s *Servo) PowerNormalPress(ctx context.Context) (bool, error) {
	vals, err := s.call(ctx, "power_normal_press")
	if err != nil {
		return false, err
	}
	if len(vals) != 1 {
		return false, errors.Errorf("PowerNormalPress got unexpected len. Params: %q; expected len(1)", vals)
	}
	val := vals[0]
	if v, ok := val.(bool); ok {
		return v, nil
	}
	return false, errors.Errorf("PowerNormalPress got unexpected non-bool param: %q", val)
}
