// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			platform.RegisterUpstartServiceServer(srv, &UpstartService{s})
		},
	})
}

// UpstartService implements tast.cros.platform.UpstartService.
type UpstartService struct {
	s *testing.ServiceState
}

// CheckJob validates that the given upstart job is running.
func (*UpstartService) CheckJob(ctx context.Context, request *platform.CheckJobRequest) (*empty.Empty, error) {
	return &empty.Empty{}, upstart.CheckJob(ctx, request.JobName)
}
