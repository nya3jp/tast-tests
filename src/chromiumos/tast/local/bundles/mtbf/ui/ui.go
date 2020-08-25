// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/mtbf/service"
	mtbfui "chromiumos/tast/local/mtbf/ui"
	server "chromiumos/tast/services/mtbf/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			server.RegisterUIServer(srv, &UI{service.New(s)})
		},
	})
}

// A UI implements the tast/services/mtbf/ui.UI interface.
type UI struct {
	service.Service
}

func (s *UI) WaitElement(ctx context.Context, req *server.WaitElementRequest) (*empty.Empty, error) {
	if err := s.PrePrepare(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	tconn, err := s.CR.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	role := ui.RoleType(req.Role)
	timeout := time.Duration(req.Timeout)
	mtbfui.WaitForElement(ctx, tconn, role, req.Name, timeout*time.Millisecond)

	return &empty.Empty{}, nil
}

func (s *UI) ClickElement(ctx context.Context, req *server.ClickElementRequest) (*empty.Empty, error) {
	if err := s.PrePrepare(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	tconn, err := s.CR.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	role := ui.RoleType(req.Role)
	mtbfui.ClickElement(ctx, tconn, role, req.Name)

	return &empty.Empty{}, nil
}
