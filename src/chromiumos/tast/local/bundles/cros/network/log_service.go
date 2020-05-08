// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			network.RegisterLogServer(srv, &LogService{s: s})
		},
	})
}

// LogService implements tast.cros.network.Log gRPC service.
type LogService struct {
	s *testing.ServiceState
}

// Print writes something with ContextLogf.
func (s *LogService) Print(ctx context.Context, str *network.String) (*empty.Empty, error) {
	testing.ContextLogf(ctx, "Logging: %s", str.S)
	return &empty.Empty{}, nil
}
