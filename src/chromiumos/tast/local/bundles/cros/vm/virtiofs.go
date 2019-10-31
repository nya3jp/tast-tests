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
			"rootfs_aarch64.tar.xz",
			"rootfs_x86_64.tar.xz",
			"vmlinux_aarch64.xz",
			"vmlinux_x86_64.xz",
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

	s.Log("Unpacking kernel and root file system")
	var rootSrc, vmlinux string
	if runtime.GOARCH == "amd64" {
		rootSrc = s.DataPath("rootfs_x86_64.tar.xz")
		vmlinux = s.DataPath("vmlinux_x86_64.xz")
	} else {
		rootSrc = s.DataPath("rootfs_aarch64.tar.xz")
		vmlinux = s.DataPath("vmlinux_aarch64.xz")
	}

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
	if err := xz.Run(); err != nil {
		s.Fatal("Failed to decompress kernel: ", err)
	}
	kernelSrc.Close()
	kernelDst.Close()

	rootfs := filepath.Join(td, "rootfs")
	if err := os.Mkdir(rootfs, 0755); err != nil {
		s.Fatal("Failed to create directory for rootfs: ", err)
	}

	tar := testexec.CommandContext(ctx, "tar", "xaf", rootSrc, "-C", rootfs)
	if err := tar.Run(); err != nil {
		tar.DumpLog(ctx)
		s.Fatal("Failed to unpack root file system: ", err)
	}

	logFile := filepath.Join(s.OutDir(), "serial.log")

	// The sandbox needs to be disabled because the test creates some device nodes, which is
	// only possible when running as root in the initial namespace.
	args := []string{
		"run",
		"-p", "root=/dev/root rootfstype=virtiofs init=/bin/run-pjdfstest.sh",
		"-c", "1",
		"-m", "256",
		"-s", td,
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", logFile),
		"--shared-dir", fmt.Sprintf("%s:/dev/root:type=fs", rootfs),
		"--disable-sandbox",
		kernel,
	}

	s.Log("Running pjdfstests")
	cmd := testexec.CommandContext(ctx, "crosvm", args...)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run crosvm: ", err)
	}

	log, err := ioutil.ReadFile(logFile)
	if err != nil {
		s.Fatal("Failed to read serial log: ", err)
	}

	if strings.Contains(string(log), "FAIL") {
		s.Error("pjdfstests failed")
	}
}
