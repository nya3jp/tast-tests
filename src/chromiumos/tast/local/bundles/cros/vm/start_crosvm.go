// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"io"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartCrosvm,
		Desc:         "Checks that crosvm starts termina and runs commands through stdin",
		Contacts:     []string{"smbarber@chromium.org", "crosvm-dev@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host"},
		// TODO(smbarber): Download only files for the current architecture.
		Data: []string{
			"termina_kernel_aarch64",
			"termina_kernel_x86_64",
			"termina_rootfs_aarch64.img",
			"termina_rootfs_x86_64.img",
		},
	})
}

func StartCrosvm(ctx context.Context, s *testing.State) {
	kernelName, rootfsName, err := vmDataName(ctx)
	if err != nil {
		s.Fatal("Failed to find VM image: ", err)
	}
	kernelPath := s.DataPath(kernelName)
	rootfsPath := s.DataPath(rootfsName)

	cvm, err := vm.NewCrosvm(ctx, &vm.CrosvmParams{
		VMKernel:   kernelPath,
		RootfsPath: rootfsPath,
		KernelArgs: []string{"init=/bin/bash"},
	})
	if err != nil {
		s.Fatal("Failed to start crosvm: ", err)
	}
	defer cvm.Close(ctx)

	testing.ContextLog(ctx, "Waiting for VM to boot")
	startCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	line, err := cvm.WaitForOutput(startCtx, regexp.MustCompile("localhost\\b.*#"))
	if err != nil {
		s.Fatal("Didn't get VM prompt: ", err)
	}
	s.Logf("Saw prompt in line %q", line)

	const cmd = "/bin/ls -1 /"
	s.Logf("Running %q and waiting for output", cmd)
	if _, err = io.WriteString(cvm.Stdin(), cmd+"\n"); err != nil {
		s.Fatalf("Failed to write %q command: %v", cmd, err)
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if line, err = cvm.WaitForOutput(cmdCtx, regexp.MustCompile("^sbin$")); err != nil {
		s.Errorf("Didn't get expected %q output: %v", cmd, err)
	} else {
		s.Logf("Saw line %q", line)
	}
}

// vmDataName returns the name of the VM kernel and rootfs files to use for the current architecture.
func vmDataName(ctx context.Context) (string, string, error) {
	u, err := sysutil.Uname()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get uname")
	}
	if u.Machine == "aarch64" {
		return "termina_kernel_aarch64", "termina_rootfs_aarch64.img", nil
	} else if u.Machine == "x86_64" {
		return "termina_kernel_x86_64", "termina_rootfs_x86_64.img", nil
	}

	return "", "", errors.Errorf("no known VM image for architecture %q", u.Machine)
}
