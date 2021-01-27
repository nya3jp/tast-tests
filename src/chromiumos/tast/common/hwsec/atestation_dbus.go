// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	apb "chromiumos/system_api/attestation_proto"
)

// AttestationDBus is an interface of attestation D-Bus client.
type AttestationDBus interface {
	// GetStatus returns the attestation status.
	GetStatus(ctx context.Context, req *apb.GetStatusRequest) (*apb.GetStatusReply, error)

	// CreateEnrollRequest create enroll request.
	CreateEnrollRequest(ctx context.Context, req *apb.CreateEnrollRequestRequest) (*apb.CreateEnrollRequestReply, error)

	// FinishEnroll finish enroll request.
	FinishEnroll(ctx context.Context, req *apb.FinishEnrollRequest) (*apb.FinishEnrollReply, error)

	// CreateCertificateRequest create certificate request.
	CreateCertificateRequest(ctx context.Context, req *apb.CreateCertificateRequestRequest) (*apb.CreateCertificateRequestReply, error)

	// FinishCertificateRequest finish certificate request.
	FinishCertificateRequest(ctx context.Context, req *apb.FinishCertificateRequestRequest) (*apb.FinishCertificateRequestReply, error)

	// SignEnterpriseChallenge sign enterprise challenge.
	SignEnterpriseChallenge(ctx context.Context, req *apb.SignEnterpriseChallengeRequest) (*apb.SignEnterpriseChallengeReply, error)

	// SignSimpleChallenge sign simple challenge.
	SignSimpleChallenge(ctx context.Context, req *apb.SignSimpleChallengeRequest) (*apb.SignSimpleChallengeReply, error)

	// GetKeyInfo returns the key info.
	GetKeyInfo(ctx context.Context, req *apb.GetKeyInfoRequest) (*apb.GetKeyInfoReply, error)

	// GetEnrollmentID returns the enrollment id.
	GetEnrollmentID(ctx context.Context, req *apb.GetEnrollmentIdRequest) (*apb.GetEnrollmentIdReply, error)
}
