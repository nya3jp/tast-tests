// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"strings"
	"time"

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
	DUTPDDataRole      StringControl = "dut_pd_data_role"
)

// These controls accept only "on" and "off" as values.
const (
	ECUARTCapture OnOffControl = "ec_uart_capture"
)

// USBCDataRole is a USB-C data role.
type USBCDataRole string

// USB-C data roles.
const (
	// UFP is Upward facing partner, i.e. a peripheral. The servo should normally be in this role.
	UFP USBCDataRole = "UFP"
	// DFP is Downward facing partner, i.e. a host. The DUT should normally be in this role.
	DFP USBCDataRole = "DFP"
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

// ECHibernate puts the EC into hibernation mode, after removing the servo watchdog for CCD if necessary.
func (s *Servo) ECHibernate(ctx context.Context) error {
	// hibernateDelay is the time after the EC hibernate command where it still writes output
	const hibernateDelay = 1 * time.Second

	if err := s.WatchdogRemove(ctx, WatchdogCCD); err != nil {
		return errors.Wrap(err, "failed to remove watchdog for ccd")
	}
	if err := s.RunECCommand(ctx, "hibernate"); err != nil {
		return errors.Wrap(err, "failed to run EC command")
	}
	testing.Sleep(ctx, hibernateDelay)

	// Verify the EC console is unresponsive, ignore null chars, sometimes the servo returns null when the EC is off.
	out, err := s.RunECCommandGetOutput(ctx, "version", []string{`[^\x00]+`})
	if err == nil {
		testing.ContextLogf(ctx, "Got %v expected error", out)
		return errors.New("EC is still active after hibernate")
	}
	if !strings.Contains(err.Error(), "No data was sent from the pty") && !strings.Contains(err.Error(), "EC: Timeout waiting for response.") {
		return errors.Wrap(err, "unexpected EC error")
	}
	return nil
}

// SetDUTPDDataRole tries to find the port attached to the servo, and performs a data role swap if the role doesn't match `role`.
// Will fail if there is no chromeos EC.
func (s *Servo) SetDUTPDDataRole(ctx context.Context, role USBCDataRole) error {
	return s.SetString(ctx, DUTPDDataRole, string(role))
}
