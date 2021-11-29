// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/crosserverutil"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyboardServiceGRPC,
		Desc:         "Check basic functionality of UI Automation Service",
		Contacts:     []string{"chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.ChromeStartupService"},
		Vars:         []string{},
		//TODO(jonfan): only clamshell mode
	})
}

// KeyboardServiceGRPC tests that we can enable Nearby Share on two DUTs in a single test.
func KeyboardServiceGRPC(ctx context.Context, s *testing.State) {
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

	// Connect to the Nearby Share Service so we can execute local code on the DUT.
	cs := ui.NewChromeStartupServiceClient(conn)
	loginReq := &ui.NewChromeLoginRequest{}
	if _, err := cs.NewChromeLogin(ctx, loginReq, grpc.WaitForReady(true)); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cs.CloseChrome(ctx, &empty.Empty{})

	uiautoSvc := ui.NewAutomationServiceClient(conn)
	keyboardSvc := inputs.NewKeyboardServiceClient(conn)

	launcherButton := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "ash/HomeButton"}},
			{Value: &ui.NodeWith_Name{Name: "Launcher"}},
		},
	}
	if _, err := uiautoSvc.WaitUntilExists(ctx, &ui.WaitUntilExistsRequest{Finder: launcherButton}); err != nil {
		s.Fatal("Failed to find launcher button: ", err)
	}

	for _, appName := range []string{"Chrome", "Files"} {
		if err := startAppFromLauncher(ctx, uiautoSvc, keyboardSvc, appName); err != nil {
			s.Fatal(fmt.Sprintf("Failed to start %v app with keyboard: ", appName), err)
		}
	}

	// Use Alt Tab to change the focus to Chrome App
	if err := altTab(ctx, keyboardSvc); err != nil {
		s.Fatal("Failed to Alt-Tab: ", err)
	}

	// Close apps with shortcuts
	browserWindow := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "BrowserFrame"}},
			{Value: &ui.NodeWith_Name{Name: "Chrome - New Tab"}},
		},
	}
	filesAppWindow := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "NativeAppWindowViews"}},
			{Value: &ui.NodeWith_Name{Name: "Files - My files"}},
		},
	}

	for _, appFinder := range []*ui.Finder{browserWindow, filesAppWindow} {
		if err := verifyCloseAppWithShortcuts(ctx, uiautoSvc, keyboardSvc, appFinder); err != nil {
			s.Fatal("Failed to verify closing app with shortcuts: ", err)
		}
	}
}

func verifyCloseAppWithShortcuts(ctx context.Context, uiautoSvc ui.AutomationServiceClient,
	keyboardSvc inputs.KeyboardServiceClient, appFinder *ui.Finder) error {
	if res, _ := uiautoSvc.IsNodeFound(ctx, &ui.IsNodeFoundRequest{Finder: appFinder}); !res.Found {
		return errors.New("Failed to find app")
	}

	if _, err := keyboardSvc.Accel(ctx, &inputs.AccelRequest{Key: "Ctrl+W"}); err != nil {
		return errors.Wrap(err, "Failed to type Ctrl+W")
	}

	testing.Sleep(ctx, 200*time.Millisecond)

	if res, _ := uiautoSvc.IsNodeFound(ctx, &ui.IsNodeFoundRequest{Finder: appFinder}); res.Found {
		return errors.New("Failed to close app")
	}

	return nil
}

func altTab(ctx context.Context, keyboardSvc inputs.KeyboardServiceClient) error {

	if _, err := keyboardSvc.AccelPress(ctx, &inputs.AccelPressRequest{Key: "Alt"}); err != nil {
		return errors.Wrap(err, "failed to press alt")
	}
	defer keyboardSvc.AccelRelease(ctx, &inputs.AccelReleaseRequest{Key: "Alt"})
	if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to wait")
	}
	if _, err := keyboardSvc.Accel(ctx, &inputs.AccelRequest{Key: "Tab"}); err != nil {
		return errors.Wrap(err, "failed to type tab")
	}
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	return nil
}

func startAppFromLauncher(ctx context.Context, uiautoSvc ui.AutomationServiceClient, keyboardSvc inputs.KeyboardServiceClient, appName string) error {
	// Press the search key to bring the launcher into focus.
	if _, err := keyboardSvc.Accel(ctx, &inputs.AccelRequest{Key: "Search"}); err != nil {
		return errors.Wrap(err, "Failed to press Search: ")
	}

	testing.Sleep(ctx, 100*time.Millisecond)

	if _, err := keyboardSvc.Type(ctx, &inputs.TypeRequest{Key: appName}); err != nil {
		return errors.Wrapf(err, "Failed to type: %v", appName)
	}

	testing.Sleep(ctx, 1000*time.Millisecond)

	// Click on the first time in the search list
	installedAppButton := &ui.Finder{
		NodeWiths: []*ui.NodeWith{
			{Value: &ui.NodeWith_HasClass{HasClass: "SearchResultTileItemView"}},
			{Value: &ui.NodeWith_First{}},
		},
	}

	if _, err := uiautoSvc.LeftClick(ctx, &ui.LeftClickRequest{Finder: installedAppButton}); err != nil {
		return errors.Wrapf(err, "Failed to open App: %v", appName)
	}

	return nil
}
