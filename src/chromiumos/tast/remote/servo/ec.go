// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// These are the EC Servo controls which can be get/set with a string value.
const (
	ECBoard            StringControl = "ec_board"
	ECSystemPowerState StringControl = "ec_system_powerstate"
	ECUARTCmd          StringControl = "ec_uart_cmd"
	ECUARTRegexp       StringControl = "ec_uart_regexp"
	ECUARTStream       StringControl = "ec_uart_stream"
)

// These controls accept only "on" and "off" as values.
const (
	ECUARTCapture OnOffControl = "ec_uart_capture"
)

// RunECCommand runs the given command on the EC on the device.
func (s *Servo) RunECCommand(ctx context.Context, cmd string) error {
	if err := s.SetString(ctx, ECUARTRegexp, "None"); err != nil {
		return errors.Wrap(err, "Clearing EC UART Regexp")
	}
	return s.SetString(ctx, ECUARTCmd, cmd)
}

// RunECCommandGetOutput runs the given command on the EC on the device and returns the output matching patterns.
func (s *Servo) RunECCommandGetOutput(ctx context.Context, cmd string, patterns []string) ([]interface{}, error) {
	err := s.SetStringList(ctx, ECUARTRegexp, patterns)
	if err != nil {
		return nil, errors.Wrapf(err, "setting ECUARTRegexp to %s", patterns)
	}
	defer s.SetString(ctx, ECUARTRegexp, "None")
	err = s.SetString(ctx, ECUARTCmd, cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "setting ECUARTCmd to %s", cmd)
	}
	return s.GetStringList(ctx, ECUARTCmd)
}

// GetECSystemPowerState returns the power state, like "S0" or "G3"
func (s *Servo) GetECSystemPowerState(ctx context.Context) (string, error) {
	return s.GetString(ctx, ECSystemPowerState)
}

// ForceDownwardDataRole checks each USB-C port and performs a data swap if it is in the UFP role.
func (s *Servo) ForceDownwardDataRole(ctx context.Context) error {
	for pdChannel := 0; pdChannel <= 1; pdChannel++ {
		pdState, err := s.RunECCommandGetOutput(ctx, fmt.Sprintf("pd %d state", pdChannel), []string{`Role: (\S*) State: (\S*),`})
		if err != nil {
			return errors.Wrapf(err, "getting pd %d state", pdChannel)
		}
		result := pdState[0].([]interface{})
		role := result[1].(string)
		state := result[2].(string)
		if role == "SRC-UFP" || role == "SNK-UFP" {
			testing.ContextLogf(ctx, "PD %d: Swapping data role (Role %s State %s)", pdChannel, role, state)
			if err = s.RunECCommand(ctx, fmt.Sprintf("pd %d swap data", pdChannel)); err != nil {
				return errors.Wrapf(err, "Swapping pd %d data role", pdChannel)
			}
		}
	}
	return nil
}
