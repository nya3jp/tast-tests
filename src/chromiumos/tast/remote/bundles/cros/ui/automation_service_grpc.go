// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strconv"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"

	"chromiumos/tast/remote/crosserverutil"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutomationServiceGRPC,
		Desc:         "Check basic functionalities of UI AutomationService",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"grpcServerPort"},
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
	})
}

// AutomationServiceGRPC tests basic functionalities of UI AutomationService.
func AutomationServiceGRPC(ctx context.Context, s *testing.State) {
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

	// Start Chrome on the DUT.
	cs := pb.NewChromeServiceClient(cl.Conn)
	loginReq := &pb.NewRequest{}
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.Close(ctx, &empty.Empty{})

	uiautoSvc := pb.NewAutomationServiceClient(cl.Conn)

	// Wait until both chrome app and files app buttons show up after initial login.
	filesAppShelfButtonFinder := &pb.Finder{
		NodeWiths: []*pb.NodeWith{
			{Value: &pb.NodeWith_HasClass{HasClass: "ash/ShelfAppButton"}},
			{Value: &pb.NodeWith_Name{Name: "Files"}},
		},
	}

	chromeAppShelfButtonFinder := &pb.Finder{
		NodeWiths: []*pb.NodeWith{
			{Value: &pb.NodeWith_HasClass{HasClass: "ash/ShelfAppButton"}},
			{Value: &pb.NodeWith_Name{Name: "Google Chrome"}},
		},
	}

	if _, err := uiautoSvc.WaitUntilExists(ctx, &pb.WaitUntilExistsRequest{Finder: chromeAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to find Chrome shelf button: ", err)
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &pb.WaitUntilExistsRequest{Finder: filesAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to find Files shelf button: ", err)
	}

	// Open Files App and close it by clicking the cross on the window.
	if _, err := uiautoSvc.LeftClick(ctx, &pb.LeftClickRequest{Finder: filesAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to click on Files app: ", err)
	}

	// Wait for search button on the files app to show up.
	filesAppWindowFinder := &pb.Finder{
		NodeWiths: []*pb.NodeWith{
			{Value: &pb.NodeWith_HasClass{HasClass: "Widget"}},
			{Value: &pb.NodeWith_Name{Name: "Files - My files"}},
		},
	}
	filesAppSearchButtonFinder := &pb.Finder{
		NodeWiths: []*pb.NodeWith{
			{Value: &pb.NodeWith_HasClass{HasClass: "icon-button"}},
			{Value: &pb.NodeWith_Name{Name: "Search"}},
			{Value: &pb.NodeWith_Focusable{}},
			{Value: &pb.NodeWith_Ancestor{
				Ancestor: filesAppWindowFinder,
			}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &pb.WaitUntilExistsRequest{Finder: filesAppSearchButtonFinder}); err != nil {
		s.Fatal("Failed to wait for files app search button: ", err)
	}

	// Test IsNodeFound to confirm that the search button is there.
	if res, err := uiautoSvc.IsNodeFound(ctx, &pb.IsNodeFoundRequest{Finder: filesAppSearchButtonFinder}); err != nil || !res.Found {
		s.Fatal("Failed to find node for files app search button: ", err)
	}

	// Get and verify Info for the search button.
	filesAppSearchButtonInfo, err := uiautoSvc.Info(ctx, &pb.InfoRequest{Finder: filesAppSearchButtonFinder})
	if err != nil {
		s.Fatal("Failed to get node info for files app search button: ", err)
	}
	// skip comparing Location and HtmlAttributes as those are form factor specific and likely to make the test not stable.
	filesAppSearchButtonInfo.NodeInfo.Location = nil
	filesAppSearchButtonInfo.NodeInfo.HtmlAttributes = nil
	want := &pb.NodeInfo{
		ClassName:   "icon-button menu-button",
		Name:        "Search",
		Restriction: pb.Restriction_RESTRICTION_NONE,
		Role:        pb.Role_ROLE_BUTTON,
		State:       map[string]bool{"focusable": true},
	}

	if diff := cmp.Diff(filesAppSearchButtonInfo.NodeInfo, want, protocmp.Transform()); diff != "" {
		s.Fatalf("NodeInfo mismatch for search button (-got +want):%s", diff)
	}

	// Close Files app by clicking the close button on the window.
	// To test the functionality of Info and MouseClickAtLocation, we retrieve the location of
	// the close button and clicks on that location to close the window.
	fileAppCloseButtonFinder := &pb.Finder{
		NodeWiths: []*pb.NodeWith{
			{Value: &pb.NodeWith_HasClass{HasClass: "FrameCaptionButton"}},
			{Value: &pb.NodeWith_Name{Name: "Close"}},
			{Value: &pb.NodeWith_Focusable{}},
			{Value: &pb.NodeWith_Ancestor{
				Ancestor: filesAppWindowFinder,
			}},
		},
	}
	filesAppCloseButtonInfo, err := uiautoSvc.Info(ctx, &pb.InfoRequest{Finder: fileAppCloseButtonFinder})
	if err != nil {
		s.Fatal("Failed to get node info for files app close button: ", err)
	}
	// Getting center point of the bounding box.
	r := filesAppCloseButtonInfo.NodeInfo.Location
	x, y := r.Left+r.Width/2, r.Top+r.Height/2
	mouseClickReq := &pb.MouseClickAtLocationRequest{
		ClickType: pb.ClickType_CLICK_TYPE_LEFT_CLICK,
		Point:     &pb.Point{X: x, Y: y},
	}
	if _, err := uiautoSvc.MouseClickAtLocation(ctx, mouseClickReq); err != nil {
		s.Fatalf("Failed to click at location %d,%d : %v", x, y, err)
	}
	// Verify that Files App window is gone.
	if res, err := uiautoSvc.IsNodeFound(ctx, &pb.IsNodeFoundRequest{Finder: filesAppWindowFinder}); err != nil || res.Found {
		s.Fatal("Files App window should have been gone: ", err)
	}

	// Open chrome app, open the context menu in the shelf by using RightClick and then close the app from the context menu item.
	if _, err := uiautoSvc.LeftClick(ctx, &pb.LeftClickRequest{Finder: chromeAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to click on Chrome app: ", err)
	}

	if _, err := uiautoSvc.RightClick(ctx, &pb.RightClickRequest{Finder: chromeAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to click on Chrome app: ", err)
	}

	closeButton := &pb.Finder{
		NodeWiths: []*pb.NodeWith{
			{Value: &pb.NodeWith_Name{Name: "Close"}},
			{Value: &pb.NodeWith_Role{Role: pb.Role_ROLE_MENU_ITEM}},
		},
	}

	if _, err := uiautoSvc.WaitUntilExists(ctx, &pb.WaitUntilExistsRequest{Finder: closeButton}); err != nil {
		s.Fatal("Failed to wait for close button from context menu: ", err)
	}

	if _, err := uiautoSvc.LeftClick(ctx, &pb.LeftClickRequest{Finder: closeButton}); err != nil {
		s.Fatal("Failed to close Chrome app from context menu: ", err)
	}
}
