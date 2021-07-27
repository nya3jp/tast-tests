// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"

	"chromiumos/tast/errors"
)

// These are the Cr50 Servo controls which can be get/set with a string value.
const (
	CR50UARTCmd    StringControl = "cr50_uart_cmd"
	CR50UARTRegexp StringControl = "cr50_uart_regexp"
	CR50UARTStream StringControl = "cr50_uart_stream"
)

// These controls accept only "on" and "off" as values.
const (
	CR50UARTCapture OnOffControl = "cr50_uart_capture"
)

// RunCR50Command runs the given command on the Cr50 on the device.
func (s *Servo) RunCR50Command(ctx context.Context, cmd string) error {
	if err := s.SetString(ctx, CR50UARTRegexp, "None"); err != nil {
		return errors.Wrap(err, "Clearing CR50 UART Regexp")
	}
	return s.SetString(ctx, CR50UARTCmd, cmd)
}

// RunCR50CommandGetOutput runs the given command on the Cr50 on the device and returns the output matching patterns.
func (s *Servo) RunCR50CommandGetOutput(ctx context.Context, cmd string, patterns []string) ([]interface{}, error) {
	err := s.SetStringList(ctx, CR50UARTRegexp, patterns)
	if err != nil {
		return nil, errors.Wrapf(err, "setting CR50UARTRegexp to %s", patterns)
	}
	defer s.SetString(ctx, CR50UARTRegexp, "None")
	err = s.SetString(ctx, CR50UARTCmd, cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "setting CR50UARTCmd to %s", cmd)
	}
	return s.GetStringList(ctx, CR50UARTCmd)
}
