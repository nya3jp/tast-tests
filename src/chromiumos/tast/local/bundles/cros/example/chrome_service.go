// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"encoding/json"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/cros/example"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			example.RegisterChromeServer(srv, &Chrome{s: s})
		},
	})
}

// Chrome implements tast.cros.example.Chrome gRPC service.
type Chrome struct {
	s *testing.ServiceState

	cr *chrome.Chrome
}

func (c *Chrome) New(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
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

func (c *Chrome) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := c.cr.Close(ctx)
	c.cr = nil
	return &empty.Empty{}, err
}

func (c *Chrome) EvalOnTestAPIConn(ctx context.Context, req *example.EvalOnTestAPIConnRequest) (*example.EvalOnTestAPIConnResponse, error) {
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
