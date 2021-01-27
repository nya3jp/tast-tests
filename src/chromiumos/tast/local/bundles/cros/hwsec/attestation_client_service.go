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

func (*AttestationClientService) FinishEnroll(ctx context.Context, request *attestation_proto.FinishEnrollRequest) (*attestation_proto.FinishEnrollReply, error) {
	ac, err := hwsec.NewAttestationClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.FinishEnroll(ctx, request)
}

func (*AttestationClientService) CreateCertificateRequest(ctx context.Context, request *attestation_proto.CreateCertificateRequestRequest) (*attestation_proto.CreateCertificateRequestReply, error) {
	ac, err := hwsec.NewAttestationClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.CreateCertificateRequest(ctx, request)
}

func (*AttestationClientService) FinishCertificateRequest(ctx context.Context, request *attestation_proto.FinishCertificateRequestRequest) (*attestation_proto.FinishCertificateRequestReply, error) {
	ac, err := hwsec.NewAttestationClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.FinishCertificateRequest(ctx, request)
}

func (*AttestationClientService) SignEnterpriseChallenge(ctx context.Context, request *attestation_proto.SignEnterpriseChallengeRequest) (*attestation_proto.SignEnterpriseChallengeReply, error) {
	ac, err := hwsec.NewAttestationClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.SignEnterpriseChallenge(ctx, request)
}

func (*AttestationClientService) SignSimpleChallenge(ctx context.Context, request *attestation_proto.SignSimpleChallengeRequest) (*attestation_proto.SignSimpleChallengeReply, error) {
	ac, err := hwsec.NewAttestationClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.SignSimpleChallenge(ctx, request)
}

func (*AttestationClientService) GetKeyInfo(ctx context.Context, request *attestation_proto.GetKeyInfoRequest) (*attestation_proto.GetKeyInfoReply, error) {
	ac, err := hwsec.NewAttestationClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.GetKeyInfo(ctx, request)
}

func (*AttestationClientService) GetEnrollmentID(ctx context.Context, request *attestation_proto.GetEnrollmentIdRequest) (*attestation_proto.GetEnrollmentIdReply, error) {
	ac, err := hwsec.NewAttestationClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.GetEnrollmentID(ctx, request)
}
