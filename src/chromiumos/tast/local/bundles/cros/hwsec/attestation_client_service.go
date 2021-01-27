// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"google.golang.org/grpc"

	"chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/hwsec"
	hwsecpb "chromiumos/tast/services/cros/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			hwsecpb.RegisterAttestationClientServiceServer(srv, &AttestationClientService{s})
		},
	})
}

type AttestationClientService struct {
	s *testing.ServiceState
}

func (*AttestationClientService) GetStatus(ctx context.Context, request *attestation_proto.GetStatusRequest) (*attestation_proto.GetStatusReply, error) {
	ac, err := hwsec.NewAttestationClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.GetStatus(ctx, request)
}

func (*AttestationClientService) CreateEnrollRequest(ctx context.Context, request *attestation_proto.CreateEnrollRequestRequest) (*attestation_proto.CreateEnrollRequestReply, error) {
	ac, err := hwsec.NewAttestationClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.CreateEnrollRequest(ctx, request)
}
