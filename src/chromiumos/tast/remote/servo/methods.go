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

// SetUartRegexp : Method to Set the Regular Expression
func (s *Servo) SetUartRegexp(ctx context.Context, regexps []string) bool {
	regexp := ""
	if len(regexps) == 0 {
		regexp = "None"
	} else {
		regexp += "["
		for _, exp := range regexps {
			regexp += "'" + exp + "',"
		}
		regexp += "]"
	}
	var val bool
	err := s.run(ctx, newCall("set", "ec_uart_regexp", regexp), &val)
	if err != nil {
		return false
	}
	return val

}

//SendEcCommand : Method to send the EC command to EC console
func (s *Servo) SendEcCommand(ctx context.Context, command string) (bool, error) {
	if !s.SetUartRegexp(ctx, make([]string, 0)) {
		return false, errors.New("Failed to set ec_uart_regexp")
	}
	var val bool
	err := s.run(ctx, newCall("set", "ec_uart_cmd", command), &val)
	if err != nil {
		return false, err
	}
	return true, nil
}

//SendECCommandAndGetOutput : Method to send the EC command and get the EC output response by inputing regular expression
func (s *Servo) SendEcCommandAndGetOutput(ctx context.Context, command string, regexps []string) (string, error) {
	if !s.SetUartRegexp(ctx, regexps) {
		return "", errors.New("Failed to set ec_uart_regexp")
	}
	var val bool
	s.run(ctx, newCall("set", "ec_uart_cmd", command), &val)
	var ecoutput string
	err := s.run(ctx, newCall("get", "ec_uart_cmd"), &ecoutput)
	if err != nil {
		return "", err
	}
	return ecoutput, nil
}
