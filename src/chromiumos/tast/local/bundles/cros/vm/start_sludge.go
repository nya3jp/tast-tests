// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"time"

	"github.com/mdlayher/vsock"
	"google.golang.org/grpc"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	vm_tools "chromiumos/vm_tools/vm_rpc"
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

type startupListenerServer struct {
	rec chan (bool)
}

func (s *startupListenerServer) VmReady(ctx context.Context, msg *vm_tools.EmptyMessage) (*vm_tools.EmptyMessage, error) {
	s.rec <- true
	return &vm_tools.EmptyMessage{}, nil
}

func StartSludge(ctx context.Context, s *testing.State) {
	const (
		wilcoVMJob         = "wilco_dtc"
		wilcoVMCID         = "512"
		wilcoVMStartupPort = 7788
	)

	grpcServer := grpc.NewServer()
	startupServer := startupListenerServer{}
	startupServer.rec = make(chan bool, 1)
	vm_tools.RegisterStartupListenerServer(grpcServer, &startupServer)

	lis, err := vsock.Listen(wilcoVMStartupPort)
	defer lis.Close()
	if err != nil {
		s.Fatal("Unable to listen on vsock port: ", err)
	}

	s.Log("Listening on port: ", lis.Addr())
	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	s.Log("Restarting Wilco DTC daemon")
	if err := upstart.RestartJob(ctx, wilcoVMJob); err != nil {
		s.Fatal("Wilco DTC process could not start: ", err)
	}

	startCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	select {
	case <-startupServer.rec:
		s.Log("Wilco DTC Started")
	case <-startCtx.Done():
		s.Fatal("Timed out waiting for Wilco DTC to start")
	}

	for _, name := range []string{"ddv", "sa"} {
		s.Logf("Checking %v process", name)

		cmd := testexec.CommandContext(ctx,
			"vsh", "--cid="+wilcoVMCID, "--", "pgrep", name)
		// Add a dummy buffer for stdin to force allocating a pipe. vsh uses
		// epoll internally and generates a warning (EPERM) if stdin is /dev/null.
		cmd.Stdin = &bytes.Buffer{}

		out, err := cmd.Output()
		if err != nil {
			s.Errorf("Process %v not found: %v", name, err)
		}

		s.Logf("Process %v started with PID %s", name, bytes.TrimSpace(out))
	}

	s.Log("Stopping Wilco DTC daemon")
	if err := upstart.StopJob(ctx, wilcoVMJob); err != nil {
		s.Error("Unable to stop Wilco DTC daemon")
	}
}
