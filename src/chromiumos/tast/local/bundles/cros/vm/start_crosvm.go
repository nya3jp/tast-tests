// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"io"
	"regexp"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartCrosvm,
		Desc:         "Checks that crosvm starts and runs commands",
		Contacts:     []string{"jkardatzke@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host"},
	})
}

// StartCrosvm tests that crosvm can start and launch a process.
func StartCrosvm(ctx context.Context, s *testing.State) {
	crosvmParams := new(vm.CrosvmParams)
	crosvmParams.KernelArgs = []string{"init=/bin/bash"}
	componentPath, err := vm.LoadTerminaComponent(ctx)
	if err != nil {
		s.Fatal("Unable to load component: ", err)
	}
	crosvmParams.VMPath = componentPath
	cvm, err := vm.NewCrosvm(ctx, crosvmParams)
	if err != nil {
		s.Fatal("Failed to start crosvm: ", err)
	}
	defer cvm.Close(ctx)

	testing.ContextLog(ctx, "Waiting for VM to boot")
	line, err := cvm.WaitForOutput(regexp.MustCompile("localhost\\b.*#"))
	if err != nil {
		s.Fatal("Didn't get VM prompt: ", err)
	}
	s.Logf("Saw prompt in line %q", line)

	const cmd = "/bin/ls -1 /"
	s.Logf("Running %q", cmd)
	if _, err = io.WriteString(cvm.Stdin(), cmd+"\n"); err != nil {
		s.Fatalf("Failed to write %q command: %v", cmd, err)
	}
	if line, err = cvm.WaitForOutput(regexp.MustCompile("^sbin$")); err != nil {
		s.Errorf("Didn't get expected %q output: %v", cmd, err)
	} else {
		s.Logf("Saw line %q", line)
	}
}
