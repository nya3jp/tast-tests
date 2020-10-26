// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	pb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterDeviceMinimumVersionServiceServer(srv, &DeviceMinimumVersionService{s: s})
		},
	})
}

// DeviceMinimumVersionService implements tast.cros.policy.DeviceMinimumVersionService.
type DeviceMinimumVersionService struct { // NOLINT
	s *testing.ServiceState
}

// TestUpdateRequiredScreenIsVisible creates a new instance of Chrome using the state from the existing one and
// checks that an update required screen with update now button is visible on the login page.
// Chrome is closed when function exists.
func (c *DeviceMinimumVersionService) TestUpdateRequiredScreenIsVisible(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
	)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create OOBE connection")
	}
	defer oobeConn.Close()

	if err := oobeConn.WaitForExprFailOnErr(ctx, "!document.querySelector('update-required-card[hidden]')"); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to wait for the update required screen to be visible")
	}

	return &empty.Empty{}, nil
}
