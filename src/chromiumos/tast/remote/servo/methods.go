// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"chromiumos/tast/errors"
)


const (
	ecUARTCmd = "ec_uart_cmd"
	ecRegExp  = "ec_uart_regexp"
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
		// Converting "go slice([]string{"exp1", "exp2"})" to "['exp1', 'exp2' ...]"
		regexp += "["
		for _, exp := range regexps {
			regexp += "'" + exp + "',"
		}
		regexp += "]"
	}
	var val bool
	err := s.run(ctx, newCall("set", ecRegExp, regexp), &val)
	if err != nil {
		return false
	}
	return val

}

// SendEcCommand : Method to send the EC command to EC console
func (s *Servo) SendEcCommand(ctx context.Context, command string) (bool, error) {
	if !s.SetUartRegexp(ctx, make([]string, 0)) {
		return false, errors.New("Failed to set ec_uart_regexp")
	}
	var val bool
	err := s.run(ctx, newCall("set", ecUARTCmd, command), &val)
	if err != nil {
		return false, err
	}
	return true, nil
}

// SendECCommandAndGetOutput : Method to send the EC command and get the EC output response by inputing regular expression
func (s *Servo) SendEcCommandAndGetOutput(ctx context.Context, command string, regexps []string) (string, error) {
	if !s.SetUartRegexp(ctx, regexps) {
		return "", errors.New("Failed to set ec_uart_regexp")
	}
	var val bool
	s.run(ctx, newCall("set", ecUARTCmd, command), &val)
	var ecoutput string
	err := s.run(ctx, newCall("get", ecUARTCmd), &ecoutput)
	if err != nil {
		return "", err
	}
	return ecoutput, nil
}
