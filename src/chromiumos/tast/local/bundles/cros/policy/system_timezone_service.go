// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
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

func (c *SystemTimezoneService) TestSystemTimezone(ctx context.Context, req *pb.TestSystemTimezoneRequest) (*empty.Empty, error) {

	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		return nil, errors.Wrap(err, "failed to log out")
	}

	// Wait until the timezone is set.
	if err := testing.Poll(ctx, func(ctx context.Context) error {

		out, err := testexec.CommandContext(ctx, "date", "+%Z").Output()
		if err != nil {
			return errors.Wrap(err, "failed to get the timezone")
		}
		outStr := strings.TrimSpace(string(out))

		if outStr != req.Timezone {
			return errors.Errorf("unexpected timezone: got %q; want %q", outStr, req.Timezone)
		}

		return nil

	}, &testing.PollOptions{
		Timeout: 30 * time.Second,
	}); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}
