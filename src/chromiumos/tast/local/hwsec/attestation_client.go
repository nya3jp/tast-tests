// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"github.com/godbus/dbus"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// AttestationClient talks to attestation service via D-Bus APIs.
type AttestationClient struct {
	obj dbus.BusObject
}

// NewAttestationClient connects to the D-Bus and use the result object to construct AttestationClient.
func NewAttestationClient(ctx context.Context) (*AttestationClient, error) {
	const (
		attestationName = "org.chromium.Attestation"
		attestationPath = dbus.ObjectPath("/org/chromium/Attestation")
	)
	_, obj, err := dbusutil.Connect(ctx, attestationName, attestationPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to dbus")
	}
	return &AttestationClient{obj}, nil
}

// GetKeyInfo calls "GetKeyInfo" D-Bus Interface.
func (c *AttestationClient) GetKeyInfo(ctx context.Context, req *apb.GetKeyInfoRequest) (*apb.GetKeyInfoReply, error) {
	var reply apb.GetKeyInfoReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.GetKeyInfo", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call GetKeyInfo D-Bus API")
	}
	return &reply, nil
}

// GetEndorsementInfo calls "GetEndorsementInfo" D-Bus Interface.
func (c *AttestationClient) GetEndorsementInfo(ctx context.Context, req *apb.GetEndorsementInfoRequest) (*apb.GetEndorsementInfoReply, error) {
	var reply apb.GetEndorsementInfoReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.GetEndorsementInfo", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call GetEndorsementInfo D-Bus API")
	}
	return &reply, nil
}

// GetAttestationKeyInfo calls "GetAttestationKeyInfo" D-Bus Interface.
func (c *AttestationClient) GetAttestationKeyInfo(ctx context.Context, req *apb.GetAttestationKeyInfoRequest) (*apb.GetAttestationKeyInfoReply, error) {
	var reply apb.GetAttestationKeyInfoReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.GetAttestationKeyInfo", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call GetAttestationKeyInfo D-Bus API")
	}
	return &reply, nil
}

// ActivateAttestationKey calls "ActivateAttestationKey" D-Bus Interface.
func (c *AttestationClient) ActivateAttestationKey(ctx context.Context, req *apb.ActivateAttestationKeyRequest) (*apb.ActivateAttestationKeyReply, error) {
	var reply apb.ActivateAttestationKeyReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.ActivateAttestationKey", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call ActivateAttestationKey D-Bus API")
	}
	return &reply, nil
}

// CreateCertifiableKey calls "CreateCertifiableKey" D-Bus Interface.
func (c *AttestationClient) CreateCertifiableKey(ctx context.Context, req *apb.CreateCertifiableKeyRequest) (*apb.CreateCertifiableKeyReply, error) {
	var reply apb.CreateCertifiableKeyReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.CreateCertifiableKey", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call CreateCertifiableKey D-Bus API")
	}
	return &reply, nil
}

// Decrypt calls "Decrypt" D-Bus Interface.
func (c *AttestationClient) Decrypt(ctx context.Context, req *apb.DecryptRequest) (*apb.DecryptReply, error) {
	var reply apb.DecryptReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.Decrypt", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call Decrypt D-Bus API")
	}
	return &reply, nil
}

// Sign calls "Sign" D-Bus Interface.
func (c *AttestationClient) Sign(ctx context.Context, req *apb.SignRequest) (*apb.SignReply, error) {
	var reply apb.SignReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.Sign", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call Sign D-Bus API")
	}
	return &reply, nil
}

// RegisterKeyWithChapsToken calls "RegisterKeyWithChapsToken" D-Bus Interface.
func (c *AttestationClient) RegisterKeyWithChapsToken(ctx context.Context, req *apb.RegisterKeyWithChapsTokenRequest) (*apb.RegisterKeyWithChapsTokenReply, error) {
	var reply apb.RegisterKeyWithChapsTokenReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.RegisterKeyWithChapsToken", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call RegisterKeyWithChapsToken D-Bus API")
	}
	return &reply, nil
}

// GetEnrollmentPreparations calls "GetEnrollmentPreparations" D-Bus Interface.
func (c *AttestationClient) GetEnrollmentPreparations(ctx context.Context, req *apb.GetEnrollmentPreparationsRequest) (*apb.GetEnrollmentPreparationsReply, error) {
	var reply apb.GetEnrollmentPreparationsReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.GetEnrollmentPreparations", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call GetEnrollmentPreparations D-Bus API")
	}
	return &reply, nil
}

// GetStatus calls "GetStatus" D-Bus Interface.
func (c *AttestationClient) GetStatus(ctx context.Context, req *apb.GetStatusRequest) (*apb.GetStatusReply, error) {
	var reply apb.GetStatusReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.GetStatus", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call GetStatus D-Bus API")
	}
	return &reply, nil
}

// Verify calls "Verify" D-Bus Interface.
func (c *AttestationClient) Verify(ctx context.Context, req *apb.VerifyRequest) (*apb.VerifyReply, error) {
	var reply apb.VerifyReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.Verify", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call Verify D-Bus API")
	}
	return &reply, nil
}

