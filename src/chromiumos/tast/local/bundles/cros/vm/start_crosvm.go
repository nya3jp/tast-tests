// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"io"
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
		Func:     StartCrosvm,
		Desc:     "Checks that crosvm starts termina and runs commands through stdin",
		Contacts: []string{"smbarber@chromium.org", "crosvm-dev@google.com"},
		// b:238260020 - disable aged (>1y) unpromoted informational tests
		// Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host"},
		Pre:          vm.Artifact(),
		Data:         []string{vm.ArtifactData()},
	})
}

func StartCrosvm(ctx context.Context, s *testing.State) {
	data := s.PreValue().(vm.PreData)

	td, err := ioutil.TempDir("", "tast.vm.StartCrosvm.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(td)

	ps := vm.NewCrosvmParams(
		data.Kernel,
		vm.Socket(filepath.Join(td, "crosvm_socket")),
		vm.Rootfs(data.Rootfs),
		vm.KernelArgs("init=/bin/bash"),
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
