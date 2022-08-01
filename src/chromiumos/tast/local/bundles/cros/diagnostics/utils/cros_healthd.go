// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// crosHealthdJobName is the name of the cros_healthd upstart job
const crosHealthdJobName = "cros_healthd"

// crosHealthdServiceName is the name of the cros_healthd D-Bus service
const crosHealthdServiceName = "org.chromium.CrosHealthd"

// statusResult contains the status of the current cros_healthd system
// service instance.
type statusResult struct {
	ServiceRunning                    bool
	NetworkHealthMojoRemoteBound      bool
	NetworkDiagnosticsMojoRemoteBound bool
}

// EnsureCrosHealthdRunning ensures cros_healthd is running correctly before running
// diagnostics tast test.
func EnsureCrosHealthdRunning(ctx context.Context) error {
	// Ensure cros_healthd is up.
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		return errors.Wrap(err, "failed to start cros_healthd")
	}

	// It is possible that cros_healthd actually crashes but the upstart job is regarded
	// as running. Wait for the D-Bus service to be available to detect this case.
	bus, err := dbusutil.SystemBus()
	if err != nil {
		return errors.Wrap(err, "failed to connect to the D-Bus system bus")
	}

	if err := dbusutil.WaitForService(ctx, bus, crosHealthdServiceName); err != nil {
		return errors.Wrapf(err, "failed to wait for %q D-Bus service",
			crosHealthdServiceName)
	}

	// Wait until the Mojo bootstrap flow is done.
	if err := waitForMojoBootstrap(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for mojo bootstrap")
	}

	return nil
}

// waitForMojoBootstrap will ensure that the cros_healthd is started and poll
// the cros_healthd service to ensure that Chrome has successfully sent the
// external mojo remotes to cros_healthd.
func waitForMojoBootstrap(ctx context.Context) error {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		return errors.Wrap(err, "failed to start cros_healthd")
	}

	// By default use the context's deadline for a timeout if set, otherwise
	// default to 15 seconds.
	deadline, ok := ctx.Deadline()
	timeout := 15 * time.Second
	if ok {
		timeout = deadline.Sub(time.Now())
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check that cros_healthd status is ready.
		status, err := runStatus(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(
				err, "unable to get cros_healthd status"))
		}
		if !status.ServiceRunning {
			return errors.New("cros_healthd service is not running")
		}
		if !status.NetworkHealthMojoRemoteBound {
			return errors.New("Network Health mojo remote is not bound")
		}
		if !status.NetworkDiagnosticsMojoRemoteBound {
			return errors.New("Network Diagnostics mojo remote is not bound")
		}

		return nil
	}, &testing.PollOptions{Interval: 250 * time.Millisecond,
		Timeout: timeout}); err != nil {
		return errors.Wrap(err, "timeout out waiting for cros_health bootstrap")
	}
	return nil
}

// runStatus runs cros-health-tool's status command to get the status of the
// cros_healthd system daemon. On success the function will return a
// *statusResult, or the error that occurred.
func runStatus(ctx context.Context) (*statusResult, error) {
	b, err := testexec.CommandContext(ctx, "cros-health-tool", "status").Output(
		testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run 'cros-health-tool status'")
	}

	output := string(b)
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	var status statusResult
	// Expected output:
	entries := []struct {
		key      string
		expected string
		dest     *bool
	}{{
		"cros_health service status: ",
		"running",
		&status.ServiceRunning,
	}, {
		"network health mojo remote bound: ",
		"true",
		&status.NetworkHealthMojoRemoteBound,
	}, {
		"network diagnostics mojo remote bound: ",
		"true",
		&status.NetworkDiagnosticsMojoRemoteBound,
	}}
	if len(lines) != len(entries) {
		return nil, errors.Errorf(
			"unexpected number of lines from cros_healthd status. Got %v, want %v: %q",
			len(lines), len(entries), output)
	}

	for i, e := range entries {
		// Check the field key is correct.
		if !strings.HasPrefix(lines[i], e.key) {
			return nil, errors.Errorf("unexpected key in line %q, want %q", lines[i],
				e.expected)
		}

		// Check the field value is correct. If not the value is false.
		*e.dest = (strings.TrimPrefix(lines[i], e.key) == e.expected)
	}

	return &status, nil
}
