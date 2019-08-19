// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package restart

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunTest performs the restarter test, which brings the container/VM down and
// back up again the required number of times, ensuring that `uptime` is
// correct each time.
func RunTest(ctx context.Context, s *testing.State, cont *vm.Container, numRestarts int) {
	var startupTime time.Time
	var err error
	if startupTime, err = startTime(ctx, cont); err != nil {
		s.Fatal("Failed to get startup time: ", err)
	}

	for i := 0; i < numRestarts; i++ {
		s.Logf("Restart #%d, startup time was %v", i+1, startupTime)
		if err := cont.VM.Stop(ctx); err != nil {
			s.Fatal("Failed to close VM: ", err)
		}

		// While the VM is down, this command is expected to fail.
		if out, err := cont.Command(ctx, "pwd").Output(); err == nil {
			s.Fatalf("Expected command to fail while the container was shut down, but got: %q", string(out))
		} else {
			s.Log("Received an expected error running a container command: ", err)
		}

		// Start the VM and container.
		if err := cont.VM.Start(ctx); err != nil {
			s.Fatal("Failed to start VM: ", err)
		}
		if err := cont.StartAndWait(ctx, s.OutDir()); err != nil {
			s.Fatal("Failed to start container: ", err)
		}

		// Compare start times.
		var newStartupTime time.Time
		if newStartupTime, err = startTime(ctx, cont); err != nil {
			s.Fatal("Failed to get new startup time: ", err)
		}
		if !newStartupTime.After(startupTime) {
			s.Errorf("Restarted container didnt have a later startup time, %v vs %v", startupTime, newStartupTime)
		}
		startupTime = newStartupTime
	}
}

func startTime(ctx context.Context, cont *vm.Container) (time.Time, error) {
	out, err := cont.Command(ctx, "uptime", "--since").Output(testexec.DumpLogOnError)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "failed to run uptime cmd")
	}
	t, err := time.Parse("2006-01-02 15:04:05\n", string(out))
	if err != nil {
		return time.Time{}, errors.Wrap(err, "failed to parse uptime")
	}
	return t, nil
}
