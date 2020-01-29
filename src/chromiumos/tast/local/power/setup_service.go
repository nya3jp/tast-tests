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

// disableService is an Action that disables a service.
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

// DisableService creates an Action that disables a service if it exists
// and is running.
func DisableService(ctx context.Context, name string) Action {
	return &disableService{
		ctx:   ctx,
		name:  name,
		state: serviceMissing,
	}
}
