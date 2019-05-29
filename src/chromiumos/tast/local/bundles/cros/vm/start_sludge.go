// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartSludge,
		Desc:         "Checks that sludge VM can start correctly on wilco devices",
		Contacts:     []string{"tbegin@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host, wilco"},
	})
}

// StartSludge starts an instance of sludge VM and tests that the DTC binaries
// are running. If everything is running correctly, it then shuts down the VM.
func StartSludge(ctx context.Context, s *testing.State) {
	const (
		wilcoVMJob = "wilco_dtc"
		wilcoVMCID = "512"
	)

	s.Log("Starting Wilco DTC process")
	if err := upstart.RestartJob(ctx, wilcoVMJob); err != nil {
		s.Fatal("Wilco DTC process could not start: ", err)
	}

	for _, name := range []string{"ddv", "sa"} {
		s.Logf("Checking %v process", name)

		// Poll to check if the processes have started.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			cmd := testexec.CommandContext(ctx,
				"vsh", "--cid="+wilcoVMCID, "--", "pgrep", name)
			// Add a dummy buffer for stdin to force allocating a pipe. vsh uses
			// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
			cmd.Stdin = &bytes.Buffer{}

			out, err := cmd.CombinedOutput()
			if err != nil {
				return err
			}

			s.Logf("Process %v started with PID %s", name, out)
			return nil

		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Errorf("%v process not found: %v", name, err)
		}
	}

	s.Log("Stopping Wilco DTC process")
	if err := upstart.StopJob(ctx, wilcoVMJob); err != nil {
		s.Error("unable to stop Wilco DTC process")
	}
}
