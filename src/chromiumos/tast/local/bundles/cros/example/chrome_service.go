// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"encoding/json"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/testing"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			example.RegisterChromeServiceServer(srv, &ChromeService{s: s})
		},
	})
}

// ChromeService implements tast.cros.example.ChromeService.
type ChromeService struct {
	s *testing.ServiceState

	cr *chrome.Chrome
}

func (c *ChromeService) New(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	c.cr = cr
	return &empty.Empty{}, nil
}

func (c *ChromeService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := c.cr.Close(ctx)
	c.cr = nil
	return &empty.Empty{}, err
}

func (c *ChromeService) EvalOnTestAPIConn(ctx context.Context, req *example.EvalOnTestAPIConnRequest) (*example.EvalOnTestAPIConnResponse, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	conn, err := c.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	var res json.RawMessage
	if err := conn.Eval(ctx, req.Expr, &res); err != nil {
		return nil, err
	}
	return &example.EvalOnTestAPIConnResponse{ValueJson: string(res)}, nil
}

func (c *ChromeService) OpenPage(ctx context.Context, req *example.OpenPageRequest) (*empty.Empty, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	_, err := c.cr.NewConn(ctx, req.Url)

	if err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (c *ChromeService) RelaunchAfterUpdate(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	cr := c.cr

	policyutil.OSSettingsPage(ctx, cr, "help")

	tconn, _ := cr.TestAPIConn(ctx)

	ui := uiauto.New(tconn)

	restart := nodewith.Name("Restart").Role(role.Button)

	ui.WaitUntilExists(restart)(ctx)
	ui.LeftClick(restart)(ctx)

	return &empty.Empty{}, nil
}