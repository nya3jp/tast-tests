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
	"runtime"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Virtiofs,
		Desc:     "Tests that the crosvm virtio-fs device works correctly",
		Contacts: []string{"chirantan@chromium.org", "crosvm-core@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Data: []string{
			"termina_rootfs_aarch64.img",
			"termina_rootfs_x86_64.img",
			"virtiofs_kernel_aarch64.xz",
			"virtiofs_kernel_x86_64.xz",
		},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"vm_host"},
	})
}

func Virtiofs(ctx context.Context, s *testing.State) {
	// Create a temporary directory on the stateful partition rather than in memory.
	td, err := ioutil.TempDir("/usr/local/tmp", "tast.vm.Virtiofs.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	var rootfs, vmlinux string
	if runtime.GOARCH == "amd64" {
		rootfs = s.DataPath("termina_rootfs_x86_64.img")
		vmlinux = s.DataPath("virtiofs_kernel_x86_64.xz")
	} else {
		rootfs = s.DataPath("termina_rootfs_aarch64.img")
		vmlinux = s.DataPath("virtiofs_kernel_aarch64.xz")
	}

	s.Log("Unpacking kernel")
	kernel := filepath.Join(td, "kernel")
	kernelSrc, err := os.Open(vmlinux)
	if err != nil {
		s.Fatal("Failed to open vmlinux: ", err)
	}

	kernelDst, err := os.Create(kernel)
	if err != nil {
		s.Fatal("Failed to create kernel destination file: ", err)
	}

	xz := testexec.CommandContext(ctx, "xz", "-d", "-c")
	xz.Stdin = kernelSrc
	xz.Stdout = kernelDst
	if err := xz.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to decompress kernel: ", err)
	}
	kernelSrc.Close()
	kernelDst.Close()

	shared := filepath.Join(td, "shared")
	if err := os.Mkdir(shared, 0755); err != nil {
		s.Fatal("Failed to create shared directory: ", err)
	}

	logFile := filepath.Join(s.OutDir(), "serial.log")

	// The sandbox needs to be disabled because the test creates some device nodes, which is
	// only possible when running as root in the initial namespace.
	args := []string{
		"run",
		"-p", "root=/dev/vda init=/usr/bin/run-pjdfstest -- shared",
		"-r", rootfs,
		"-c", "1",
		"-m", "256",
		"-s", td,
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", logFile),
		"--shared-dir", fmt.Sprintf("%s:shared:type=fs", shared),
		"--disable-sandbox",
		kernel,
	}

	s.Log("Running pjdfstests")
	cmd := testexec.CommandContext(ctx, "crosvm", args...)
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
