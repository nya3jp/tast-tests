// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	pb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterSystemTimezoneServiceServer(srv, &SystemTimezoneService{s: s})
		},
	})
}

// SystemTimezoneService implements tast.cros.policy.SystemTimezoneService.
type SystemTimezoneService struct { // NOLINT
	s *testing.ServiceState
}

func (c *SystemTimezoneService) GetSystemTimezone(ctx context.Context, req *empty.Empty) (*pb.GetSystemTimezoneResponse, error) {

	out, err := testexec.CommandContext(ctx, "date", "+%Z").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timezone")
	}
	outStr := strings.TrimSpace(string(out))

	return &pb.GetSystemTimezoneResponse{
		Timezone: outStr,
	}, nil
}
