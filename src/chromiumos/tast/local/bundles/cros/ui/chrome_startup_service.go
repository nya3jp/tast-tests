// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	common "chromiumos/tast/local/common"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ui.RegisterChromeStartupServiceServer(srv,
				&ChromeStartupService{s: s, sharedObject: common.SharedObjectsForServiceSingleton})
		},
		GuaranteeCompatibility: true,
	})
}

// ChromeStartupService implements tast.cros.ui.ChromeStartupService
//TODO(jonfan): Replace examples chrome.proto??
type ChromeStartupService struct {
	s            *testing.ServiceState
	sharedObject *common.SharedObjectsForService
}

// NewChromeLogin logs into Chrome with Nearby Share flags enabled.
func (svc *ChromeStartupService) NewChromeLogin(ctx context.Context, req *ui.NewChromeLoginRequest) (*empty.Empty, error) {
	opts, _ := toOptions(req)
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		testing.ContextLog(ctx, "Failed to start Chrome")
		return nil, err
	}
	svc.sharedObject.Chrome = cr

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

func toOptions(req *ui.NewChromeLoginRequest) ([]chrome.Option, error) {
	var options []chrome.Option

	if req.KeepState {
		options = append(options, chrome.KeepState())
	}

	if req.TryReuseSession {
		options = append(options, chrome.TryReuseSession())
	}

	switch req.GetLoginMode() {
	case ui.NewChromeLoginRequest_LOGIN_MODE_NO_LOGIN:
		options = append(options, chrome.NoLogin())
	case ui.NewChromeLoginRequest_LOGIN_MODE_FAKE_LOGIN:
		options = append(options, chrome.FakeLogin(toCreds(req.Credentials)))
	case ui.NewChromeLoginRequest_LOGIN_MODE_GAIA_LOGIN:
		options = append(options, chrome.GAIALogin(toCreds(req.Credentials)))
	case ui.NewChromeLoginRequest_LOGIN_MODE_GUEST_LOGIN:
		options = append(options, chrome.GuestLogin())
	}

	if req.ExtraArgs != nil && len(req.ExtraArgs) > 0 {
		options = append(options, chrome.ExtraArgs(req.ExtraArgs...))
	}

	if req.EnableFeatures != nil && len(req.EnableFeatures) > 0 {
		options = append(options, chrome.EnableFeatures(req.EnableFeatures...))
	}

	if req.DisableFeatures != nil && len(req.DisableFeatures) > 0 {
		options = append(options, chrome.DisableFeatures(req.DisableFeatures...))
	}

	return options, nil
}

func toCreds(c *ui.NewChromeLoginRequest_Credentials) chrome.Creds {
	return chrome.Creds{
		User:       c.Username,
		Pass:       c.Password,
		GAIAID:     c.GaiaId,
		Contact:    c.Contact,
		ParentUser: c.ParentUsername,
		ParentPass: c.ParentPassword,
	}
}
