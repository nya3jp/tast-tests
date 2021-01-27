// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	hwsecpb "chromiumos/tast/services/cros/hwsec"
	"chromiumos/tast/testing"
)

// AttestationClient talks to attestation service via gRPC D-Bus APIs.
type AttestationClient struct {
	dut     *dut.DUT
	rpcHint *testing.RPCHint
}

// NewAttestationClient use the dut and rpc hint objects to construct AttestationClient.
func NewAttestationClient(d *dut.DUT, r *testing.RPCHint) (*AttestationClient, error) {
	return &AttestationClient{d, r}, nil
}

// GetStatus calls "GetStatus" gRPC D-Bus Interface.
func (a *AttestationClient) GetStatus(ctx context.Context, req *apb.GetStatusRequest) (*apb.GetStatusReply, error) {
	cl, err := rpc.Dial(ctx, a.dut, a.rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	ac := hwsecpb.NewAttestationClientServiceClient(cl.Conn)
	return ac.GetStatus(ctx, req)
}

// CreateEnrollRequest calls "CreateEnrollRequest" gRPC D-Bus Interface.
func (a *AttestationClient) CreateEnrollRequest(ctx context.Context, req *apb.CreateEnrollRequestRequest) (*apb.CreateEnrollRequestReply, error) {
	cl, err := rpc.Dial(ctx, a.dut, a.rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	ac := hwsecpb.NewAttestationClientServiceClient(cl.Conn)
	return ac.CreateEnrollRequest(ctx, req)
}

// FinishEnroll calls "FinishEnroll" gRPC D-Bus Interface.
func (a *AttestationClient) FinishEnroll(ctx context.Context, req *apb.FinishEnrollRequest) (*apb.FinishEnrollReply, error) {
	cl, err := rpc.Dial(ctx, a.dut, a.rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	ac := hwsecpb.NewAttestationClientServiceClient(cl.Conn)
	return ac.FinishEnroll(ctx, req)
}

// CreateCertificateRequest calls "CreateCertificateRequest" gRPC D-Bus Interface.
func (a *AttestationClient) CreateCertificateRequest(ctx context.Context, req *apb.CreateCertificateRequestRequest) (*apb.CreateCertificateRequestReply, error) {
	cl, err := rpc.Dial(ctx, a.dut, a.rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	ac := hwsecpb.NewAttestationClientServiceClient(cl.Conn)
	return ac.CreateCertificateRequest(ctx, req)
}

// FinishCertificateRequest calls "FinishCertificateRequest" gRPC D-Bus Interface.
func (a *AttestationClient) FinishCertificateRequest(ctx context.Context, req *apb.FinishCertificateRequestRequest) (*apb.FinishCertificateRequestReply, error) {
	cl, err := rpc.Dial(ctx, a.dut, a.rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	ac := hwsecpb.NewAttestationClientServiceClient(cl.Conn)
	return ac.FinishCertificateRequest(ctx, req)
}

// SignEnterpriseChallenge calls "SignEnterpriseChallenge" gRPC D-Bus Interface.
func (a *AttestationClient) SignEnterpriseChallenge(ctx context.Context, req *apb.SignEnterpriseChallengeRequest) (*apb.SignEnterpriseChallengeReply, error) {
	cl, err := rpc.Dial(ctx, a.dut, a.rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	ac := hwsecpb.NewAttestationClientServiceClient(cl.Conn)
	return ac.SignEnterpriseChallenge(ctx, req)
}

// SignSimpleChallenge calls "SignSimpleChallenge" gRPC D-Bus Interface.
func (a *AttestationClient) SignSimpleChallenge(ctx context.Context, req *apb.SignSimpleChallengeRequest) (*apb.SignSimpleChallengeReply, error) {
	cl, err := rpc.Dial(ctx, a.dut, a.rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	ac := hwsecpb.NewAttestationClientServiceClient(cl.Conn)
	return ac.SignSimpleChallenge(ctx, req)
}

// GetKeyInfo calls "GetKeyInfo" gRPC D-Bus Interface.
func (a *AttestationClient) GetKeyInfo(ctx context.Context, req *apb.GetKeyInfoRequest) (*apb.GetKeyInfoReply, error) {
	cl, err := rpc.Dial(ctx, a.dut, a.rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	ac := hwsecpb.NewAttestationClientServiceClient(cl.Conn)
	return ac.GetKeyInfo(ctx, req)
}

// GetEnrollmentID calls "GetEnrollmentID" gRPC D-Bus Interface.
func (a *AttestationClient) GetEnrollmentID(ctx context.Context, req *apb.GetEnrollmentIdRequest) (*apb.GetEnrollmentIdReply, error) {
	cl, err := rpc.Dial(ctx, a.dut, a.rpcHint, "cros")
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	ac := hwsecpb.NewAttestationClientServiceClient(cl.Conn)
	return ac.GetEnrollmentID(ctx, req)
}
