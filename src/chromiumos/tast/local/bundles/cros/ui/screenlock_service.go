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
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ui.RegisterScreenLockServiceServer(srv, &ScreenLockService{s: s})
		},
	})
}

type ScreenLockService struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
}

func (p *ScreenLockService) ReuseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	cr, err := chrome.New(ctx, chrome.TryReuseSession())
	if err != nil {
		return nil, err
	}
	p.cr = cr
	return &empty.Empty{}, nil
}

func (p *ScreenLockService) NewChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
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

func (p *ScreenLockService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if p.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := p.cr.Close(ctx)
	p.cr = nil
	return &empty.Empty{}, err
}

func (p *ScreenLockService) Lock(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if p.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	tconn, err := p.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to lock the screen")
	}

	return &empty.Empty{}, nil
}
