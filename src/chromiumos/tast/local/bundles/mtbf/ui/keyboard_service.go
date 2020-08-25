// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/service"
	pb "chromiumos/tast/services/mtbf/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterKeyboardServiceServer(srv, &KeyboardService{service.New(s)})
		},
	})
}

type KeyboardService struct {
	service.Service
}

func (k *KeyboardService) Accel(ctx context.Context, req *pb.KeyboardAccelRequest) (*empty.Empty, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, err
	}
	defer kb.Close()

	if req.Times < 1 {
		return nil, errors.New("times need greater than 0")
	}

	for i := 0; i < int(req.Times); i++ {
		if err := kb.Accel(ctx, req.Command); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (k *KeyboardService) Type(ctx context.Context, req *pb.KeyboardTypeRequest) (*empty.Empty, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, err
	}
	defer kb.Close()

	if req.Text != "" {
		if err := kb.Type(ctx, req.Text); err != nil {
			return nil, err
		}
	}

	return nil, nil
}
