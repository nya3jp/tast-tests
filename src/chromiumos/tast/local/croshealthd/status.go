// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package croshealthd

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// runStatus runs cros-health-tool's status command to get the status of the
// cros_healthd system daemon.
func runStatus(ctx context.Context) ([]string, error) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		return nil, errors.Wrap(err, "failed to start cros_healthd")
	}

	b, err := testexec.CommandContext(ctx, "cros-health-tool", "status").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run 'cros-health-tool status'")
	}

	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	return lines, nil
}

// waitForMojoBootstrap will poll the cros_healthd service to ensure that Chrome
// has successfully sent the external mojo remotes to cros_healthd.
func waitForMojoBootstrap(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check that cros_healthd status is ready.
		lines, err := runStatus(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "unable to runStatus"))
		}
		if len(lines) != 3 {
			return testing.PollBreak(errors.New("unable to get cros_healthd status"))
		}

		// Expected output:
		expected := []string{
			"cros_health service status: running",
			"network health mojo remote bound: true",
			"network diagnostics mojo remote bound: true",
		}
		for i, e := range expected {
			if lines[i] != e {
				return errors.Errorf("unexpected output line. got %v, want %v", lines[i], e)
			}
		}

		return nil
	}, &testing.PollOptions{Interval: 250 * time.Millisecond, Timeout: 15 * time.Second}); err != nil {
		return errors.Wrap(err, "timeout out waiting for cros_health bootstrap")
	}
	return nil
}
