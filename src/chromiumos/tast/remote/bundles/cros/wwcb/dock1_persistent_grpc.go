// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wwcb

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/apps"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/services/cros/wwcb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock1PersistentGRPC,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Windows persistent settings for dual display through a Dock",
		Contacts:     []string{"newmanliu19020@allion.corp-partner.google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps: []string{
			"tast.cros.wwcb.DisplayService",
			"tast.cros.apps.AppsService",
			"tast.cros.browser.ChromeService",
			"tast.cros.ui.AutomationService",
		},
	})
}

func Dock1PersistentGRPC(ctx context.Context, s *testing.State) {
	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Start Chrome on the DUT.
	cs := ui.NewChromeServiceClient(cl.Conn)
	loginReq := &ui.NewRequest{}
	if _, err := cs.New(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.Close(ctx, &empty.Empty{})

	appsSvc := pb.NewAppsServiceClient(cl.Conn)
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

	displaySvc := wwcb.NewDisplayServiceClient(cl.Conn)

	result, err := displaySvc.GetDisplayCount(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to check external display: ", err)
	}
	s.Log(result)

	if _, err := displaySvc.SetMirrorDisplay(ctx, &wwcb.QueryRequest{Enable: true}); err != nil {
		s.Fatal("Failed to enable mirror display: ", err)
	}

	if _, err := displaySvc.SetMirrorDisplay(ctx, &wwcb.QueryRequest{Enable: false}); err != nil {
		s.Fatal("Failed to disable mirror display: ", err)
	}

	if _, err := displaySvc.SetPrimaryDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 1}); err != nil {
		s.Fatal("Failed to set external display as primary: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Files - My files", DisplayIndex: 1}); err != nil {
		s.Fatal("Failed to verify window on external display: ", err)
	}

	if _, err := displaySvc.SetPrimaryDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to set internal display as primary: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Files - My files", DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to verify window on internal display: ", err)
	}

	if _, err := displaySvc.SwitchWindowToDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 1}); err != nil {
		s.Fatal("Failed to switch window to external display 1")
	}

	if _, err := displaySvc.SwitchWindowToDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 2}); err != nil {
		s.Fatal("Failed to switch window to external display 2")
	}

	if _, err := displaySvc.SwitchWindowToDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to switch window to internal display 0")
	}

}