// CreateEnrollRequest calls "CreateEnrollRequest" D-Bus Interface.
func (c *AttestationClient) CreateEnrollRequest(ctx context.Context, req *apb.CreateEnrollRequestRequest) (*apb.CreateEnrollRequestReply, error) {
	var reply apb.CreateEnrollRequestReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.CreateEnrollRequest", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call CreateEnrollRequest D-Bus API")
	}
	return &reply, nil
}

// FinishEnroll calls "FinishEnroll" D-Bus Interface.
func (c *AttestationClient) FinishEnroll(ctx context.Context, req *apb.FinishEnrollRequest) (*apb.FinishEnrollReply, error) {
	var reply apb.FinishEnrollReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.FinishEnroll", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call FinishEnroll D-Bus API")
	}
	return &reply, nil
}

// CreateCertificateRequest calls "CreateCertificateRequest" D-Bus Interface.
func (c *AttestationClient) CreateCertificateRequest(ctx context.Context, req *apb.CreateCertificateRequestRequest) (*apb.CreateCertificateRequestReply, error) {
	var reply apb.CreateCertificateRequestReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.CreateCertificateRequest", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call CreateCertificateRequest D-Bus API")
	}
	return &reply, nil
}

// FinishCertificateRequest calls "FinishCertificateRequest" D-Bus Interface.
func (c *AttestationClient) FinishCertificateRequest(ctx context.Context, req *apb.FinishCertificateRequestRequest) (*apb.FinishCertificateRequestReply, error) {
	var reply apb.FinishCertificateRequestReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.FinishCertificateRequest", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call FinishCertificateRequest D-Bus API")
	}
	return &reply, nil
}

// Enroll calls "Enroll" D-Bus Interface.
func (c *AttestationClient) Enroll(ctx context.Context, req *apb.EnrollRequest) (*apb.EnrollReply, error) {
	var reply apb.EnrollReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.Enroll", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call Enroll D-Bus API")
	}
	return &reply, nil
}

// GetCertificate calls "GetCertificate" D-Bus Interface.
func (c *AttestationClient) GetCertificate(ctx context.Context, req *apb.GetCertificateRequest) (*apb.GetCertificateReply, error) {
	var reply apb.GetCertificateReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.GetCertificate", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call GetCertificate D-Bus API")
	}
	return &reply, nil
}

// SignEnterpriseChallenge calls "SignEnterpriseChallenge" D-Bus Interface.
func (c *AttestationClient) SignEnterpriseChallenge(ctx context.Context, req *apb.SignEnterpriseChallengeRequest) (*apb.SignEnterpriseChallengeReply, error) {
	var reply apb.SignEnterpriseChallengeReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.SignEnterpriseChallenge", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call SignEnterpriseChallenge D-Bus API")
	}
	return &reply, nil
}

// SignSimpleChallenge calls "SignSimpleChallenge" D-Bus Interface.
func (c *AttestationClient) SignSimpleChallenge(ctx context.Context, req *apb.SignSimpleChallengeRequest) (*apb.SignSimpleChallengeReply, error) {
	var reply apb.SignSimpleChallengeReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.SignSimpleChallenge", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call SignSimpleChallenge D-Bus API")
	}
	return &reply, nil
}

// SetKeyPayload calls "SetKeyPayload" D-Bus Interface.
func (c *AttestationClient) SetKeyPayload(ctx context.Context, req *apb.SetKeyPayloadRequest) (*apb.SetKeyPayloadReply, error) {
	var reply apb.SetKeyPayloadReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.SetKeyPayload", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call SetKeyPayload D-Bus API")
	}
	return &reply, nil
}

// DeleteKeys calls "DeleteKeys" D-Bus Interface.
func (c *AttestationClient) DeleteKeys(ctx context.Context, req *apb.DeleteKeysRequest) (*apb.DeleteKeysReply, error) {
	var reply apb.DeleteKeysReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.DeleteKeys", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call DeleteKeys D-Bus API")
	}
	return &reply, nil
}

// ResetIdentity calls "ResetIdentity" D-Bus Interface.
func (c *AttestationClient) ResetIdentity(ctx context.Context, req *apb.ResetIdentityRequest) (*apb.ResetIdentityReply, error) {
	var reply apb.ResetIdentityReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.ResetIdentity", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call ResetIdentity D-Bus API")
	}
	return &reply, nil
}

// GetEnrollmentID calls "GetEnrollmentID" D-Bus Interface.
func (c *AttestationClient) GetEnrollmentID(ctx context.Context, req *apb.GetEnrollmentIdRequest) (*apb.GetEnrollmentIdReply, error) {
	var reply apb.GetEnrollmentIdReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.GetEnrollmentId", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call GetEnrollmentId D-Bus API")
	}
	return &reply, nil
}

// GetCertifiedNvIndex calls "GetCertifiedNvIndex" D-Bus Interface.
func (c *AttestationClient) GetCertifiedNvIndex(ctx context.Context, req *apb.GetCertifiedNvIndexRequest) (*apb.GetCertifiedNvIndexReply, error) {
	var reply apb.GetCertifiedNvIndexReply
	if err := dbusutil.CallProtoMethod(ctx, c.obj, "org.chromium.Attestation.GetCertifiedNvIndex", req, &reply); err != nil {
		return nil, errors.Wrap(err, "failed to call GetCertifiedNvIndex D-Bus API")
	}
	return &reply, nil
}
