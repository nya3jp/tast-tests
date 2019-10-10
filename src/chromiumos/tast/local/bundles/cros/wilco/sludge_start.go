// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/wilco/wvm"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SludgeStart,
		Desc: "Starts a new instance of sludge VM and tests that the DTC binaries are running",
		Contacts: []string{
			"tbegin@chromium.org",
			"cros-containers-dev@google.com",
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
	})
}

func SludgeStart(ctx context.Context, s *testing.State) {
	const (
		storagePath = "/opt/dtc/storage"
		diagPath    = "/opt/dtc/diagnostics"
	)

	// Shorten the total context by 5 seconds to allow for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Expect the VM to start within 5 seconds.
	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := wvm.StartSludge(startCtx, wvm.DefaultSludgeConfig()); err != nil {
		s.Fatal("Unable to start sludge VM: ", err)
	}
	defer wvm.StopSludge(cleanupCtx)

	// Wait for the ddv dbus service to be up and running before continuing the
	// test.
	if err := wvm.WaitForDDVDbus(startCtx); err != nil {
		s.Fatal("DDV dbus service not available: ", err)
	}

	for _, name := range []string{"ddv", "ddtm", "sa"} {
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
