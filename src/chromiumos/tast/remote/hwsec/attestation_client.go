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
