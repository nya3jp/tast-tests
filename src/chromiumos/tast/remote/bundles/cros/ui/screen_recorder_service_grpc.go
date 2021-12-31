// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/remote/crosserverutil"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenRecorderServiceGRPC,
		Desc:         "Check basic functionalities of ScreenRecorderService",
		Contacts:     []string{"jonfan@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"grpcServerPort"},
		Params: []testing.Param{{
			Name: "given_path",
			Val:  "record.webm",
		}, {
			Name: "temp_path",
			Val:  "",
		}},
	})
}

func ScreenRecorderServiceGRPC(ctx context.Context, s *testing.State) {
	grpcServerPort := crosserverutil.DefaultGRPCServerPort
	if portStr, ok := s.Var("grpcServerPort"); ok {
		if portInt, err := strconv.Atoi(portStr); err == nil {
			grpcServerPort = portInt
		}
	}

	// Connect to TCP based gRPC Server on DUT.
	cl, err := crosserverutil.Dial(ctx, s.DUT(), "localhost", grpcServerPort, true)
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	fileName := s.Param().(string)
	if fileName != "" {
		// Append test output directory to file name
		fileName = filepath.Join(s.OutDir(), fileName)
	}
	verifyScreenRecording(ctx, cl.Conn, fileName, s)
}

func verifyScreenRecording(ctx context.Context, conn *grpc.ClientConn, fileName string, s *testing.State) {
	// Make a screen recording
	actualFileName := makeScreenRecording(ctx, conn, fileName, s)

	// verify if the file is saved in the desired path
	if fileName != "" && fileName != actualFileName {
		s.Fatalf("Requested file name and actual file name mismatch. requested: %v  actual: %v ", fileName, actualFileName)
	}

	//Verify that screen recording is on the file system
	if _, err := s.DUT().Conn().CommandContext(ctx, "[", "-e", actualFileName, "]").CombinedOutput(); err != nil {
		s.Fatal(fmt.Sprintf("Failed to find recording: %v", fileName), err)
	}
}

func makeScreenRecording(ctx context.Context, conn *grpc.ClientConn, fileName string, s *testing.State) string {
	// Start Chrome on the DUT.
	cs := pb.NewChromeServiceClient(conn)
	loginReq := &pb.NewRequest{}
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.Close(ctx, &empty.Empty{})

	// Start Recording
	recorderSvc := pb.NewScreenRecorderServiceClient(conn)
	req := &pb.StartRequest{FileName: fileName}
	if _, err := recorderSvc.Start(ctx, req); err != nil {
		s.Fatal("Failed to start screen recording: ", err)
	}
	// Verify that Start correctly return an error when there is already a recording in progress
	if _, err := recorderSvc.Start(ctx, req); err == nil {
		s.Fatal("should return an error calling Start() when there is a recording in process: ", err)
	}

	// Performs some actions on the UI like Opening Files App
	uiautoSvc := pb.NewAutomationServiceClient(conn)
	filesAppShelfButtonFinder := &pb.Finder{
		NodeWiths: []*pb.NodeWith{
			{Value: &pb.NodeWith_HasClass{HasClass: "ash/ShelfAppButton"}},
			{Value: &pb.NodeWith_Name{Name: "Files"}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &pb.WaitUntilExistsRequest{Finder: filesAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to find Files shelf button: ", err)
	}
	if _, err := uiautoSvc.LeftClick(ctx, &pb.LeftClickRequest{Finder: filesAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to click on Files app: ", err)
	}
	testing.Sleep(ctx, time.Second)

	// Stopping and saving screen recording
	resp, err := recorderSvc.Stop(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to stop and save recording: ", err)
	}
	// Verify that StopSaveRelease returns an error when this is no recording in progress
	if _, err := recorderSvc.Stop(ctx, &empty.Empty{}); err == nil {
		s.Fatal("should return an error when there is no recording in process")
	}

	return resp.FileName
}
