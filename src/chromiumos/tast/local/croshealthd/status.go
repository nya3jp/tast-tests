// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package croshealthd

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// statusResult contains the status of the current cros_healthd system
// service instance.
type statusResult struct {
	ServiceRunning                    bool
	NetworkHealthMojoRemoteBound      bool
	NetworkDiagnosticsMojoRemoteBound bool
}

// runStatus runs cros-health-tool's status command to get the status of the
// cros_healthd system daemon. On success the function will return a
// *statusResult, or the error that occurred.
func runStatus(ctx context.Context) (*statusResult, error) {
	b, err := testexec.CommandContext(ctx, "cros-health-tool", "status").Output(testexec.DumpLogOnError)
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
		return nil, errors.Errorf("unexpected number of lines from cros_healthd status. Got %v, want %v: %q", len(lines), len(entries), output)
	}

	for i, e := range entries {
		// Check the field key is correct.
		if !strings.HasPrefix(lines[i], e.key) {
			return nil, errors.Errorf("unexpected key in line %q, want %q", lines[i], e.expected)
		}

		// Check the field value is correct. If not the value is false.
		*e.dest = (strings.TrimPrefix(lines[i], e.key) == e.expected)
	}
	return &status, nil
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
			return testing.PollBreak(errors.Wrap(err, "unable to get cros_healthd status"))
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
	}, &testing.PollOptions{Interval: 250 * time.Millisecond, Timeout: timeout}); err != nil {
		return errors.Wrap(err, "timeout out waiting for cros_health bootstrap")
	}
	return nil
}
