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

// RunStatus runs cros-health-tool's status command to get the status of the
// cros_healthd system daemon.
func RunStatus(ctx context.Context) ([]byte, error) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		return nil, errors.Wrap(err, "failed to start cros_healthd")
	}

	b, err := testexec.CommandContext(ctx, "cros-health-tool", "status").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run 'cros-health-tool status'")
	}
	return b, nil
}

// WaitForMojoBootstrap will poll the cros_healthd service to ensure that Chrome
// has successfully sent the external mojo remotes to cros_healthd.
func WaitForMojoBootstrap(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check that cros_healthd status is ready.
		out, err := RunStatus(ctx)
		if err != nil {
			return errors.Wrap(err, "unable to RunStatus")
		}
		lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
		if len(lines) != 3 {
			return errors.New("Unable to get cros_healthd status")
		}

		// Expected output:
		// cros_health service status: running
		// network health mojo remote bound: true
		// network diagnostics mojo remote bound: true
		m := map[string]string{
			lines[0]: "running",
			lines[1]: "true",
			lines[2]: "true",
		}
		for k, v := range m {
			if err := checkValue(k, v); err != nil {
				return err
			}
		}

		return nil
	}, &testing.PollOptions{Interval: 250 * time.Millisecond, Timeout: 15 * time.Second}); err != nil {
		return errors.Wrap(err, "timeout out waiting for cros_health bootstrap")
	}
	return nil
}

// checkValue checks the value in a key value pair is equal to the provided
// value. Expects line to containe a key: value pair separated by a colon (:).
func checkValue(line, value string) error {
	pair := strings.Split(line, ": ")
	if len(pair) != 2 {
		return errors.Errorf("got unexpected key value pair: %v", line)
	}

	if pair[1] != value {
		return errors.Errorf("unexpected value. Got %v; want %v", pair[1], value)
	}

	return nil
}
