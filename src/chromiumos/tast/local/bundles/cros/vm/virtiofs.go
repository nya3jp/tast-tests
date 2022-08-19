// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/bundles/cros/vm/dlc"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const runPjdfstest string = "run-pjdfstest.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         Virtiofs,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that the crosvm virtio-fs device works correctly",
		Contacts:     []string{"crosvm-core@google.com"},
		// b:238260020 - disable aged (>1y) unpromoted informational tests
		// Attr:         []string{"group:mainline", "informational"},
		Data:         []string{runPjdfstest},
		Timeout:      20 * time.Minute,
		SoftwareDeps: []string{"vm_host", "chrome", "dlc"},
		Fixture:      "vmDLC",
	})
}

func setupCrosvmCmd(ctx context.Context, kernel, serialLog, script string, scriptArgs []string) *testexec.Cmd {
	kernParams := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
		fmt.Sprintf("init=%s", script),
		"--",
	}
	kernParams = append(kernParams, scriptArgs...)

	ps := vm.NewCrosvmParams(
		kernel,
		vm.SharedDir("/", "/dev/root", "fs", "always"),
		vm.DisableSandbox(),
		vm.KernelArgs(kernParams...),
		vm.SerialOutput(serialLog),
	)

	args := []string{"--nofile=262144", "crosvm"}
	args = append(args, ps.ToArgs()...)
	return testexec.CommandContext(ctx, "prlimit", args...)
}

func Virtiofs(ctx context.Context, s *testing.State) {
	// Create a temporary directory on the stateful partition rather than in memory.
	td, err := ioutil.TempDir("/usr/local/tmp", "tast.vm.Virtiofs.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	// The test needs the execute bit set on every component in the test directory
	// in order for rename(2) as a non-root user to succeed.
	if err := os.Chmod(td, 0755); err != nil {
		s.Fatal("Failed to change permissions on temporary directory: ", err)
	}

	data := s.FixtValue().(dlc.FixtData)

	logFile := filepath.Join(s.OutDir(), "serial.log")

	script := s.DataPath(runPjdfstest)

	output, err := os.Create(filepath.Join(s.OutDir(), "crosvm.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output.Close()

	s.Log("Running pjdfstests")
	cmd := setupCrosvmCmd(ctx, data.Kernel, logFile, script, []string{td})
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	log, err := ioutil.ReadFile(logFile)
	if err != nil {
		s.Fatal("Failed to read serial log: ", err)
	}

	lines := strings.Split(string(log), "\n")

	// Assume the test failed unless we see the "All tests successful" message. The log
	// is thousands of lines long and the messages we care about are at the end so iterate
	// over the lines in reverse order.
	failed := true
	failIdx := -1
	for idx := len(lines) - 1; idx >= 0; idx-- {
		if strings.HasPrefix(lines[idx], "All tests successful") {
			// The test passed. Nothing more to see here.
			failed = false
			break
		} else if strings.HasPrefix(lines[idx], "Failed Set") {
			failIdx = idx
			break
		}
	}

	if failed {
		if failIdx != -1 {
			// Print out the failed test summary. The "Kernel panic" indicates
			// the end of the summary and is triggered by PID 1 exiting.
			for _, l := range lines[failIdx:] {
				if strings.Contains(l, "Kernel panic") {
					break
				}
				s.Log(l)
			}
		}

		s.Error("pjdfstest failed")
	}
}
