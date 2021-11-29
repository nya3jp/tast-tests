// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"

	"chromiumos/tast/remote/crosserverutil"
	chromepb "chromiumos/tast/services/cros/chrome"
	uipb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutomationServiceGRPC,
		Desc:         "Check basic functionality of UI Automation Service",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"grpcServerPort"},
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
	})
}

// AutomationServiceGRPC tests that we can enable Nearby Share on two DUTs in a single test.
func AutomationServiceGRPC(ctx context.Context, s *testing.State) {
	grpcServerPort := 4444
	if portStr, ok := s.Var("grpcServerPort"); ok {
		if portInt, err := strconv.Atoi(portStr); err == nil {
			grpcServerPort = portInt
		}
	}
	uri := s.DUT().HostName()
	hostname := strings.Split(uri, ":")[0]
	testing.ContextLogf(ctx, "hostname: %s", hostname)

	// Start CrOS server
	sshConn := s.DUT().Conn()
	if err := crosserverutil.StartCrosServer(ctx, sshConn, grpcServerPort); err != nil {
		s.Fatal("Failed to Start CrOS process: ", err)
	}
	defer crosserverutil.StopCrosServer(ctx, sshConn, grpcServerPort)

	// Setup gRPC channel
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", hostname, grpcServerPort), grpc.WithInsecure())
	if err != nil {
		s.Fatal("Fail to Setup gRPC channel: ", err)
	}
	defer conn.Close()

	// Start Chrome on the DUT.
	cs := chromepb.NewChromeServiceClient(conn)
	loginReq := &chromepb.NewRequest{}
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.Close(ctx, &empty.Empty{})

	uiautoSvc := uipb.NewAutomationServiceClient(conn)

	//Verify that both chrome app and files app buttons are there after initial loading
	filesAppShelfButtonFinder := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "ash/ShelfAppButton"}},
			{Value: &uipb.NodeWith_Name{Name: "Files"}},
		},
	}

	chromeAppShelfButtonFinder := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "ash/ShelfAppButton"}},
			{Value: &uipb.NodeWith_Name{Name: "Google Chrome"}},
		},
	}

	if _, err := uiautoSvc.WaitUntilExists(ctx, &uipb.WaitUntilExistsRequest{Finder: chromeAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to find Chrome shelf button: ", err)
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &uipb.WaitUntilExistsRequest{Finder: filesAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to find Files shelf button: ", err)
	}

	// Open Files App and close it by clicking the cross on the window
	if _, err := uiautoSvc.LeftClick(ctx, &uipb.LeftClickRequest{Finder: filesAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to click on Files app: ", err)
	}

	filesAppWindowFinder := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "Widget"}},
			{Value: &uipb.NodeWith_Name{Name: "Files - My files"}},
		},
	}
	//Wait for search button to show up
	filesAppSearchButtonFinder := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "icon-button"}},
			{Value: &uipb.NodeWith_Name{Name: "Search"}},
			{Value: &uipb.NodeWith_Focusable{}},
			{Value: &uipb.NodeWith_Ancestor{
				Ancestor: filesAppWindowFinder,
			}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &uipb.WaitUntilExistsRequest{Finder: filesAppSearchButtonFinder}); err != nil {
		s.Fatal("Failed to wait for files app search button: ", err)
	}

	// Check to see if the search button is there.
	if res, err := uiautoSvc.IsNodeFound(ctx, &uipb.IsNodeFoundRequest{Finder: filesAppSearchButtonFinder}); err != nil || !res.Found {
		s.Fatal("Failed to find node for files app search button: ", err)
	}

	// Get and compare Info for the search button
	infoResp, err := uiautoSvc.Info(ctx, &uipb.InfoRequest{Finder: filesAppSearchButtonFinder})
	if err != nil {
		s.Fatal("Failed to get node info for files app search button: ", err)
	}
	want := &uipb.NodeInfo{
		ClassName: "icon-button menu-button",
		Name:      "Search",
	}

	if diff := cmp.Diff(infoResp.NodeInfo, want, protocmp.Transform()); diff != "" {
		s.Fatalf("NodeInfo mismatch for search button (-got +want):%s", diff)
	}

	// Close Files app by clicking the cross on the window
	fileAppCloseButtonFinder := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "FrameCaptionButton"}},
			{Value: &uipb.NodeWith_Name{Name: "Close"}},
			{Value: &uipb.NodeWith_Focusable{}},
			{Value: &uipb.NodeWith_Ancestor{
				Ancestor: filesAppWindowFinder,
			}},
		},
	}
	if _, err := uiautoSvc.LeftClick(ctx, &uipb.LeftClickRequest{Finder: fileAppCloseButtonFinder}); err != nil {
		s.Fatal("Failed to click on Files app: ", err)
	}

	// Verify that Files App search button is gone
	if res, err := uiautoSvc.IsNodeFound(ctx, &uipb.IsNodeFoundRequest{Finder: filesAppSearchButtonFinder}); err != nil || res.Found {
		s.Fatal("Search button should have been gone: ", err)
	}

	// Open chrome app and close it from the context menu in the shelf
	if _, err := uiautoSvc.LeftClick(ctx, &uipb.LeftClickRequest{Finder: chromeAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to click on Chrome app: ", err)
	}

	if _, err := uiautoSvc.RightClick(ctx, &uipb.RightClickRequest{Finder: chromeAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to click on Chrome app: ", err)
	}

	closeButton := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_Name{Name: "Close"}},
			{Value: &uipb.NodeWith_Role{Role: uipb.Role_ROLE_MENU_ITEM}},
		},
	}

	if _, err := uiautoSvc.WaitUntilExists(ctx, &uipb.WaitUntilExistsRequest{Finder: closeButton}); err != nil {
		s.Fatal("Failed to wait for close button from context menu: ", err)
	}

	if _, err := uiautoSvc.LeftClick(ctx, &uipb.LeftClickRequest{Finder: closeButton}); err != nil {
		s.Fatal("Failed to close Chrome app from context menu: ", err)
	}
}
