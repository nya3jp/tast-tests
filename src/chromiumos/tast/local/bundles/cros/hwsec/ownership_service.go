// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	hwseclocal "chromiumos/tast/local/hwsec"
	hwsecpb "chromiumos/tast/services/cros/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			hwsecpb.RegisterOwnershipServiceServer(srv, &OwnershipService{s})
		},
	})
}

type OwnershipService struct {
	s *testing.ServiceState
}

// EnsureTPMIsReset calls the local EnsureTPMIsReset hwsec helpers.
func (*OwnershipService) EnsureTPMIsReset(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	testing.ContextLog(ctx, "Requesting a local TPM reset")

	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create local helper")
	}

	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to reset TPMd")
	}

	return &empty.Empty{}, nil
}

// EnsureTPMAndSystemStateAreReset calls the local EnsureTPMAndSystemStateAreReset hwsec helpers.
func (*OwnershipService) EnsureTPMAndSystemStateAreReset(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	testing.ContextLog(ctx, "Requesting a local TPM and state reset")

	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create local helper")
	}

	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to reset TPMd")
	}

	return &empty.Empty{}, nil
}

func (*OwnershipService) ResetDeviceToFactoryStateForZTE(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	testing.ContextLog(ctx, "Resetting device to factory state")

	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create local helper")
	}

	if err := helper.ResetDeviceToFactoryStateForZTE(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to reset device to factory state")
	}

	return &empty.Empty{}, nil
}
