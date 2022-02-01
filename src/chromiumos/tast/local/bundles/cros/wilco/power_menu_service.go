// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	pb "chromiumos/tast/services/cros/wilco"
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
	s  *testing.ServiceState
	cr *chrome.Chrome
}

func (p *PowerMenuService) NewChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if p.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	p.cr = cr
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

	tconn, err := p.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	// Check if the power menu is displayed
	ui := uiauto.New(tconn)
	exists, err := ui.IsNodeFound(ctx, nodewith.HasClass("PowerButtonMenuView").Role(role.Menu))
	if err != nil {
		return nil, errors.Wrap(err, "failed to find power menu")
	}

	return &pb.IsPowerMenuPresentResponse{IsMenuPresent: exists}, nil
}
