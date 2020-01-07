// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: VMStart,
		Desc: "Starts a new instance of the Wilco DTC VM and tests that the DTC binaries are running",
		Contacts: []string{
			"tbegin@chromium.org",
			"cros-containers-dev@google.com",
			"chromeos-wilco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"vm_host", "wilco"},
	})
}

func VMStart(ctx context.Context, s *testing.State) {
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

	if err := wilco.StartVM(startCtx, wilco.DefaultVMConfig()); err != nil {
		s.Fatal("Unable to start Wilco DTC VM: ", err)
	}
	defer wilco.StopVM(cleanupCtx)

	// Wait for the ddv dbus service to be up and running before continuing the
	// test.
	if err := wilco.WaitForDDVDBus(startCtx); err != nil {
		s.Fatal("DDV dbus service not available: ", err)
	}

	for _, name := range []string{"ddv", "ddtm", "sa"} {
		cmd := vm.CreateVSHCommand(ctx, wilco.WilcoVMCID, "pgrep", name)
		if out, err := cmd.Output(testexec.DumpLogOnError); err != nil {
			s.Errorf("Process %v not found: %v", name, err)
		} else {
			s.Logf("Process %v started with PID %s", name, bytes.TrimSpace(out))
		}
	}

	for _, path := range []string{storagePath, diagPath} {
		cmd := vm.CreateVSHCommand(ctx, wilco.WilcoVMCID, "mountpoint", path)
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("Path %v is not mounted inside VM: %v", path, err)
		} else {
			s.Logf("Path %v is mounted inside VM", path)
		}
	}
}
