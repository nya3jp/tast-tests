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
const runPjdfstestVhostUser string = "run-pjdfstest-vhost.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         Virtiofs,
		Desc:         "Tests that the crosvm virtio-fs device works correctly",
		Contacts:     []string{"chirantan@chromium.org", "crosvm-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{runPjdfstest, runPjdfstestVhostUser},
		Timeout:      20 * time.Minute,
		SoftwareDeps: []string{"vm_host", "dlc"},
		Fixture:      "vmDLC",
		Params: []testing.Param{{
			Val: false,
		}, {
			Name: "vhost_user_fs",
			Val:  true,
		}},
	})
}

func setupCrosvmParams(kernel, serialLog, script string, scriptArgs []string) *vm.CrosvmParams {
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
	return ps
}

func runTest(ctx context.Context, s *testing.State, crosvmParams *vm.CrosvmParams) {
	output, err := os.Create(filepath.Join(s.OutDir(), "crosvm.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}

	defer output.Close()
	crosvmArgs := []string{"--nofile=262144", "crosvm"}
	crosvmArgs = append(crosvmArgs, crosvmParams.ToArgs()...)
	cmd := testexec.CommandContext(ctx, "prlimit", crosvmArgs...)
	cmd.Stdout = output
	cmd.Stderr = output
	devlog, err := os.Create(filepath.Join(s.OutDir(), "device.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer devlog.Close()

	s.Log("Running pjdfstests")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}
}

func runTestWithVhostUserFS(ctx context.Context, s *testing.State, crosvmParams *vm.CrosvmParams, td string) {
	output, err := os.Create(filepath.Join(s.OutDir(), "crosvm.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}

	defer output.Close()
	sock := filepath.Join(td, "vhost-user-fs.sock")
	vm.VhostUserFS(sock, "fstag")(crosvmParams)
	crosvmArgs := []string{"--nofile=262144", "crosvm"}
	crosvmArgs = append(crosvmArgs, crosvmParams.ToArgs()...)
	cmd := testexec.CommandContext(ctx, "prlimit", crosvmArgs...)
	cmd.Stdout = output
	cmd.Stderr = output
	devlog, err := os.Create(filepath.Join(s.OutDir(), "device.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer devlog.Close()

	devArgs := []string{
		"device",
		"fs",
		"--socket", sock,
		"--tag", "fstag",
		"--shared-dir", td,
	}
	devCmd := testexec.CommandContext(ctx, "crosvm", devArgs...)
	devCmd.Stdout = devlog
	devCmd.Stderr = devlog
	if err := devCmd.Start(); err != nil {
		s.Fatal("Failed to start vhost-user fs device: ", err)
	}

	s.Log("Running pjdfstests")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	// vhost-user-fs device must stop right after all of VMs stopped.
	if err := devCmd.Wait(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to complete vhost-user-net-device: ", err)
	}
}

func Virtiofs(ctx context.Context, s *testing.State) {
	isVhostUserFS := s.Param().(bool)

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

	if isVhostUserFS {
		script := s.DataPath(runPjdfstestVhostUser)
		crosvmParams := setupCrosvmParams(data.Kernel, logFile, script, []string{td})
		runTestWithVhostUserFS(ctx, s, crosvmParams, td)
	} else {
		script := s.DataPath(runPjdfstest)
		crosvmParams := setupCrosvmParams(data.Kernel, logFile, script, []string{td})
		runTest(ctx, s, crosvmParams)
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
