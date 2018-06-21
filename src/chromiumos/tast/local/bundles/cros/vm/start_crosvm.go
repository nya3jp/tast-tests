// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartCrosVM,
		Desc:         "Checks that crosvm can start and stop",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host"},
	})
}

func StartCrosVM(s *testing.State) {
	kernelArgs := []string{"-p", "init=/bin/bash"}
	cvm, err := vm.StartVM(s.Context(), "", kernelArgs)
	if err != nil {
		s.Fatal("Failed to start crosvm: ", err)
	}
	defer cvm.Close(s.Context())

	testing.ContextLog(s.Context(), "Waiting for VM to boot")
	if gotPrompt, _, err := cvm.WaitPrompt(s.Context()); err != nil {
		s.Fatal("Failed to get initial prompt: ", err)
	} else if !gotPrompt {
		s.Fatal("Failed to get initial prompt")
	}
	testing.ContextLog(s.Context(), "VM booted")

	output, err := cvm.RunCommand(s.Context(), "/bin/ls /")
	if err != nil {
		s.Fatal("Failed to run command: ", err)
	}

	if !bytes.Contains(output.Bytes(), []byte("sbin")) {
		s.Fatal("No sbin directory found in the VM")
	}
}
