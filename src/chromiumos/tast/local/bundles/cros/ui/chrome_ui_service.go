// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterChromeUIServiceServer(srv, &ChromeUIService{})
		},
	})
}

// ChromeUIService implements the methods defined in ChromeUIServiceServer.
type ChromeUIService struct{}

// EnsureLoginScreen emulates log out, and ensures login screen.
func (c *ChromeUIService) EnsureLoginScreen(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to restart ui job")
	}
	return &empty.Empty{}, nil
}
