// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"google.golang.org/grpc"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/hwsec"
	hwsecpb "chromiumos/tast/services/cros/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			hwsecpb.RegisterAttestationDBusServiceServer(srv, &AttestationDBusService{s})
		},
	})
}

type AttestationDBusService struct {
	s *testing.ServiceState
}

func (*AttestationDBusService) GetStatus(ctx context.Context, request *apb.GetStatusRequest) (*apb.GetStatusReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.GetStatus(ctx, request)
}

func (*AttestationDBusService) CreateEnrollRequest(ctx context.Context, request *apb.CreateEnrollRequestRequest) (*apb.CreateEnrollRequestReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.CreateEnrollRequest(ctx, request)
}

func (*AttestationDBusService) FinishEnroll(ctx context.Context, request *apb.FinishEnrollRequest) (*apb.FinishEnrollReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.FinishEnroll(ctx, request)
}

func (*AttestationDBusService) CreateCertificateRequest(ctx context.Context, request *apb.CreateCertificateRequestRequest) (*apb.CreateCertificateRequestReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.CreateCertificateRequest(ctx, request)
}

func (*AttestationDBusService) FinishCertificateRequest(ctx context.Context, request *apb.FinishCertificateRequestRequest) (*apb.FinishCertificateRequestReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.FinishCertificateRequest(ctx, request)
}

func (*AttestationDBusService) SignEnterpriseChallenge(ctx context.Context, request *apb.SignEnterpriseChallengeRequest) (*apb.SignEnterpriseChallengeReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.SignEnterpriseChallenge(ctx, request)
}

func (*AttestationDBusService) SignSimpleChallenge(ctx context.Context, request *apb.SignSimpleChallengeRequest) (*apb.SignSimpleChallengeReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.SignSimpleChallenge(ctx, request)
}

func (*AttestationDBusService) GetKeyInfo(ctx context.Context, request *apb.GetKeyInfoRequest) (*apb.GetKeyInfoReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.GetKeyInfo(ctx, request)
}

func (*AttestationDBusService) GetEnrollmentID(ctx context.Context, request *apb.GetEnrollmentIdRequest) (*apb.GetEnrollmentIdReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.GetEnrollmentID(ctx, request)
}

func (*AttestationDBusService) SetKeyPayload(ctx context.Context, request *apb.SetKeyPayloadRequest) (*apb.SetKeyPayloadReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.SetKeyPayload(ctx, request)
}

func (*AttestationDBusService) RegisterKeyWithChapsToken(ctx context.Context, request *apb.RegisterKeyWithChapsTokenRequest) (*apb.RegisterKeyWithChapsTokenReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.RegisterKeyWithChapsToken(ctx, request)
}

func (*AttestationDBusService) DeleteKeys(ctx context.Context, request *apb.DeleteKeysRequest) (*apb.DeleteKeysReply, error) {
	ac, err := hwsec.NewAttestationDBus(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create attestation client")
	}
	return ac.DeleteKeys(ctx, request)
}
