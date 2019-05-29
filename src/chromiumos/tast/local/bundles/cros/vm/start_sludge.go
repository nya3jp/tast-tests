// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const wilcoVMJob = "wilco_dtc"
const wilcoVMCID = "512"

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartSludge,
		Desc:         "Checks that sludge VM can start on wilco devices",
		Contacts:     []string{"tbegin@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host, wilco"},
	})
}

// StartSludge - Starts an instance of sludge vm and tests that the DTC binaries
//  are able to start
func StartSludge(ctx context.Context, s *testing.State) {

	if !upstart.JobExists(ctx, wilcoVMJob) {
		s.Fatal("Wilco DTC process does not exist")
	}

	_, state, _, _ := upstart.JobStatus(ctx, wilcoVMJob)
	started := state == upstart.RunningState

	if started {
		s.Log("Wilco DTC process already running")
	}

	if err := upstart.EnsureJobRunning(ctx, wilcoVMJob); err != nil {
		s.Fatal("Wilco DTC process could not start: ", err)
	}

	// Wait for the VM and binaries to stabilize
	testing.Sleep(2 * time.Second)

	s.Log("Checking DDV Binary Status")
	output, err := checkProcessStatus(ctx, "ddv")
	if err != nil {
		s.Fatal("DDV Binary is not running: ", err.Error())
	}
	s.Logf("DDV Binary running with PID: %s", output)

	s.Log("Checking SA Binary Status")
	output, err = checkProcessStatus(ctx, "sa")
	if err != nil {
		s.Fatal("DDV Binary is not running: ", err.Error())
	}
	s.Logf("SA Binary running with PID: %s", output)

	// Stop the process if it was not started initially
	if !started {
		err := upstart.StopJob(ctx, wilcoVMJob)
		if err != nil {
			s.Log("Unable to stop Wilco DTC process")
		}
	}
}

func checkProcessStatus(ctx context.Context, processName string) ([]byte, error) {
	cmd := testexec.CommandContext(ctx,
		"vsh",
		fmt.Sprintf("--cid=%s", wilcoVMCID),
		"--",
		"pgrep",
		processName,
	)
	cmd.Stdin = &bytes.Buffer{}

	output, err := cmd.CombinedOutput()
	return output, err
}
