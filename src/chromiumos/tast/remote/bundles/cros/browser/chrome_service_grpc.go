// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package browser

import (
	"context"
	"fmt"
	"strconv"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/remote/crosserverutil"
	pb "chromiumos/tast/services/cros/browser"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeServiceGRPC,
		Desc:         "Check basic functionality of ChromeService",
		Contacts:     []string{"jonfan@google.com", "chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"grpcServerPort"},
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
		Params: []testing.Param{{
			Name: "default_login",
			Val:  pb.NewRequest{},
		}, {
			Name: "fake_login",
			Val: pb.NewRequest{
				LoginMode: pb.LoginMode_FAKE_LOGIN,
				Credentials: &pb.NewRequest_Credentials{
					Username: "testuser@gmail.com",
					Password: "testpass",
				},
				TryReuseSession: false,
				KeepState:       false,
				EnableFeatures:  []string{"GwpAsanMalloc", "GwpAsanPartitionAlloc"},
				ExtraArgs:       []string{"--enable-logging"},
			},
		}, {
			Name: "try_reuse_sessions",
			Val: pb.NewRequest{
				LoginMode: pb.LoginMode_FAKE_LOGIN,
				Credentials: &pb.NewRequest_Credentials{
					Username: "testuser@gmail.com",
					Password: "testpass",
				},
				TryReuseSession: true,
				KeepState:       true,
				// Requesting the same features and args in order to reuse the same session
				EnableFeatures: []string{"GwpAsanMalloc", "GwpAsanPartitionAlloc"},
				ExtraArgs:      []string{"--enable-logging"},
			},
		}},
	})
}

// ChromeServiceGRPC tests ChromeService functionalities for managing chrome lifecycle.
func ChromeServiceGRPC(ctx context.Context, s *testing.State) {
	grpcServerPort := 4445
	if portStr, ok := s.Var("grpcServerPort"); ok {
		if portInt, err := strconv.Atoi(portStr); err == nil {
			grpcServerPort = portInt
		}
	}

	//Setup forwarder to expose remote gRPC server port through SSH connection
	conn := s.DUT().Conn()
	addr := fmt.Sprintf("localhost:%d", grpcServerPort)
	forwarder, err := conn.ForwardLocalToRemote("tcp", addr, addr,
		func(err error) { testing.ContextLog(ctx, "Port forwarding error: ", err) })
	if err != nil {
		s.Fatal("Failed to setup port forwarding: ", err)
	}
	defer func() {
		if err = forwarder.Close(); err != nil {
			s.Fatal("Failed to close port forwarding")
		}
	}()

	//Connect to TCP based gRPC Server on DUT.
	cl, err := crosserverutil.Dial(ctx, s.DUT(), "localhost", grpcServerPort)
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Start Chrome on DUT
	cs := pb.NewChromeServiceClient(cl.Conn)
	loginReq := s.Param().(pb.NewRequest)
	if _, err := cs.New(ctx, &loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Close Chrome on DUT
	if _, err := cs.Close(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to close Chrome: ", err)
	}
}
