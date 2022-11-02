// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wwcb contains remote Tast tests that work with Chromebook
package wwcb

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wwcb/utils"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/apps"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/services/cros/wwcb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock2PersistentGRPC,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Windows persistent settings for dual display through a Dock",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"DockingID", "ExtDispID", "ExtDispID2"},
		ServiceDeps: []string{
			"tast.cros.wwcb.DisplayService",
			"tast.cros.apps.AppsService",
			"tast.cros.browser.ChromeService",
		},
	})
}

func Dock2PersistentGRPC(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID := s.RequiredVar("ExtDispID")
	extDispID2 := s.RequiredVar("ExtDispID2")

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

	displaySvc := wwcb.NewDisplayServiceClient(cl.Conn)
	appsSvc := pb.NewAppsServiceClient(cl.Conn)

	// Open IP power and initialize fixtures.
	if err := utils.OpenIppower(ctx, []int{1}); err != nil {
		s.Fatal("Failed to open IP Power: ", err)
	}
	if err := utils.InitFixture(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}
	defer utils.CloseAllFixture(ctx)

	s.Log("Step 1 - Connect two external displays and docking station")

	if err := utils.ControlFixture(ctx, extDispID, "on"); err != nil {
		s.Fatal("Failed to connect first external display: ", err)
	}

	if err := utils.ControlFixture(ctx, extDispID2, "on"); err != nil {
		s.Fatal("Failed to connect second external display: ", err)
	}

	if err := utils.ControlFixture(ctx, dockingID, "on"); err != nil {
		s.Fatal("Failed to connect docking station: ", err)
	}

	if _, err := displaySvc.VerifyDisplayCount(ctx, &wwcb.QueryRequest{DisplayCount: 3}); err != nil {
		s.Fatal("Failed to verify display count: ", err)
	}

	s.Log("Step 2 - Open two apps on external display")

	if _, err := appsSvc.LaunchApp(ctx, &pb.LaunchAppRequest{AppName: "Files", TimeoutSecs: 60}); err != nil {
		s.Fatal("Failed to launch files app: ", err)
	}

	if _, err := appsSvc.LaunchPrimaryBrowser(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to launch primary browser: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := displaySvc.SwitchWindowToDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 1, WindowTitle: "Files - My files"}); err != nil {
			return errors.Wrap(err, "failed to switch files window to first external display")
		}

		if _, err := displaySvc.SwitchWindowToDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 2, WindowTitle: "Chrome - New Tab"}); err != nil {
			return errors.Wrap(err, "failed to switch chrome window to second external display")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to switch two windows to external display: ", err)
	}

	s.Log("Step 3 - Unplug and re-plug in, check windows on expected display")

	if err := utils.ControlFixture(ctx, extDispID, "off"); err != nil {
		s.Fatal("Failed to disconnect first external display: ", err)
	}

	if err := utils.ControlFixture(ctx, extDispID2, "off"); err != nil {
		s.Fatal("Failed to disconnect second external display: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Files - My files", DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to verify files window on internal display: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Chrome - New Tab", DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to verify chrome window on internal display: ", err)
	}

	if err := utils.ControlFixture(ctx, extDispID, "on"); err != nil {
		s.Fatal("Failed to connect first external display: ", err)
	}

	if err := utils.ControlFixture(ctx, extDispID2, "on"); err != nil {
		s.Fatal("Failed to connect second external display: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Files - My files", DisplayIndex: 1}); err != nil {
		s.Fatal("Failed to verify files window on first external display: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Chrome - New Tab", DisplayIndex: 2}); err != nil {
		s.Fatal("Failed to verify chrome window on second external display: ", err)
	}

	s.Log("Step 4 - Test primary mode")

	if _, err := displaySvc.SetPrimaryDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to set internal display as primary: ", err)
	}

	// Retry to switch window, because need to switch windows twice then it would show on internal display.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := displaySvc.SwitchWindowToDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 0, WindowTitle: "Files - My files"}); err != nil {
			return errors.Wrap(err, "failed to switch files window to internal display")
		}

		if _, err := displaySvc.SwitchWindowToDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 0, WindowTitle: "Chrome - New Tab"}); err != nil {
			return errors.Wrap(err, "failed to switch chrome window to internal display")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		s.Fatal("Failed to switch windows to internal display: ", err)
	}

	if _, err := displaySvc.SetPrimaryDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 1}); err != nil {
		s.Fatal("Failed to set external display as primary: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Files - My files", DisplayIndex: 1}); err != nil {
		s.Fatal("Failed to verify files window on external display: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Chrome - New Tab", DisplayIndex: 1}); err != nil {
		s.Fatal("Failed to verify chrome window on external display: ", err)
	}

	s.Log("Step 5 - Unplug and re-plug in, check windows on expected display")

	if err := utils.ControlFixture(ctx, extDispID, "off"); err != nil {
		s.Fatal("Failed to disconnect first external display: ", err)
	}

	if _, err := displaySvc.VerifyDisplayCount(ctx, &wwcb.QueryRequest{DisplayCount: 2}); err != nil {
		s.Fatal("Failed to verify display count: ", err)
	}

	if err := utils.ControlFixture(ctx, extDispID2, "off"); err != nil {
		s.Fatal("Failed to disconnect second external display: ", err)
	}

	if _, err := displaySvc.VerifyDisplayCount(ctx, &wwcb.QueryRequest{DisplayCount: 1}); err != nil {
		s.Fatal("Failed to verify display count: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Files - My files", DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to verify files window on internal display: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Chrome - New Tab", DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to verify chrome window on internal display: ", err)
	}

	if err := utils.ControlFixture(ctx, extDispID, "on"); err != nil {
		s.Fatal("Failed to connect first external display: ", err)
	}

	if _, err := displaySvc.VerifyDisplayCount(ctx, &wwcb.QueryRequest{DisplayCount: 2}); err != nil {
		s.Fatal("Failed to verify display count: ", err)
	}

	if err := utils.ControlFixture(ctx, extDispID2, "on"); err != nil {
		s.Fatal("Failed to connect second external display: ", err)
	}

	if _, err := displaySvc.VerifyDisplayCount(ctx, &wwcb.QueryRequest{DisplayCount: 3}); err != nil {
		s.Fatal("Failed to verify display count: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Files - My files", DisplayIndex: 1}); err != nil {
		s.Fatal("Failed to verify files window on external display: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Chrome - New Tab", DisplayIndex: 1}); err != nil {
		s.Fatal("Failed to verify chrome window on external display: ", err)
	}

	s.Log("Step 6 - Test mirror mode")

	if _, err := displaySvc.SetPrimaryDisplay(ctx, &wwcb.QueryRequest{DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to set internal display as primary: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Files - My files", DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to verify files window on internal display: ", err)
	}

	if _, err := displaySvc.VerifyWindowOnDisplay(ctx, &wwcb.QueryRequest{WindowTitle: "Chrome - New Tab", DisplayIndex: 0}); err != nil {
		s.Fatal("Failed to verify chrome window on internal display: ", err)
	}

	if _, err := displaySvc.SetMirrorDisplay(ctx, &wwcb.QueryRequest{Enable: true}); err != nil {
		s.Fatal("Failed to enable mirror display: ", err)
	}

	if _, err := displaySvc.SetMirrorDisplay(ctx, &wwcb.QueryRequest{Enable: false}); err != nil {
		s.Fatal("Failed to disable mirror display: ", err)
	}
}
