// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
)

// UtilityAttestationClient wraps and the functions of AttestationClient.
type UtilityAttestationClient struct {
	ac AttestationClient
}

// NewUtilityAttestationClient creates a new UtilityAttestationClient.
func NewUtilityAttestationClient(ac AttestationClient) (*UtilityAttestationClient, error) {
	return &UtilityAttestationClient{ac}, nil
}

// IsPreparedForEnrollment checks if prepared for enrollment.
func (u *UtilityAttestationClient) IsPreparedForEnrollment(ctx context.Context) (bool, error) {
	status, err := u.ac.GetStatus(ctx, &apb.GetStatusRequest{})
	if err != nil {
		return false, errors.Wrap(err, "failed to call |GetStatus|")
	}
	return status.GetPreparedForEnrollment(), nil
}

// CreateEnrollRequest creates enroll request.
func (u *UtilityAttestationClient) CreateEnrollRequest(ctx context.Context, pcaType PCAType) (string, error) {
	acaType := apb.ACAType(ACAType(pcaType))
	reply, err := u.ac.CreateEnrollRequest(ctx, &apb.CreateEnrollRequestRequest{AcaType: &acaType})
	if err != nil {
		return "", errors.Wrap(err, "failed to call |CreateEnrollRequest|")
	}
	if reply.GetStatus() != apb.AttestationStatus_STATUS_SUCCESS {
		return "", errors.Errorf("failed |CreateEnrollRequest|: %s", reply.GetStatus().String())
	}
	return string(reply.GetPcaRequest()), nil
}
