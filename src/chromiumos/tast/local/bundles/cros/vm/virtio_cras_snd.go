// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const runAlsaConformanceTest string = "run-alsa-conformance-test.sh"

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtioCrasSnd,
		Desc:         "Tests that the crosvm CRAS virtio-snd device works correctly",
		Contacts:     []string{"woodychow@google.com", "crosvm-core@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{runAlsaConformanceTest},
		Timeout:      20 * time.Minute,
		SoftwareDeps: []string{"vm_host", "dlc"},
		Pre:          vm.Dlc(),
	})
}

func VirtioCrasSnd(ctx context.Context, s *testing.State) {
	// Create a temporary directory on the stateful partition rather than in memory.
	td, err := ioutil.TempDir("/usr/local/tmp", "tast.vm.VirtioCrasSnd.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	// The test needs the execute bit set on every component in the test directory
	// in order for rename(2) as a non-root user to succeed.
	if err := os.Chmod(td, 0755); err != nil {
		s.Fatal("Failed to change permissions on temporary directory: ", err)
	}

	data := s.PreValue().(vm.PreData)

	logFile := filepath.Join(s.OutDir(), "serial.log")

	params := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
		fmt.Sprintf("init=%s", s.DataPath(runAlsaConformanceTest)),
		"--",
		td,
	}

	// The sandbox needs to be disabled because the test creates some device nodes, which is
	// only possible when running as root in the initial namespace.
	args := []string{
		"--nofile=262144",
		"crosvm", "run",
		"-p", strings.Join(params, " "),
		"-c", "1",
		"-m", "256",
		"-s", td,
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", logFile),
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		"--cras-snd",
		"--disable-sandbox",
		data.Kernel,
	}

	output, err := os.Create(filepath.Join(s.OutDir(), "crosvm.log"))
	if err != nil {
		s.Fatal("Failed to create crosvm log file: ", err)
	}
	defer output.Close()

	s.Log("Running Alsa conformance test")
	cmd := testexec.CommandContext(ctx, "prlimit", args...)
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	log, err := ioutil.ReadFile(logFile)
	if err != nil {
		s.Fatal("Failed to read serial log: ", err)
	}
	s.Log(string(log))
}
