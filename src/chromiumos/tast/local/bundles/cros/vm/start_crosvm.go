// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"time"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartCrosVM,
		Desc:         "Checks that crosvm can start and stop.",
		Timeout:      300 * time.Second,
		SoftwareDeps: []string{"vm_host"},
	})
}

func StartCrosVM(s *testing.State) {
	kernel_args := []string{"-p", "init=/bin/bash"}
	cvm, err := vm.StartVM(s.Context(), nil, kernel_args)
	if err != nil {
		s.Fatal("Failed to start crosvm: ", err)
	}

	testing.ContextLog(s.Context(), "Waiting for VM to boot")
	var got_prompt bool
	got_prompt, _, err = cvm.WaitPrompt(s.Context())
	if err != nil {
		s.Fatal("Failed to get initial prompt: ", err)
	}
	if !got_prompt {
		s.Fatal("Failed to get initial prompt")
	}
	testing.ContextLog(s.Context(), "VM booted")

	var output *bytes.Buffer
	output, err = cvm.RunCommand(s.Context(), "/bin/ls")
	if err != nil {
		s.Fatal("Failed to run command: ", err)
	}

	if !bytes.Contains(output.Bytes(), []byte("sbin")) {
		s.Fatal("No sbin directory found in the VM")
	}

	err = cvm.StopVm()
	if err != nil {
		s.Fatal("Failed to stop crosvm: ", err)
	}
}
