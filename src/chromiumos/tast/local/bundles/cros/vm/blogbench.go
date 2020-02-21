// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

	"golang.org/x/sys/unix"

	"chromiumos/tast/local/bundles/cros/vm/common"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	kB int64 = 1024
	mB       = 1024 * kB
	gB       = 1024 * mB

	runBlogbench string = "run-blogbench.sh"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Blogbench,
		Desc:         "Tests crosvm storage device performance",
		Contacts:     []string{"chirantan@chromium.org", "crosvm-core@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Data:         []string{common.VirtiofsKernel(), runBlogbench},
		SoftwareDeps: []string{"vm_host"},
		Params: []testing.Param{
			{
				Name:    "block",
				Val:     "block",
				Timeout: 5 * time.Minute,
			},
			{
				Name:    "virtiofs",
				Val:     "fs",
				Timeout: 5 * time.Minute,
			},
			{
				Name:    "p9",
				Val:     "p9",
				Timeout: 5 * time.Minute,
			},
		},
	})
}

func Blogbench(ctx context.Context, s *testing.State) {
	// Create a temporary directory on the stateful partition rather than in memory.
	td, err := ioutil.TempDir("/usr/local/tmp", "tast.vm.Blogbench.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	vmlinux := s.DataPath(common.VirtiofsKernel())

	kernel := filepath.Join(td, "kernel")
	if err := common.UnpackKernel(ctx, vmlinux, kernel); err != nil {
		s.Fatal("Failed to unpack kernel: ", err)
	}

	shared, err := ioutil.TempDir(td, "shared.")
	if err != nil {
		s.Fatal("Failed to create shared directory: ", err)
	}
	defer os.RemoveAll(shared)

	f, err := ioutil.TempFile(td, "block.")
	if err != nil {
		s.Fatal("Failed to create block device file: ", err)
	}
	if err := f.Truncate(8 * gB); err != nil {
		s.Fatal("Failed to set block device file size: ", err)
	}
	block := f.Name()

	defer os.Remove(block)
	f.Close()

	// Increase the max open file limit as the benchmark creates a lot of files.
	rlim := &unix.Rlimit{
		Cur: 262144,
		Max: 262144,
	}
	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, rlim); err != nil {
		s.Fatal("Failed to increase open file limit: ", err)
	}

	logFile := filepath.Join(s.OutDir(), "serial.log")

	args := []string{
		"run",
		"-c", "1",
		"-m", "256",
		"-s", td,
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", logFile),
	}

	kind := s.Param().(string)
	var tag string
	if kind == "block" {
		tag = "/dev/vda"
		args = append(args, "--rwdisk", block)
	} else if kind == "fs" {
		tag = "shared"
		args = append(args, "--shared-dir",
			fmt.Sprintf("%s:%s:type=%s:cache=always:timeout=3600:writeback=true", shared, tag, kind))
	} else if kind == "p9" {
		tag = "shared"
		args = append(args, "--shared-dir", fmt.Sprintf("%s:%s", shared, tag))
	} else {
		s.Fatal("Unknown storage device type: ", err)
	}

	params := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		fmt.Sprintf("init=%s", s.DataPath(runBlogbench)),
		"--",
		kind,
		tag,
		td,
	}

	args = append(args, "-p", strings.Join(params, " "), kernel)

	s.Log("Running blogbench")
	cmd := testexec.CommandContext(ctx, "crosvm", args...)
	if err := cmd.LogStdout(); err != nil {
		s.Fatal("Failed to set up stdout log: ", err)
	}
	if err := cmd.LogStderr(); err != nil {
		s.Fatal("Failed to set up stderr log: ", err)
	}

	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to run crosvm: ", err)
	}

	log, err := ioutil.ReadFile(logFile)
	if err != nil {
		s.Fatal("Failed to read serial log: ", err)
	}

	lines := strings.Split(string(log), "\n")

	// The messages we care about are at the end of the log so iterate over the lines in
	// reverse order.
	writeScore := 0
	readScore := 0
	for idx := len(lines) - 1; idx >= 0; idx-- {
		if writeScore != 0 && readScore != 0 {
			break
		}

		if !strings.HasPrefix(lines[idx], "Final score for") {
			continue
		}

		if _, err := fmt.Sscanf(lines[idx], "Final score for writes:\t%v", &writeScore); err == nil {
			continue
		}

		if _, err := fmt.Sscanf(lines[idx], "Final score for reads :\t%v", &readScore); err != nil {
			s.Error("Failed to get score: ", err)
		}
	}

	s.Logf("Read score = %v, write score = %v", readScore, writeScore)

	perfValues := perf.NewValues()
	perfValues.Append(perf.Metric{
		Name:      "read",
		Variant:   kind,
		Unit:      "score",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, float64(readScore))

	perfValues.Append(perf.Metric{
		Name:      "write",
		Variant:   kind,
		Unit:      "score",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, float64(writeScore))
	perfValues.Save(s.OutDir())
}
