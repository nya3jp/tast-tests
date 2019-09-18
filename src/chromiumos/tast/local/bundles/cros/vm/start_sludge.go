// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartSludge,
		Desc:         "Starts a new instance of sludge VM and tests that the DTC binaries are running",
		Contacts:     []string{"tbegin@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
	})
}

func StartSludge(ctx context.Context, s *testing.State) {
	const (
		storagePath = "/opt/dtc/storage"
		diagPath    = "/opt/dtc/diagnostics"
	)

	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	vm.StartSludge(startCtx, true)
	defer vm.StopSludge(ctx)

	for _, name := range []string{"ddv", "sa"} {
		s.Logf("Checking %v process", name)

		out, err := vm.SendVshCommand(ctx, vm.WilcoVMCID, "pgrep", name)
		if err != nil {
			s.Errorf("Process %v not found: %v", name, err)
		} else {
			s.Logf("Process %v started with PID %s", name, bytes.TrimSpace(out))
		}
	}

	for _, path := range []string{storagePath, diagPath} {
		s.Logf("Checking %v path", path)

		_, err := vm.SendVshCommand(ctx, vm.WilcoVMCID, "test", "-d", path)
		if err != nil {
			s.Errorf("Path %v does not exist inside VM: %v", path, err)
		} else {
			s.Logf("Path %v is mounted inside VM", path)
		}
	}
}
