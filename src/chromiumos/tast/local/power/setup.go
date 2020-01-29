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
	"chromiumos/tast/local/setup"
	"chromiumos/tast/local/testexec"
)

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

// disableService is an setup.SetupAction that disables a service.
type disableService struct {
	ctx   context.Context
	name  string
	state int
}

// Setup stops a service if it exists and is running.
func (a *disableService) Setup() error {
	state, err := serviceStatus(a.ctx, a.name)
	if err != nil {
		return errors.Wrap(err, "unable to read service status")
	}
	a.state = state
	if a.state != serviceRunning {
		return nil
	}
	if err := testexec.CommandContext(a.ctx, "stop", a.name).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable to stop service %q", a.name)
	}
	return nil
}

// Cleanup restarts a service if it was running.
func (a *disableService) Cleanup() error {
	if a.state != serviceRunning {
		return nil
	}
	if err := testexec.CommandContext(a.ctx, "start", a.name).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable to restart service %q", a.name)
	}
	return nil
}

// DisableService creates a SetupAction that disables a service if it exists
// and is running.
func DisableService(ctx context.Context, name string) setup.SetupAction {
	return &disableService{
		ctx:   ctx,
		name:  name,
		state: serviceMissing,
	}
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

// setBacklightLux is a setup.SetupAction that sets the screen backlight
// brightness.
type setBacklightLux struct {
	ctx            context.Context
	prevBrightness uint
	lux            uint
}

// Setup sets the screen backlight lux to a.lux.
func (a *setBacklightLux) Setup() error {
	prevBrightness, err := getBacklightBrightness(a.ctx)
	if err != nil {
		return err
	}
	a.prevBrightness = prevBrightness
	brightness, err := getDefaultBacklightBrightness(a.ctx, a.lux)
	if err != nil {
		return err
	}
	return setBacklightBrightness(a.ctx, brightness)
}

// Cleanup restores the previous backlight brightness.
func (a *setBacklightLux) Cleanup() error {
	return setBacklightBrightness(a.ctx, a.prevBrightness)
}

// SetBacklightLux creates a SetupAction that sets the screen backlight to a
// given lux level.
func SetBacklightLux(ctx context.Context, lux uint) setup.SetupAction {
	return &setBacklightLux{
		ctx:            ctx,
		prevBrightness: 0,
		lux:            lux,
	}
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

// setKBBrightness is a setup.SetupAction that sets the keyboard backlight
// brightness.
type setKBBrightness struct {
	ctx            context.Context
	brightness     uint
	prevBrightness uint
}

// Setup sets the keyboard backlight brightness.
func (a *setKBBrightness) Setup() error {
	prevBrightness, err := getKeyboardBrightness(a.ctx)
	if err != nil {
		return err
	}
	a.prevBrightness = prevBrightness
	return setKeyboardBrightness(a.ctx, a.brightness)
}

// Cleanup restores the previous keyboard backlight brightness.
func (a *setKBBrightness) Cleanup() error {
	return setKeyboardBrightness(a.ctx, a.prevBrightness)
}

// SetKeyboardBrightness creates a setup.SetupAction that sets the keyboard
// backlight brightness.
func SetKeyboardBrightness(ctx context.Context, brightness uint) setup.SetupAction {
	return &setKBBrightness{
		ctx:            ctx,
		brightness:     brightness,
		prevBrightness: 0,
	}
}

// DefaultPowerSetup prepares a DUT to have a power test run by consistently
// configuring power draining components and disabling sources of variance.
func DefaultPowerSetup(ctx context.Context, s *setup.Setup) {
	s.Append(DisableService(ctx, "powerd"))
	s.Append(DisableService(ctx, "update-engine"))
	s.Append(DisableService(ctx, "vnc"))
	s.Append(DisableService(ctx, "dptf"))
	s.Append(SetBacklightLux(ctx, 150))
	s.Append(SetKeyboardBrightness(ctx, 24))

	// TODO: audio
	// TODO: WiFi
	// TODO: Battery discharge
	// TODO: bluetooth
	// TODO: SetLightbarBrightness
	// TODO: nightlight off
}
