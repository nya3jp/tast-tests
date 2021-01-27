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
		return false, errors.Wrap(err, "failed to call attestation status")
	}
	return status.GetPreparedForEnrollment(), nil
}
