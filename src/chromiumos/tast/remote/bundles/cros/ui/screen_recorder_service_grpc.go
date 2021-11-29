// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/remote/bundles/cros/crosserverutil"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenRecorderServiceGRPC,
		Desc:         "Check basic functionality of UI Automation Service",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.ChromeStartupService"},
		Vars:         []string{},
		//TODO(jonfan): only clamshell mode
	})
}

// ScreenRecorderServiceGRPC tests that we can enable Nearby Share on two DUTs in a single test.
func ScreenRecorderServiceGRPC(ctx context.Context, s *testing.State) {
	//TODO(jonfan): move to tast test parameter with default
	port := 4444
	uri := s.DUT().HostName()
	hostname := strings.Split(uri, ":")[0]

	// Start CrOS server
	if err := crosserverutil.StartCrosServer(ctx, s, port); err != nil {
		s.Fatal("Failed to Start CrOS process: ", err)
	}
	defer crosserverutil.StopCrosServer(ctx, s, port)

	// Setup gRPC channel
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", hostname, port), grpc.WithInsecure())
	if err != nil {
		s.Fatal("Fail to Setup gRPC channel: ", err)
	}
	defer conn.Close()

	// Make a screen recording
	fileName := filepath.Join(s.OutDir(), "record.webm")
	makeScreenRecording(ctx, conn, fileName, s)

	//Verify that screen recording is there
	sshConn := s.DUT().Conn()
	if _, err := sshConn.CommandContext(ctx, "[", "-e", fileName, "]").CombinedOutput(); err != nil {
		s.Fatal(fmt.Sprintf("Failed to find recording: %v", fileName), err)
	}
}

func makeScreenRecording(ctx context.Context, conn *grpc.ClientConn, fileName string, s *testing.State) {

	// Connect to the Nearby Share Service so we can execute local code on the DUT.
	cs := ui.NewChromeStartupServiceClient(conn)
	loginReq := &ui.NewChromeLoginRequest{}
	if _, err := cs.NewChromeLogin(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.CloseChrome(ctx, &empty.Empty{})

	// Start Recording
	recorderSvc := ui.NewScreenRecorderServiceClient(conn)
	if _, err := recorderSvc.Start(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start screen recording: ", err)
	}
	// Verify that Start correctly return an error when there is already a recording in progress
	if _, err := recorderSvc.Start(ctx, &empty.Empty{}); err == nil {
		s.Fatal("should return an error calling Start() when there is a recording in process: ", err)
	}

	defer func() {
		testing.ContextLogf(ctx, "Saving video to : %s", fileName)
		req := &ui.StopSaveReleaseRequest{FileName: fileName}
		if _, err := recorderSvc.StopSaveRelease(ctx, req); err != nil {
			s.Fatal("Failed to stop and save recording: ", err)
		}
		// Verify that StopSaveRelease returns an error when this is no recording in progress
		if _, err := recorderSvc.StopSaveRelease(ctx, req); err == nil {
			s.Fatal("should return an error when there is no recording in process ", err)
		}
	}()

	uiautoSvc := ui.NewAutomationServiceClient(conn)

	filesAppShelfButtonFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "ash/ShelfAppButton"}},
			{Value: &ui.NodeWith_Name{Name: "Files"}},
		},
	}

	if _, err := uiautoSvc.WaitUntilExists(ctx, &ui.WaitUntilExistsRequest{Finder: filesAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to find Files shelf button: ", err)
	}

	// Open Files App and close it by clicking the cross on the window
	if _, err := uiautoSvc.LeftClick(ctx, &ui.LeftClickRequest{Finder: filesAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to click on Files app: ", err)
	}

	filesAppWindowFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "NativeAppWindowViews"}},
			{Value: &ui.NodeWith_Name{Name: "Files - My files"}},
		},
	}

	testing.Sleep(ctx, time.Second)

	// Close Files app by clicking the cross on the window
	fileAppCloseButtonFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "FrameCaptionButton"}},
			{Value: &ui.NodeWith_Name{Name: "Close"}},
			{Value: &ui.NodeWith_Focusable{}},
			{Value: &ui.NodeWith_Ancestor{
				Ancestor: filesAppWindowFinder,
			}},
		},
	}

	if _, err := uiautoSvc.LeftClick(ctx, &ui.LeftClickRequest{Finder: fileAppCloseButtonFinder}); err != nil {
		s.Fatal("Failed to click on Files app: ", err)
	}

	testing.Sleep(ctx, time.Second)
}
