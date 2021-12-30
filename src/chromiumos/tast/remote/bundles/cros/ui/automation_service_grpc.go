// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strconv"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"

	"chromiumos/tast/remote/crosserverutil"
	chromepb "chromiumos/tast/services/cros/browser"
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
	//Connect to gRPC Server on DUT.
	hostname, port := getGRPCHostnamePort(ctx, s)
	cl, err := crosserverutil.Dial(ctx, s.DUT(), hostname, port)
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Start Chrome on the DUT.
	cs := chromepb.NewChromeServiceClient(cl.Conn)
	loginReq := &chromepb.NewRequest{}
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.Close(ctx, &empty.Empty{})

	uiautoSvc := uipb.NewAutomationServiceClient(cl.Conn)

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
	filesAppSearchButtonInfo, err := uiautoSvc.Info(ctx, &uipb.InfoRequest{Finder: filesAppSearchButtonFinder})
	if err != nil {
		s.Fatal("Failed to get node info for files app search button: ", err)
	}
	// skip comparing Location and HtmlAttributes as those are form factor specific.
	filesAppSearchButtonInfo.NodeInfo.Location = nil
	filesAppSearchButtonInfo.NodeInfo.HtmlAttributes = nil
	want := &uipb.NodeInfo{
		ClassName:   "icon-button menu-button",
		Name:        "Search",
		Restriction: uipb.Restriction_NONE,
		Role:        uipb.Role_BUTTON,
		State:       map[string]bool{"focusable": true},
	}

	if diff := cmp.Diff(filesAppSearchButtonInfo.NodeInfo, want, protocmp.Transform()); diff != "" {
		s.Fatalf("NodeInfo mismatch for search button (-got +want):%s", diff)
	}

	// Close Files app by clicking the cross on the window
	// We will fetch the location of the Node first, followed by click on the center
	// of that location.
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
	filesAppCloseButtonInfo, err := uiautoSvc.Info(ctx, &uipb.InfoRequest{Finder: fileAppCloseButtonFinder})
	if err != nil {
		s.Fatal("Failed to get node info for files app close button: ", err)
	}
	r := filesAppCloseButtonInfo.NodeInfo.Location
	x, y := r.Left+r.Width/2, r.Top+r.Height/2
	mouseClickReq := &uipb.MouseClickAtLocationRequest{
		ClickType: uipb.ClickType_LEFT_CLICK,
		Point:     &uipb.Point{X: x, Y: y},
	}
	if _, err := uiautoSvc.MouseClickAtLocation(ctx, mouseClickReq); err != nil {
		s.Fatalf("Failed to click at location %d,%d : %v", x, y, err)
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
			{Value: &uipb.NodeWith_Role{Role: uipb.Role_MENU_ITEM}},
		},
	}

	if _, err := uiautoSvc.WaitUntilExists(ctx, &uipb.WaitUntilExistsRequest{Finder: closeButton}); err != nil {
		s.Fatal("Failed to wait for close button from context menu: ", err)
	}

	if _, err := uiautoSvc.LeftClick(ctx, &uipb.LeftClickRequest{Finder: closeButton}); err != nil {
		s.Fatal("Failed to close Chrome app from context menu: ", err)
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
