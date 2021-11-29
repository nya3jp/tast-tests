// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	commoncros "chromiumos/tast/common/cros"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ui.RegisterChromeStartupServiceServer(srv,
				&ChromeStartupService{s: s, sharedObject: commoncros.SharedObjectsForServiceSingleton})
		},
		GuaranteeCompatibility: true,
	})
}

// ChromeStartupService implements tast.cros.ui.ChromeStartupService
type ChromeStartupService struct {
	s            *testing.ServiceState
	sharedObject *commoncros.SharedObjectsForService
}

// NewChromeLogin logs into Chrome with Nearby Share flags enabled.
func (svc *ChromeStartupService) NewChromeLogin(ctx context.Context, req *ui.NewChromeLoginRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "NewChromeLogin LOG 1")
	nearbyOpts := []chrome.Option{}
	// nearbyOpts := []chrome.Option{
	// 	chrome.EnableFeatures("GwpAsanMalloc", "GwpAsanPartitionAlloc"),
	// 	chrome.DisableFeatures("SplitSettingsSync"),
	// 	chrome.ExtraArgs("--nearby-share-verbose-logging", "--enable-logging", "--vmodule=*blue*=1", "--vmodule=*nearby*=1"),
	// }
	if req.Username != "" {
		nearbyOpts = append(nearbyOpts, chrome.GAIALogin(chrome.Creds{User: req.Username, Pass: req.Password}))
	}
	// if req.KeepState {
	// 	nearbyOpts = append(nearbyOpts, chrome.KeepState())
	// }
	// for _, flag := range req.EnabledFlags {
	// 	nearbyOpts = append(nearbyOpts, chrome.EnableFeatures(flag))
	// }

	cr, err := chrome.New(ctx, nearbyOpts...)
	if err != nil {
		testing.ContextLog(ctx, "Failed to start Chrome")
		return nil, err
	}
	svc.sharedObject.Chrome = cr
	// tconn, err := cr.TestAPIConn(ctx)
	// if err != nil {
	// 	testing.ContextLog(ctx, "Failed to get a connection to the Test Extension")
	// 	return nil, err
	// }
	// svc.tconn = tconn
	// testing.ContextLog(ctx, "NewChromeLogin LOG 3")
	return &empty.Empty{}, nil
}

// CloseChrome closes all surfaces and Chrome.
// This will likely be called in a defer in remote tests instead of called explicitly. So log everything that fails to aid debugging later.
func (svc *ChromeStartupService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if svc.sharedObject.Chrome == nil {
		testing.ContextLog(ctx, "Chrome not available")
		return nil, errors.New("Chrome not available")
	}

	err := svc.sharedObject.Chrome.Close(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to close Chrome: ", err)
	}

	svc.sharedObject.Chrome = nil
	return &empty.Empty{}, err
}
