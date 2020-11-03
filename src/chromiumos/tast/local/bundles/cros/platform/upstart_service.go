// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

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

type UpstartService struct {
	s *testing.ServiceState
}

func (*UpstartService) CheckJob(ctx context.Context, request *platform.CheckJobRequest) (*platform.CheckJobResponse, error) {
	resp := platform.CheckJobResponse{}
	err := upstart.CheckJob(ctx, request.JobName)
	if err != nil {
		resp.Error = err.Error()
	}

	return &resp, nil
}
