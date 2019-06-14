// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"io"
	"regexp"
	"time"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartCrosvm,
		Desc:         "Checks that crosvm starts termina and runs commands through stdin",
		Contacts:     []string{"jkardatzke@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host"},
	})
}

func StartCrosvm(ctx context.Context, s *testing.State) {
	componentPath, err := vm.LoadTerminaComponent(ctx)
	if err != nil {
		s.Fatal("Unable to load component: ", err)
	}
	cvm, err := vm.NewCrosvm(ctx, &vm.CrosvmParams{
		KernelArgs: []string{"init=/bin/bash"},
		VMPath:     componentPath,
	})
	if err != nil {
		s.Fatal("Failed to start crosvm: ", err)
	}
	defer cvm.Close(ctx)

	testing.ContextLog(ctx, "Waiting for VM to boot")
	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
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
	cmdCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	if line, err = cvm.WaitForOutput(cmdCtx, regexp.MustCompile("^sbin$")); err != nil {
		s.Errorf("Didn't get expected %q output: %v", cmd, err)
	} else {
		s.Logf("Saw line %q", line)
	}
}
