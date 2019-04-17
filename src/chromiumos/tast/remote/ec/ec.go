// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ec is used to communicate with EC console through servo
package ec

import (
	"chromiumos/tast/remote/servo"
	"context"
	"errors"
	"strings"
)

const (
	ecUARTCmd = "ec_uart_cmd"
	ecRegexp  = "ec_uart_regexp"
)

type EC struct {
	s *servo.Servo
}

func New(s *servo.Servo) *EC {
	return &EC{s}
}

func (e *EC) uartCmd(ctx context.Context, command string) (bool, error) {
	if _, err := e.uartRegexp(ctx, []string{}); err != nil {
		return false, errors.New("Failed to set ec_uart_regexp")
	}
	var val bool
	if err := e.s.Run(ctx, servo.NewCall("set", ecUARTCmd, command), &val); err != nil {
		return false, err
	}
	return val, nil
}

func (e *EC) uartRegexp(ctx context.Context, patterns []string) (bool, error) {

	var expr string
	if len(patterns) == 0 {
		expr = "None"
	} else {
		// Convert Go string array to Python string array literal
		expr = "['" + strings.Join(patterns, "','") + "']"
	}
	var val bool
	err := e.s.Run(ctx, servo.NewCall("set", ecRegexp, expr), &val)
	return val, err

}

func (e *EC) Output(ctx context.Context, command string, patterns []string) (string, error) {
	if _, err := e.uartRegexp(ctx, patterns); err != nil {
		return "", errors.New("Failed to set ec_uart_regexp")
	}
	var val bool
	e.s.Run(ctx, servo.NewCall("set", ecUARTCmd, command), &val)
	var ecoutput string
	if err := e.s.Run(ctx, servo.NewCall("get", ecUARTCmd), &ecoutput); err != nil {
		return "", err
	}
	return ecoutput, nil
}
