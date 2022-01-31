// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/crosserverutil"
	inputspb "chromiumos/tast/services/cros/inputs"
	uipb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	shortTimeInterval = 500 * time.Millisecond
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CombineGRPC,
		Desc:         "Check basic functionality of Automation Service Combine method",
		Contacts:     []string{"jonfan@google.com", "chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"grpcServerPort"},
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
	})
}

// CombineGRPC check Automation Service Combine method.
func CombineGRPC(ctx context.Context, s *testing.State) {
	grpcServerPort := crosserverutil.DefaultGRPCServerPort
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

	// Start Chrome on the DUT.
	cs := uipb.NewChromeServiceClient(cl.Conn)
	loginReq := &uipb.NewRequest{}
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.Close(ctx, &empty.Empty{})

	uiautoSvc := uipb.NewAutomationServiceClient(cl.Conn)
	keyboardSvc := inputspb.NewKeyboardServiceClient(cl.Conn)

	// Open launcher and start apps with keyboard shortcuts.
	launcherButton := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "ash/HomeButton"}},
			{Value: &uipb.NodeWith_Name{Name: "Launcher"}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &uipb.WaitUntilExistsRequest{Finder: launcherButton}); err != nil {
		s.Fatal("Failed to find launcher button: ", err)
	}

	for _, appName := range []string{"Chrome", "Files"} {
		if err := startAppFromLauncher(ctx, uiautoSvc, appName); err != nil {
			s.Fatal(fmt.Sprintf("failed to start %v app with keyboard: ", appName), err)
		}
	}

	// Use keyboard Alt-Tab to change the focus to from Files to Chrome App.
	if err := altTab(ctx, uiautoSvc); err != nil {
		s.Fatal("Failed to Alt-Tab: ", err)
	}

	// Close apps with shortcuts.
	browserWindow := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "BrowserFrame"}},
			{Value: &uipb.NodeWith_Name{Name: "Chrome - New Tab"}},
		},
	}
	filesAppWindow := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "Widget"}},
			{Value: &uipb.NodeWith_Name{Name: "Files - My files"}},
		},
	}

	for _, appFinder := range []*uipb.Finder{browserWindow, filesAppWindow} {
		if err := verifyCloseAppWithShortcuts(ctx, uiautoSvc, keyboardSvc, appFinder); err != nil {
			s.Fatal("Failed to verify closing app with shortcuts: ", err)
		}
	}
}

func verifyCloseAppWithShortcuts(ctx context.Context, uiautoSvc uipb.AutomationServiceClient,
	keyboardSvc inputspb.KeyboardServiceClient, appFinder *uipb.Finder) error {
	//Verify that app is present to begin with.
	if res, _ := uiautoSvc.IsNodeFound(ctx, &uipb.IsNodeFoundRequest{Finder: appFinder}); !res.Found {
		return errors.New("failed to find app")
	}
	//Use keyboard shortcut to close app.
	if _, err := keyboardSvc.Accel(ctx, &inputspb.AccelRequest{Key: "Ctrl+W"}); err != nil {
		return errors.Wrap(err, "failed to type Ctrl+W")
	}
	if _, err := uiautoSvc.Sleep(ctx, &uipb.SleepRequest{Duration: shortTimeInterval.Nanoseconds()}); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	if res, _ := uiautoSvc.IsNodeFound(ctx, &uipb.IsNodeFoundRequest{Finder: appFinder}); res.Found {
		return errors.New("failed to close app")
	}
	return nil
}

func altTab(ctx context.Context, uiautoSvc uipb.AutomationServiceClient) error {
	req := &uipb.CombineRequest{
		Name: "Alt Tab",
		Actions: []*uipb.Action{
			{Request: &uipb.Action_AccelPressRequest{AccelPressRequest: &inputspb.AccelPressRequest{Key: "Alt"}}},
			{Request: &uipb.Action_SleepRequest{SleepRequest: &uipb.SleepRequest{Duration: int64(shortTimeInterval)}}},
			{Request: &uipb.Action_AccelRequest{AccelRequest: &inputspb.AccelRequest{Key: "Tab"}}},
			{Request: &uipb.Action_SleepRequest{SleepRequest: &uipb.SleepRequest{Duration: int64(shortTimeInterval)}}},
			{Request: &uipb.Action_AccelReleaseRequest{AccelReleaseRequest: &inputspb.AccelReleaseRequest{Key: "Alt"}}},
		},
	}
	if _, err := uiautoSvc.Combine(ctx, req); err != nil {
		return errors.Wrap(err, "failed to Alt Tab")
	}

	return nil
}

func startAppFromLauncher(ctx context.Context, uiautoSvc uipb.AutomationServiceClient, appName string) error {
	// Press the search key to bring the launcher and search for app.
	searchBox := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "Textfield"}},
			{Value: &uipb.NodeWith_Editable{}},
			{Value: &uipb.NodeWith_Focused{}},
		},
	}
	searchCombinedReq := &uipb.CombineRequest{
		Name: fmt.Sprintf("Search for app:%s", appName),
		Actions: []*uipb.Action{
			{Request: &uipb.Action_AccelRequest{AccelRequest: &inputspb.AccelRequest{Key: "Search"}}},
			{Request: &uipb.Action_WaitUntilExistsRequest{WaitUntilExistsRequest: &uipb.WaitUntilExistsRequest{Finder: searchBox}}},
			{Request: &uipb.Action_TypeRequest{TypeRequest: &inputspb.TypeRequest{Key: appName}}},
		},
	}
	if _, err := uiautoSvc.Combine(ctx, searchCombinedReq); err != nil {
		return errors.Wrap(err, "failed to search for app")
	}

	// Click on the first item in the search tile list.
	installedAppButton := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "SearchResultTileItemView"}},
			{Value: &uipb.NodeWith_First{}},
		},
	}
	clickAppCombinedReq := &uipb.CombineRequest{
		Name: fmt.Sprintf("Click on first item in search list", appName),
		Actions: []*uipb.Action{
			{Request: &uipb.Action_WaitUntilExistsRequest{WaitUntilExistsRequest: &uipb.WaitUntilExistsRequest{Finder: installedAppButton}}},
			{Request: &uipb.Action_LeftClickRequest{LeftClickRequest: &uipb.LeftClickRequest{Finder: installedAppButton}}},
		},
	}
	if _, err := uiautoSvc.Combine(ctx, clickAppCombinedReq); err != nil {
		return errors.Wrap(err, "failed to search for app")
	}

	return nil
}
