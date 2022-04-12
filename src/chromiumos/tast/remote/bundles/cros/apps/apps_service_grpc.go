// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"strconv"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/remote/crosserverutil"
	pb "chromiumos/tast/services/cros/apps"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParams struct {
	chromeRequest *ui.NewRequest
	browserName   string
}

var disableFeatures = []string{"DefaultWebAppInstallation"}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppsServiceGRPC,
		Desc:         "Check basic functionalities of AppsService",
		Contacts:     []string{"msta@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"grpcServerPort"},
		HardwareDeps: hwdep.D(hwdep.Model("betty")),
		LacrosStatus: testing.LacrosVariantExists,
		Params: []testing.Param{
			{
				Name: "ash",
				Val: testParams{
					chromeRequest: &ui.NewRequest{DisableFeatures: disableFeatures},
					browserName:   "Chrome",
				},
			},
			{
				Name: "lacros",
				Val: testParams{
					chromeRequest: &ui.NewRequest{DisableFeatures: disableFeatures, EnableFeatures: []string{"LacrosSupport", "LacrosPrimary"}},
					browserName:   "Lacros",
				},
			},
		},
	})
}

// AppsServiceGRPC tests basic functionalities of UI AppsService.
func AppsServiceGRPC(ctx context.Context, s *testing.State) { // NOLINT
	variant := s.Param().(testParams)

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
	cs := ui.NewChromeServiceClient(cl.Conn)
	if _, err := cs.New(ctx, variant.chromeRequest, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.Close(ctx, &empty.Empty{})

	appsSvc := pb.NewAppsServiceClient(cl.Conn)
	if _, err := appsSvc.LaunchApp(ctx, &pb.LaunchAppRequest{AppName: "Unknown app", TimeoutSecs: 1}); err == nil {
		s.Fatal("Launch non-existent app succeeded")
	}

	if _, err := appsSvc.LaunchApp(ctx, &pb.LaunchAppRequest{AppName: "Files", TimeoutSecs: 60}); err != nil {
		s.Fatal("Failed to launch files app: ", err)
	}

	uiautoSvc := ui.NewAutomationServiceClient(cl.Conn)

	filesAppWindowFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_Name{Name: "Files - My files"}},
			{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_WINDOW}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}

	if _, err := uiautoSvc.WaitUntilExists(ctx, &ui.WaitUntilExistsRequest{Finder: filesAppWindowFinder}); err != nil {
		s.Fatal("Files app never appeared: ", err)
	}

	browser, err := appsSvc.LaunchPrimaryBrowser(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to launch primary browser: ", err)
	}
	if browser.Name != variant.browserName {
		s.Fatalf("Incorrect browser name: got %v; want %v", browser.Name, variant.browserName)
	}

	browserWindowFinder := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_NameContaining{NameContaining: "New Tab"}},
			{Value: &ui.NodeWith_Role{Role: ui.Role_ROLE_WINDOW}},
			{Value: &ui.NodeWith_First{First: true}},
		},
	}

	if _, err := uiautoSvc.WaitUntilExists(ctx, &ui.WaitUntilExistsRequest{Finder: browserWindowFinder}); err != nil {
		s.Fatal("Browser never appeared: ", err)
	}
}
