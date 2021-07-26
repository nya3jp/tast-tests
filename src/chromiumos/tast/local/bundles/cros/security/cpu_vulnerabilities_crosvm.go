// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CPUVulnerabilitiesCrosvm,
		Desc: "Confirm CPU vulnerabilities are mitigated in the guest kernel",
		Contacts: []string{
			"swboyd@chromium.org", // Tast port author
			"crosvm-dev@google.com",
			"chromeos-security@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host"},
		Pre:          vm.Artifact(),
		Data:         []string{vm.ArtifactData()},
	})
}

func CPUVulnerabilitiesCrosvm(ctx context.Context, s *testing.State) {
	data := s.PreValue().(vm.PreData)

	td, err := ioutil.TempDir("", "tast.security.CPUVulnerabilitiesCrosvm.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	ps := vm.NewCrosvmParams(
		data.Kernel,
		vm.Socket(filepath.Join(td, "crosvm_socket")),
		vm.Rootfs(data.Rootfs),
		vm.KernelArgs("init=/bin/sh PS1=tast>>"),
	)

	cvm, err := vm.NewCrosvm(ctx, ps)
	if err != nil {
		s.Fatal("Failed to start crosvm: ", err)
	}
	defer func() {
		if err := cvm.Close(ctx); err != nil {
			s.Error("Failed to close crosvm: ", err)
		}
	}()

	testing.ContextLog(ctx, "Waiting for VM to boot")
	startCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	line, err := cvm.WaitForOutput(startCtx, regexp.MustCompile("Run /bin/sh as init process"))
	if err != nil {
		s.Fatal("VM didn't boot: ", err)
	}
	s.Logf("Saw kernel run init in line %q", line)

	s.Log("Mounting sysfs")
	const mountCmd = "/bin/mount -t sysfs sys /sys"
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if lines, err := cvm.RunCommand(s, cmdCtx, mountCmd); err != nil {
		s.Error("Couldn't mount sysfs: ", err)
	} else {
		s.Logf("Lines are: ", lines)
	}

	const cmd = "/bin/grep -li not /sys/devices/system/cpu/vulnerabilities/*"
	cmdCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if files, err := cvm.RunCommand(s, cmdCtx, cmd); err != nil {
		s.Error("Couldn't grep for vulnerable in sysfs: ", err)
	} else {
		s.Logf("Lines are: ", files)

		if len(files) > 0 {
			for _, f := range files {
				s.Errorf("File %q has CPU vulnerabilities", f)
			}
		}
	}
}
