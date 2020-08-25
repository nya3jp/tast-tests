// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/mtbf/mtbfutil/common"
	"chromiumos/tast/local/mtbf/service"
	server "chromiumos/tast/services/mtbf/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			server.RegisterShelfServer(srv, &Shelf{service.New(s)})
		},
	})
}

// A Shelf implements the tast/services/mtbf/ui.ShelfServer interface.
type Shelf struct {
	service.Service
}

func (s *Shelf) OpenSystemTray(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	conn, err := s.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLog(ctx, "Shelf: open SystemTray")

	// make sure the shelf is shown before clicking the UI node
	if err := common.ShelfVisible(ctx, conn, func() error {
		params := ui.FindParams{ClassName: "UnifiedSystemTray"}
		return common.ClickElement(ctx, conn, params)
	}); err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeClickSystemTray, err)
	}

	return &empty.Empty{}, nil
}
