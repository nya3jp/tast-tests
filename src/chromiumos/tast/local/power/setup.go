// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// Setup tracks all the things we need to do to put the DUT back to how it
// was configured before this power test.
type Setup struct {
	ctx       context.Context
	callbacks []func()
	err       error
}

// NewSetup creates a new test setup and cleanup struct.
func NewSetup(ctx context.Context) *Setup {
	return &Setup{ctx: ctx, callbacks: []func(){}, err: nil}
}

// append adds a cleanup task to the setup.
func (s *Setup) append(callback func()) {
	if s.err != nil {
		callback()
		return
	}
	s.callbacks = append(s.callbacks, callback)
}

// fail checks a result needed for setup. If there is an error, then
// the setup is marked as failed, and all previous cleanup tasks are run.
func (s *Setup) fail(err error) {
	if s.err != nil {
		return
	}
	s.Cleanup()
	s.err = err
}

// Cleanup restores a DUT to is pre-test configuration.
func (s *Setup) Cleanup() {
	for _, callback := range s.callbacks {
		callback()
	}
	s.callbacks = []func(){}
}

// Error checks to see if the setup was successful.
func (s *Setup) Error() error {
	return s.err
}

// serviceStatusRe is used to parse the result of a service status command.
var serviceStatusRe = regexp.MustCompile("^.* (start/running, process \\d+)|(stop/waiting)\n$")

const (
	serviceMissing = iota
	serviceRunning
	serviceStopped
)

// serviceStatus returns the status of a service.
func serviceStatus(ctx context.Context, serviceName string) (int, error) {
	output, err := testexec.CommandContext(ctx, "status", serviceName).Output()
	if err != nil {
		return serviceMissing, nil
	}
	match := serviceStatusRe.FindSubmatch(output)
	if match == nil {
		return serviceMissing, errors.Wrapf(err, "unable to parse status %q of service %q", output, serviceName)
	}
	if match[1] != nil {
		return serviceRunning, nil
	} else if match[2] != nil {
		return serviceStopped, nil
	}
	return serviceMissing, nil
}

// StopService stops a service if it is running, and updates cleanup callbacks
// to restart the service.
func (s *Setup) StopService(serviceName string) {
	if s.Error() != nil {
		return
	}
	prevStatus, err := serviceStatus(s.ctx, serviceName)
	if err != nil {
		s.fail(err)
		return
	}
	if prevStatus != serviceRunning {
		// Service is not running, so we don't need to do anything.
		return
	}
	if err := testexec.CommandContext(s.ctx, "stop", serviceName).Run(testexec.DumpLogOnError); err != nil {
		s.fail(errors.Wrap(err, "unable to stop service"))
		return
	}
	s.append(func() {
		testexec.CommandContext(s.ctx, "start", serviceName).Run(testexec.DumpLogOnError)
	})
}

// getBacklightBrightness returns the current backlight brightness in percent.
func getBacklightBrightness(ctx context.Context) (uint, error) {
	output, err := testexec.CommandContext(ctx, "backlight_tool", "--get_brightness").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get current backlight brightness")
	}
	brightness, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to parse current backlight brightness from %q", output)
	}
	return uint(brightness), nil
}

// getDefaultBacklightBrightness returns the backlight brightness at a given
// lux level.
func getDefaultBacklightBrightness(ctx context.Context, lux uint) (uint, error) {
	luxArg := "--lux=" + strconv.FormatUint(uint64(lux), 10)
	output, err := testexec.CommandContext(ctx, "backlight_tool", "--get_initial_brightness", luxArg).Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get default backlight brightness")
	}
	brightness, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "unable to parse default backlight brightness")
	}
	return uint(brightness), nil
}

// setBacklightBrightness sets the backlight brightness.
func setBacklightBrightness(ctx context.Context, brightness uint) error {
	brightnessArg := "--set_brightness=" + strconv.FormatUint(uint64(brightness), 10)
	if err := testexec.CommandContext(ctx, "backlight_tool", brightnessArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to set backlight brightness")
	}
	return nil
}

// SetBacklightBrightness sets the backlight to have a given lux level, and
// updates cleanup callbacks to restore the backlight to it's original
// brightness.
func (s *Setup) SetBacklightBrightness(lux uint) {
	if s.Error() != nil {
		return
	}
	prevBrightness, err := getBacklightBrightness(s.ctx)
	if err != nil {
		s.fail(err)
		return
	}
	targetBrightness, err := getDefaultBacklightBrightness(s.ctx, lux)
	if err != nil {
		s.fail(err)
		return
	}
	if err := setBacklightBrightness(s.ctx, targetBrightness); err != nil {
		s.fail(err)
		return
	}
	s.append(func() {
		setBacklightBrightness(s.ctx, prevBrightness)
	})
}

// getKeyboardBrightness gets the keyboard brightness.
func getKeyboardBrightness(ctx context.Context) (uint, error) {
	output, err := testexec.CommandContext(ctx, "backlight_tool", "--keyboard", "--get_brightness").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get current keyboard brightness")
	}
	brightness, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to parse current keyboard brightness from %q", output)
	}
	return uint(brightness), nil
}

// setKeyboardBrightness sets the keyboard brightness.
func setKeyboardBrightness(ctx context.Context, brightness uint) error {
	brightnessArg := "--set_brightness=" + strconv.FormatUint(uint64(brightness), 10)
	if err := testexec.CommandContext(ctx, "backlight_tool", "--keyboard", brightnessArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to set keyboard brightness")
	}
	return nil
}

// SetKeyboardBrightness sets the keyboard brightness and updates cleanup
// callbacks to restore the keyboard backlight to it's original brightness.
func (s *Setup) SetKeyboardBrightness(brightness uint) {
	if s.Error() != nil {
		return
	}
	prevBrightness, err := getKeyboardBrightness(s.ctx)
	if err != nil {
		s.fail(err)
		return
	}
	if err := setKeyboardBrightness(s.ctx, brightness); err != nil {
		s.fail(err)
		return
	}
	s.append(func() {
		setKeyboardBrightness(s.ctx, prevBrightness)
	})
}

// NewDefaultSetup prepares a DUT to have a power test run by consistently
// configuring power draining components and disabling sources of variance.
func NewDefaultSetup(ctx context.Context) *Setup {
	s := NewSetup(ctx)
	s.StopService("powerd")
	s.StopService("update-engine")
	s.StopService("vnc")
	s.StopService("dptf")
	s.SetBacklightBrightness(150)
	s.SetKeyboardBrightness(24)

	// TODO: audio
	// TODO: WiFi
	// TODO: Battery discharge
	// TODO: bluetooth
	// TODO: SetLightbarBrightness
	// TODO: nightlight off
	return s
}
