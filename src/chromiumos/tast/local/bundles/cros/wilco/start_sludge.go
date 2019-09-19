// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"bytes"
	"context"
	"time"

	wvm "chromiumos/tast/local/bundles/cros/wilco/vm"
	"chromiumos/tast/local/testexec"
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

	if err := wvm.StartSludge(startCtx, true); err != nil {
		s.Fatal("Unable to start sludge VM: ", err)
	}
	defer wvm.StopSludge(ctx)

	for _, name := range []string{"ddv", "sa"} {
		cmd := vm.CreateVSHCommand(ctx, wvm.WilcoVMCID, "pgrep", name)
		if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
			s.Errorf("Process %v not found: %v", name, err)
		} else {
			s.Logf("Process %v started with PID %s", name, bytes.TrimSpace(out))
		}
	}

	for _, path := range []string{storagePath, diagPath} {
		cmd := vm.CreateVSHCommand(ctx, wvm.WilcoVMCID, "test", "-d", path)
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Path %v does not exist inside VM: %v", path, err)
		} else {
			s.Logf("Path %v is mounted inside VM", path)
		}
	}
}
