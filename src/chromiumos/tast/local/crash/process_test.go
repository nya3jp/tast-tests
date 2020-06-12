// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os/exec"
	"strings"
	"testing"
)

// startFakeProcess starts a fake process with a randomly generated process name.
// A fake process runs for 10 seconds and exits abnormally.
func startFakeProcess(t *testing.T) (cmd *exec.Cmd, procName string) {
	t.Helper()

	// Process name length must be up to TASK_COMM_LEN (16).
	procName = fmt.Sprintf("test_%d", rand.Int31())
	shell := fmt.Sprintf("echo -n %s > /proc/self/comm; sleep 10; exit 28", procName)
	cmd = exec.Command("sh", "-c", shell)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start fake crash_sender process: %v", err)
	}

	// Wait for the process name change to take effect.
	for {
		// We don't use gopsutil to get the process name here because it somehow
		// caches the process name until it exits.
		b, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/comm", cmd.Process.Pid))
		if err != nil {
			// In a race condition comm might not exist yet.
			// We do not check os.IsNotExist here because reading /proc files
			// might return random error code other than ENOENT (crbug.com/1042000#c9).
			continue
		}
		name := strings.TrimRight(string(b), "\n")
		if name == procName {
			break
		}
	}

	return cmd, procName
}

func TestProcessRunning(t *testing.T) {
	cmd, procName := startFakeProcess(t)
	func() {
		defer cmd.Wait()
		defer cmd.Process.Kill()

		running, err := processRunning(procName)
		if err != nil {
			t.Fatal("processRunning: ", err)
		}
		if !running {
			t.Fatal("processRunning = false; want true")
		}
	}()

	running, err := processRunning(procName)
	if err != nil {
		t.Fatal("processRunning: ", err)
	}
	if running {
		t.Fatal("processRunning = true; want false")
	}
}
