// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	ppb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ppb.RegisterRollbackServiceServer(srv, &RollbackService{s: s})
		},
	})
}

// RollbackService implements tast.cros.policy.RollbackService.
type RollbackService struct {
	s *testing.ServiceState
}

func (Rollback *RollbackService) VerifyRollback(ctx context.Context, req *empty.Empty) (*ppb.RollbackSuccessfulResponse, error) {
	response := &ppb.RollbackSuccessfulResponse{}
	response.RollbackSuccessful = false

	cr, err := chrome.New(ctx, chrome.NoLogin())
	if err != nil {
		return response, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		return response, errors.Wrap(err, "failed to create OOBE connection")
	}
	defer oobeConn.Close()

	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.EnrollmentScreen.isVisible()"); err != nil {
		return response, errors.Wrap(err, "failed to wait for enrollment screen")
	}

	response.RollbackSuccessful = true
	return response, nil
}
