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
	"chromiumos/tast/local/chrome/ui"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterPowerMenuServiceServer(srv, &PowerMenuService{s: s})
		},
	})
}

// PowerMenuService implements tast.cros.power.PowerMenuService.
type PowerMenuService struct {
	s            *testing.ServiceState
	cr           *chrome.Chrome
	requestLogin bool
	tconn        *chrome.TestConn
}

func (p *PowerMenuService) NewChrome(ctx context.Context, req *pb.NewChromeRequest) (*empty.Empty, error) {
	if p.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	p.requestLogin = req.Login
	if p.requestLogin {
		cr, err := chrome.New(ctx)
		if err != nil {
			return nil, err
		}
		p.cr = cr
	} else {
		cr, err := chrome.New(ctx, chrome.KeepState(), chrome.NoLogin(), chrome.LoadSigninProfileExtension(req.Key))
		if err != nil {
			return nil, err
		}
		p.cr = cr
	}
	return &empty.Empty{}, nil
}

func (p *PowerMenuService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if p.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := p.cr.Close(ctx)
	p.cr = nil
	return &empty.Empty{}, err
}

func (p *PowerMenuService) IsPowerMenuPresent(ctx context.Context, req *empty.Empty) (*pb.IsPowerMenuPresentResponse, error) {
	if p.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	if p.requestLogin {
		tconn, err := p.cr.TestAPIConn(ctx)
		if err != nil {
			return nil, err
		}
		p.tconn = tconn
	} else {
		tconn, err := p.cr.SigninProfileTestAPIConn(ctx)
		if err != nil {
			return nil, err
		}
		p.tconn = tconn
	}

	root, err := ui.Root(ctx, p.tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get root node")
	}
	defer root.Release(ctx)

	// Check if the power menu is displayed
	params := ui.FindParams{
		ClassName: "PowerButtonMenuView",
		Role:      ui.RoleTypeMenu,
	}
	exists, err := root.DescendantExists(ctx, params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find power menu")
	}

	return &pb.IsPowerMenuPresentResponse{IsMenuPresent: exists}, nil
}
