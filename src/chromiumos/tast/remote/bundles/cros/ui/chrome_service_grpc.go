// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	// "net"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeServiceGRPC,
		Desc:         "Checks we can enable Nearby Share high-vis receving on two DUTs at once",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.ChromeStartupService"},
		Vars:         []string{nearbycommon.KeepStateVar},
	})
}

// ChromeServiceGRPC tests that we can enable Nearby Share on two DUTs in a single test.
func ChromeServiceGRPC(ctx context.Context, s *testing.State) {
	port := 4444
	uri := s.DUT().HostName()
	hostname := strings.Split(uri, ":")[0]
	testing.ContextLogf(ctx, "hostname: %s", hostname)

	// Start CrOS server
	if err := startCrosServer(ctx, s, port); err != nil {
		s.Fatal("Failed to Start CrOS process: ", err)
	}
	defer stopCrosServer(ctx, s, port)

	// Setup gRPC channel
	testing.ContextLogf(ctx, "START grpc.Dial")
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", hostname, port), grpc.WithInsecure())
	if err != nil {
		s.Fatal("Fail to Setup gRPC channel: ", err)
	}
	defer func() {
		testing.ContextLogf(ctx, "DEFER grpc connect closed")
		conn.Close()
	}()

	//Do call some grpc method

	// Connect to the Nearby Share Service so we can execute local code on the DUT.
	testing.ContextLogf(ctx, "START Chrome")
	cs := ui.NewChromeStartupServiceClient(conn)
	loginReq := &ui.NewChromeLoginRequest{}
	if _, err := cs.NewChromeLogin(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer func() {
		testing.ContextLogf(ctx, "DEFER Close Chrome")
		cs.CloseChrome(ctx, &empty.Empty{})
	}()
	// defer cs.CloseChrome(ctx, &empty.Empty{})

	ks := inputs.NewKeyboardServiceClient(conn)

	typeReq := &inputs.TypeRequest{
		Key: "abc",
	}

	if _, err := ks.Type(ctx, typeReq); err != nil {
		s.Fatal("Failed to Type ", err)
	}

}

func startCrosServer(ctx context.Context, s *testing.State, port int) error {
	sshConn := s.DUT().Conn()
	// TODO(jonfan): Refactor into helper class
	args := []string{"-rpctcp", "-port", strconv.Itoa(port), ">>", "/tmp/nearby1.log", "2>&1"}
	testing.ContextLogf(ctx, "Start CrOS server with parameters: %v", args)

	// Try to kill any process using the desired port
	stopCrosServer(ctx, s, port)

	// Open up TCP port for incoming traffic
	ipTableArgs := []string{"-A", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-j", "ACCEPT"}
	if err := sshConn.CommandContext(ctx, "iptables", ipTableArgs...).Run(); err != nil {
		s.Fatal(fmt.Sprintf("Failed to open up TCP port: %d for incoming traffic", port), err)
	}

	// Start CrOS server as a separate process
	output, _ := os.Create(filepath.Join(s.OutDir(), "cros_server.log"))
	cmd := sshConn.CommandContext(ctx, "/usr/local/libexec/tast/bundles/local_pushed/cros", args...)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		s.Fatal(fmt.Sprintf("Failed to Start CrOS Server with parameter: %v", args), err)
	}
	return nil
}

func stopCrosServer(ctx context.Context, s *testing.State, port int) error {
	testing.ContextLogf(ctx, "DEFER STOPCROSSERVER")
	sshConn := s.DUT().Conn()

	// Get the pid of process using the desired port
	out, err := sshConn.CommandContext(ctx, "lsof", "-t", fmt.Sprintf("-i:%d", port)).CombinedOutput()
	if err != nil {
		return err
	}
	pidStr := strings.TrimRight(string(out), "\r\n")
	if pidStr != "" {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return err
		} else {
			testing.ContextLogf(ctx, "Kill CrOS process pid: %d port: %d", pid, port)
			if out, err := sshConn.CommandContext(ctx, "kill", "-9", strconv.Itoa(pid)).CombinedOutput(); err != nil {
				s.Fatal(fmt.Sprintf("Failed to kill CrOS process pid: %d port: %d StdOut: %v", pid, port, out), err)
			}
		}
	}
	return nil
}

//TODO(jonfan): Logging has to wait till logging.proto is moved from internal to public package
// func setupLogging (ctx context.Context) error {
//Forward the logs from side channnel to standard out.
// cl := protocol.NewLoggingClient(conn)
// waitc := make(chan struct{})
// go func() {
// 	for {
// 		in, err := stream.Recv()
// 		if err == io.EOF {
// 			// read done.
// 			close(waitc)
// 			return
// 		}
// 		if err != nil {
// 			testing.ContextLogf("Failed reading from stream %v", err)
// 		}
// 		testing.ContextLogf("CrOS Server Log: (%d) %s", in.entry.seq, in.entry.msg)
// 	}
// }()
// <-waitc

// }
