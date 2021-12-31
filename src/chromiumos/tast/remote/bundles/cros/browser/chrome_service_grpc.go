// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package browser

import (
	"context"
	"strconv"
	"strings"

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
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"grpcServerPort"},
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
	})
}

// ChromeServiceGRPC tests ChromeService functionalities for managing chrome lifecycle.
func ChromeServiceGRPC(ctx context.Context, s *testing.State) {
	//Connect to gRPC Server on DUT.
	hostname, port := getGRPCHostnamePort(ctx, s)
	cl, err := crosserverutil.Dial(ctx, s.DUT(), hostname, port)
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Start Chrome on DUT
	cs := pb.NewChromeServiceClient(cl.Conn)
	loginReq := &pb.NewRequest{
		LoginMode: pb.LoginMode_FAKE_LOGIN,
		Credentials: &pb.NewRequest_Credentials{
			Username: "testuser@gmail.com",
			Password: "testpass",
		},
		TryReuseSession: false,
		KeepState:       true,
		EnableFeatures:  []string{"GwpAsanMalloc", "GwpAsanPartitionAlloc"},
		ExtraArgs:       []string{"--enable-logging"},
	}
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	// Close Chrome on DUT
	if _, err := cs.Close(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to close Chrome: ", err)
	}
}

func getGRPCHostnamePort(ctx context.Context, s *testing.State) (string, int) {
	grpcServerPort := 4444
	if portStr, ok := s.Var("grpcServerPort"); ok {
		if portInt, err := strconv.Atoi(portStr); err == nil {
			grpcServerPort = portInt
		}
	}
	uri := s.DUT().HostName()
	hostname := strings.Split(uri, ":")[0]
	return hostname, grpcServerPort
}
