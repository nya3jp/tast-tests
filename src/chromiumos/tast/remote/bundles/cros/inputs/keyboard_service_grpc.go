// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

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
		Func:         KeyboardServiceGRPC,
		Desc:         "Check basic functionality of KeyboardService",
		Contacts:     []string{"jonfan@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"grpcServerPort"},
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Clamshell)),
	})
}

// KeyboardServiceGRPC check KeyboardService functionalities like Type, Accel, AccelPress and AccelRelease.
func KeyboardServiceGRPC(ctx context.Context, s *testing.State) {
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
		if err := startAppFromLauncher(ctx, uiautoSvc, keyboardSvc, appName); err != nil {
			s.Fatal(fmt.Sprintf("failed to start %v app with keyboard: ", appName), err)
		}
	}

	// Use keyboard Alt-Tab to change the focus to from Files to Chrome App.
	if err := altTab(ctx, keyboardSvc); err != nil {
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
	testing.Sleep(ctx, shortTimeInterval)

	if res, _ := uiautoSvc.IsNodeFound(ctx, &uipb.IsNodeFoundRequest{Finder: appFinder}); res.Found {
		return errors.New("failed to close app")
	}

	return nil
}

func altTab(ctx context.Context, keyboardSvc inputspb.KeyboardServiceClient) error {

	if _, err := keyboardSvc.AccelPress(ctx, &inputspb.AccelPressRequest{Key: "Alt"}); err != nil {
		return errors.Wrap(err, "failed to press alt")
	}
	defer keyboardSvc.AccelRelease(ctx, &inputspb.AccelReleaseRequest{Key: "Alt"})
	if err := testing.Sleep(ctx, shortTimeInterval); err != nil {
		return errors.Wrap(err, "failed to wait")
	}
	if _, err := keyboardSvc.Accel(ctx, &inputspb.AccelRequest{Key: "Tab"}); err != nil {
		return errors.Wrap(err, "failed to type tab")
	}
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	return nil
}

func startAppFromLauncher(ctx context.Context, uiautoSvc uipb.AutomationServiceClient, keyboardSvc inputspb.KeyboardServiceClient, appName string) error {
	// Press the search key to bring the launcher into focus.
	if _, err := keyboardSvc.Accel(ctx, &inputspb.AccelRequest{Key: "Search"}); err != nil {
		return errors.Wrap(err, "failed to press Search")
	}

	//Search for app.
	searchBox := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "Textfield"}},
			{Value: &uipb.NodeWith_Editable{}},
			{Value: &uipb.NodeWith_Focused{}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &uipb.WaitUntilExistsRequest{Finder: searchBox}); err != nil {
		return errors.New("failed to find search box")
	}
	if _, err := keyboardSvc.Type(ctx, &inputspb.TypeRequest{Key: appName}); err != nil {
		return errors.Wrapf(err, "failed to type: %v", appName)
	}

	// Click on the first item in the search tile list.
	installedAppButton := &uipb.Finder{
		NodeWiths: []*uipb.NodeWith{
			{Value: &uipb.NodeWith_HasClass{HasClass: "SearchResultTileItemView"}},
			{Value: &uipb.NodeWith_First{}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &uipb.WaitUntilExistsRequest{Finder: installedAppButton}); err != nil {
		return errors.New("failed to find installed app button")
	}
	if _, err := uiautoSvc.LeftClick(ctx, &uipb.LeftClickRequest{Finder: installedAppButton}); err != nil {
		return errors.Wrapf(err, "failed to open App: %v", appName)
	}

	return nil
}
