// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	nearbycommon "chromiumos/tast/common/cros/nearbyshare"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ui.RegisterScreenRecorderServiceServer(srv, &ScreenRecorderService{s: s})
		},
		GuaranteeCompatibility: true,
	})
}

// ScreenRecorderService implements tast.cros.ui.ScreenRecorderService
type ScreenRecorderService struct {
	s *testing.ServiceState

	cr              *chrome.Chrome
	tconn           *chrome.TestConn
	deviceName      string
	senderSurface   *nearbyshare.SendSurface
	receiverSurface *nearbyshare.ReceiveSurface
	chromeReader    *syslog.LineReader
	messageReader   *syslog.LineReader
	fileNames       []string
	username        string
	dataUsage       nearbycommon.DataUsage
	visibility      nearbycommon.Visibility
	btsnoopCmd      *testexec.Cmd
}

// NewChromeLogin logs into Chrome with Nearby Share flags enabled.
func (n *ScreenRecorderService) NewChromeLogin(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	testing.ContextLog(ctx, "NewChromeLogin LOG 1")
	if n.cr != nil {
		return nil, errors.New("Chrome already available")
	}
	nearbyOpts := []chrome.Option{
		chrome.EnableFeatures("GwpAsanMalloc", "GwpAsanPartitionAlloc"),
		chrome.DisableFeatures("SplitSettingsSync"),
		chrome.ExtraArgs("--nearby-share-verbose-logging", "--enable-logging", "--vmodule=*blue*=1", "--vmodule=*nearby*=1"),
	}
	// n.username = chrome.DefaultUser
	// if req.Username != "" {
	// 	n.username = req.Username
	// 	nearbyOpts = append(nearbyOpts, chrome.GAIALogin(chrome.Creds{User: req.Username, Pass: req.Password}))
	// }
	// if req.KeepState {
	// 	nearbyOpts = append(nearbyOpts, chrome.KeepState())
	// }
	// testing.ContextLog(ctx, "NewChromeLogin LOG 2")
	// testing.ContextLog(ctx, req.EnabledFlags)
	// for _, flag := range req.EnabledFlags {
	// 	nearbyOpts = append(nearbyOpts, chrome.EnableFeatures(flag))
	// }

	cr, err := chrome.New(ctx, nearbyOpts...)
	if err != nil {
		testing.ContextLog(ctx, "Failed to start Chrome")
		return nil, err
	}
	n.cr = cr
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get a connection to the Test Extension")
		return nil, err
	}
	n.tconn = tconn
	testing.ContextLog(ctx, "NewChromeLogin LOG 3")
	return &empty.Empty{}, nil
}

// CloseChrome closes all surfaces and Chrome.
// This will likely be called in a defer in remote tests instead of called explicitly. So log everything that fails to aid debugging later.
func (n *ScreenRecorderService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if n.cr == nil {
		testing.ContextLog(ctx, "Chrome not available")
		return nil, errors.New("Chrome not available")
	}
	os.RemoveAll(nearbycommon.SendDir)
	if n.senderSurface != nil {
		if err := n.senderSurface.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Closing SendSurface failed: ", err)
		}
	}
	if n.receiverSurface != nil {
		if err := n.receiverSurface.Close(ctx); err != nil {
			testing.ContextLog(ctx, "Closing ReceiveSurface failed: ", err)
		}
	}
	err := n.cr.Close(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Faied to close Chrome in Nearby Share service: ", err)
	} else {
		testing.ContextLog(ctx, "Nearby Share service closed successfully for: ", n.deviceName)
	}
	n.cr = nil
	return &empty.Empty{}, err
}
