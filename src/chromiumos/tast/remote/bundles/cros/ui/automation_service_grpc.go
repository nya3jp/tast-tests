// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"

	"chromiumos/tast/remote/bundles/cros/crosserverutil"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutomationServiceGRPC,
		Desc:         "Check basic functionality of UI Automation Service",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.ChromeStartupService"},
		Vars:         []string{},
		//TODO(jonfan): only clamshell mode
	})
}

// AutomationServiceGRPC tests that we can enable Nearby Share on two DUTs in a single test.
func AutomationServiceGRPC(ctx context.Context, s *testing.State) {
	//TODO(jonfan): move to tast test parameter with default
	port := 4444
	uri := s.DUT().HostName()
	hostname := strings.Split(uri, ":")[0]
	testing.ContextLogf(ctx, "hostname: %s", hostname)

	// Start CrOS server
	if err := crosserverutil.StartCrosServer(ctx, s, port); err != nil {
		s.Fatal("Failed to Start CrOS process: ", err)
	}
	defer crosserverutil.StopCrosServer(ctx, s, port)

	// Setup gRPC channel
	testing.ContextLogf(ctx, "START grpc.Dial")
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", hostname, port), grpc.WithInsecure())
	if err != nil {
		s.Fatal("Fail to Setup gRPC channel: ", err)
	}
	defer conn.Close()

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

	uiautoSvc := ui.NewAutomationServiceClient(conn)

	//Verify that both chrome app and files app buttons are there after initial loading
	filesAppShelfButtonFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "ash/ShelfAppButton"}},
			{Value: &ui.NodeWith_Name{Name: "Files"}},
		},
	}

	chromeAppShelfButtonFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "ash/ShelfAppButton"}},
			{Value: &ui.NodeWith_Name{Name: "Google Chrome"}},
		},
	}

	if _, err := uiautoSvc.WaitUntilExists(ctx, &ui.WaitUntilExistsRequest{Finder: chromeAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to find Chrome shelf button: ", err)
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
	//Wait for search button to show up
	filesAppSearchButtonFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "icon-button"}},
			{Value: &ui.NodeWith_Name{Name: "Search"}},
			{Value: &ui.NodeWith_Focusable{}},
			{Value: &ui.NodeWith_Ancestor{
				Ancestor: filesAppWindowFinder,
			}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &ui.WaitUntilExistsRequest{Finder: filesAppSearchButtonFinder}); err != nil {
		s.Fatal("Failed to wait for files app search button: ", err)
	}

	// Check to see if the search button is there.
	if res, err := uiautoSvc.IsNodeFound(ctx, &ui.IsNodeFoundRequest{Finder: filesAppSearchButtonFinder}); err != nil || !res.Found {
		s.Fatal("Failed to find node for files app search button: ", err)
	}

	// Get and compare NodeInfo for the search button
	nodeInfoResp, err := uiautoSvc.NodeInfo(ctx, &ui.NodeInfoRequest{Finder: filesAppSearchButtonFinder})
	if err != nil {
		s.Fatal("Failed to get node info for files app search button: ", err)
	}
	want := &ui.NodeInfo{
		ClassName: "icon-button menu-button",
		Name:      "Search",
	}

	if diff := cmp.Diff(nodeInfoResp.NodeInfo, want, protocmp.Transform()); diff != "" {
		s.Fatalf("NodeInfo mismatch for search button (-got +want):\n%s", diff)
	}

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

	// Verify that Files App search button is gone
	if res, err := uiautoSvc.IsNodeFound(ctx, &ui.IsNodeFoundRequest{Finder: filesAppSearchButtonFinder}); err != nil || res.Found {
		s.Fatal("Search button should have been gone: ", err)
	}

	// Open chrome app and close it from the context menu in the shelf
	if _, err := uiautoSvc.LeftClick(ctx, &ui.LeftClickRequest{Finder: chromeAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to click on Chrome app: ", err)
	}

	if _, err := uiautoSvc.RightClick(ctx, &ui.RightClickRequest{Finder: chromeAppShelfButtonFinder}); err != nil {
		s.Fatal("Failed to click on Chrome app: ", err)
	}

	closeButton := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_Name{Name: "Close"}},
			{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_MENU_ITEM}},
		},
	}

	if _, err := uiautoSvc.WaitUntilExists(ctx, &ui.WaitUntilExistsRequest{Finder: closeButton}); err != nil {
		s.Fatal("Failed to wait for close button from context menu: ", err)
	}

	if _, err := uiautoSvc.LeftClick(ctx, &ui.LeftClickRequest{Finder: closeButton}); err != nil {
		s.Fatal("Failed to close Chrome app from context menu: ", err)
	}
}

/*
//TODO(jonfan): Move crosserver start/teardown to helper
func startCrosServer(ctx context.Context, s *testing.State, port int) error {
	sshConn := s.DUT().Conn()
	// TODO(jonfan): Refactor into helper class
	args := []string{"-rpctcp", "-port", strconv.Itoa(port)}
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
*/
