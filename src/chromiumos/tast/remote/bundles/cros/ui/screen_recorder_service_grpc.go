// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/remote/crosserverutil"
	chromepb "chromiumos/tast/services/cros/browser"
	uipb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenRecorderServiceGRPC,
		Desc:         "Check basic functionalities of ScreenRecorderService",
		Contacts:     []string{"jonfan@google.com", "chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"grpcServerPort"},
	})
}

func ScreenRecorderServiceGRPC(ctx context.Context, s *testing.State) {
	//Connect to gRPC Server on DUT.
	port := 4444
	if portStr, ok := s.Var("grpcServerPort"); ok {
		if portInt, err := strconv.Atoi(portStr); err == nil {
			port = portInt
		}
	}
	uri := s.DUT().HostName()
	hostname := strings.Split(uri, ":")[0]
	cl, err := crosserverutil.Dial(ctx, s.DUT(), hostname, port)
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Verify recording flow with desired path as an input.
	fileName := filepath.Join(s.OutDir(), "record.webm")
	verifyScreenRecording(ctx, cl.Conn, fileName, s)

	// Verify recording flow using temporary path
	verifyScreenRecording(ctx, cl.Conn, "", s)

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
	cs := chromepb.NewChromeServiceClient(conn)
	loginReq := &chromepb.NewRequest{}
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.Close(ctx, &empty.Empty{})

	// Start Recording
	recorderSvc := uipb.NewScreenRecorderServiceClient(conn)
	req := &uipb.StartRequest{FileName: fileName}
	if _, err := recorderSvc.Start(ctx, req); err != nil {
		s.Fatal("Failed to start screen recording: ", err)
	}
	// Verify that Start correctly return an error when there is already a recording in progress
	if _, err := recorderSvc.Start(ctx, req); err == nil {
		s.Fatal("should return an error calling Start() when there is a recording in process: ", err)
	}

	// Performs some actions on the UI like Opening Files App
	uiautoSvc := uipb.NewAutomationServiceClient(conn)
	filesAppShelfButtonFinder := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "ash/ShelfAppButton"}},
			{Value: &uipb.NodeWith_Name{Name: "Files"}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &uipb.WaitUntilExistsRequest{Finder: filesAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to find Files shelf button: ", err)
	}
	if _, err := uiautoSvc.LeftClick(ctx, &uipb.LeftClickRequest{Finder: filesAppShelfButtonFinder}); err != nil {
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
