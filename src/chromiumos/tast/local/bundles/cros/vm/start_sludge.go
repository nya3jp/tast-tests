// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
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
		// Const values from /etc/init/wilco_dtc.conf on device
		wilcoVMJob         = "wilco_dtc"
		wilcoVMCID         = "512"
		wilcoVMStartupPort = 7788
	)

	// Load the vhost-vsock module
	if err := testexec.CommandContext(ctx, "modprobe", "-q", "vhost-vsock").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Unable to load vhost-vsock module: ", err)
	}

	server, err := vm.NewStartupListenerServer(wilcoVMStartupPort)
	if err != nil {
		s.Fatal("Unable to start VM startup listener gRPC server: ", err)
	}

	if err := server.Start(); err != nil {
		s.Fatal("Unable to start listening server: ", err)
	}
	defer server.Stop()

	s.Log("Restarting Wilco DTC daemon")
	if err := upstart.RestartJob(ctx, wilcoVMJob); err != nil {
		s.Fatal("Wilco DTC process could not start: ", err)
	}

	startCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := server.WaitReady(startCtx); err != nil {
		s.Fatal("Error waiting for Wilco DTC VM to start: ", err)
	}

	for _, name := range []string{"ddv", "sa"} {
		s.Logf("Checking %v process", name)

		cmd := testexec.CommandContext(ctx,
			"vsh", "--cid="+wilcoVMCID, "--", "pgrep", name)
		// Add a dummy buffer for stdin to force allocating a pipe. vsh uses
		// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
		cmd.Stdin = &bytes.Buffer{}

		out, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			s.Errorf("Process %v not found: %v", name, err)
		} else {
			s.Logf("Process %v started with PID %s", name, bytes.TrimSpace(out))
		}
	}

	s.Log("Stopping Wilco DTC daemon")
	if err := upstart.StopJob(ctx, wilcoVMJob); err != nil {
		s.Error("Unable to stop Wilco DTC daemon")
	}
}
