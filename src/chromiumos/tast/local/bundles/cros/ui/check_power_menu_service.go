// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/procutil"
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

// PowerMenuService implements tast.cros.ui.PowerMenuService.
type PowerMenuService struct {
	s              *testing.ServiceState
	cr             *chrome.Chrome
	loginRequested bool
	tconn          *chrome.TestConn
}

func (p *PowerMenuService) NewChrome(ctx context.Context, req *pb.NewChromeRequest) (*empty.Empty, error) {
	if p.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	oldproc, err := ashproc.Root()
	if err != nil {
		return nil, errors.Wrap(err, "error getting Chrome root PID")
	}

	p.loginRequested = req.Login
	if p.loginRequested {
		p.cr, err = chrome.New(ctx, chrome.Region("us"))
	} else {
		p.cr, err = chrome.New(ctx, chrome.Region("us"), chrome.KeepState(), chrome.NoLogin(), chrome.LoadSigninProfileExtension(req.Key))
	}
	if err != nil {
		return nil, err
	}

	// Make sure older chrome is gone.
	if err := procutil.WaitForTerminated(ctx, oldproc, 30*time.Second); err != nil {
		return nil, errors.Wrap(err, "chrome is not terminated")
	}

	// Then, wait for the new chrome processes.
	if _, err := ashproc.WaitForRoot(ctx, 30*time.Second); err != nil {
		return nil, errors.Wrap(err, "chrome is not restarted")
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

func (p *PowerMenuService) PowerMenuPresent(ctx context.Context, req *empty.Empty) (*pb.PowerMenuPresentResponse, error) {
	if p.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	var err error
	if p.loginRequested {
		p.tconn, err = p.cr.TestAPIConn(ctx)
	} else {
		p.tconn, err = p.cr.SigninProfileTestAPIConn(ctx)
	}
	if err != nil {
		return nil, err
	}

	// Check if the power menu is displayed
	finder := nodewith.ClassName("PowerButtonMenuView").Onscreen().First()
	exists, err := uiauto.New(p.tconn).IsNodeFound(ctx, finder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find power menu")
	}

	return &pb.PowerMenuPresentResponse{IsMenuPresent: exists}, nil
}

// PowerMenuItem reads from the power menu.
// Checking PowerMenuPresent is required prior to calling PowerMenuItem.
func (p *PowerMenuService) PowerMenuItem(ctx context.Context, req *empty.Empty) (*pb.PowerMenuItemResponse, error) {
	if p.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	if p.tconn == nil {
		return nil, errors.New("Test API connection not available")
	}

	ui := uiauto.New(p.tconn)
	menu := nodewith.ClassName("PowerButtonMenuItemView")
	menuItems, err := ui.NodesInfo(ctx, menu)
	if err != nil {
		return nil, err
	}

	var itemsName []string
	for _, val := range menuItems {
		itemsName = append(itemsName, val.Name)
	}

	return &pb.PowerMenuItemResponse{MenuItems: itemsName}, nil
}
