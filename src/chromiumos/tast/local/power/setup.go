// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"

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

// NewDefaultSetup prepares a DUT to have a power test run by consistently
// configuring power draining components and disabling sources of variance.
func NewDefaultSetup(ctx context.Context) *Setup {
	s := NewSetup(ctx)
	s.StopService("powerd")
	s.StopService("update-engine")
	s.StopService("vnc")
	s.StopService("dptf")

	// TODO: backlight
	// TODO: keyboard light
	// TODO: audio
	// TODO: WiFi
	// TODO: Battery discharge
	// TODO: bluetooth
	// TODO: SetLightbarBrightness
	// TODO: nightlight off
	return s
}
