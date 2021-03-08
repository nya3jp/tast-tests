// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"encoding/hex"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
)

// AttestationError wraps the attestation error status.
type AttestationError struct {
	*errors.E
	apb.AttestationStatus
}

// AttestationClient wraps and the functions of AttestationDBus.
type AttestationClient struct {
	ac AttestationDBus
}

// NewAttestationClient creates a new AttestationClient.
func NewAttestationClient(ac AttestationDBus) *AttestationClient {
	return &AttestationClient{ac}
}

// hexEncode encode the []byte into hex-encoded []byte; also returns encountered error if any
func hexEncode(src []byte) []byte {
	dst := make([]byte, hex.EncodedLen(len(src)))
	hex.Encode(dst, src)
	return dst
}

// IsPreparedForEnrollment checks if prepared for enrollment.
func (u *AttestationClient) IsPreparedForEnrollment(ctx context.Context) (bool, error) {
	status, err := u.ac.GetStatus(ctx, &apb.GetStatusRequest{})
	if err != nil {
		return false, errors.Wrap(err, "failed to call |GetStatus|")
	}
	return status.GetPreparedForEnrollment(), nil
}

// IsEnrolled checks if DUT is enrolled.
func (u *AttestationClient) IsEnrolled(ctx context.Context) (bool, error) {
	status, err := u.ac.GetStatus(ctx, &apb.GetStatusRequest{})
	if err != nil {
		return false, errors.Wrap(err, "failed to call |GetStatus|")
	}
	return status.GetEnrolled(), nil
}

// CreateEnrollRequest creates enroll request.
func (u *AttestationClient) CreateEnrollRequest(ctx context.Context, pcaType PCAType) (string, error) {
	acaType := apb.ACAType(ACAType(pcaType))
	reply, err := u.ac.CreateEnrollRequest(ctx, &apb.CreateEnrollRequestRequest{AcaType: &acaType})
	if err != nil {
		return "", errors.Wrap(err, "failed to call |CreateEnrollRequest|")
	}
	if reply.GetStatus() != apb.AttestationStatus_STATUS_SUCCESS {
		return "", &AttestationError{
			errors.Errorf("failed |CreateEnrollRequest|: %s", reply.GetStatus().String()),
			reply.GetStatus(),
		}
	}
	return string(reply.GetPcaRequest()), nil
}

// FinishEnroll handles enroll response.
func (u *AttestationClient) FinishEnroll(ctx context.Context, pcaType PCAType, resp string) error {
	acaType := apb.ACAType(ACAType(pcaType))
	reply, err := u.ac.FinishEnroll(ctx, &apb.FinishEnrollRequest{
		PcaResponse: []byte(resp),
		AcaType:     &acaType,
	})
	if err != nil {
		return errors.Wrap(err, "failed to call |FinishEnroll|")
	}
	if reply.GetStatus() != apb.AttestationStatus_STATUS_SUCCESS {
		return &AttestationError{
			errors.Errorf("failed |FinishEnroll|: %s", reply.GetStatus().String()),
			reply.GetStatus(),
		}
	}
	return nil
}

// CreateCertRequest creates a cert request.
func (u *AttestationClient) CreateCertRequest(
	ctx context.Context,
	pcaType PCAType,
	profile apb.CertificateProfile,
	username,
	origin string) (string, error) {
	acaType := apb.ACAType(ACAType(pcaType))
	reply, err := u.ac.CreateCertificateRequest(ctx, &apb.CreateCertificateRequestRequest{
		AcaType:            &acaType,
		CertificateProfile: &profile,
		Username:           &username,
		RequestOrigin:      &origin,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to call |CreateCertificateRequest|")
	}
	if reply.GetStatus() != apb.AttestationStatus_STATUS_SUCCESS {
		return "", &AttestationError{
			errors.Errorf("failed |CreateCertificateRequest|: %s", reply.GetStatus().String()),
			reply.GetStatus(),
		}
	}
	return string(reply.GetPcaRequest()), nil
}

// FinishCertRequest handles cert response.
func (u *AttestationClient) FinishCertRequest(ctx context.Context, resp, username, label string) error {
	reply, err := u.ac.FinishCertificateRequest(ctx, &apb.FinishCertificateRequestRequest{
		PcaResponse: []byte(resp),
		KeyLabel:    &label,
		Username:    &username,
	})
	if err != nil {
		return errors.Wrap(err, "failed to call |FinishCertificateRequest|")
	}
	if reply.GetStatus() != apb.AttestationStatus_STATUS_SUCCESS {
		return &AttestationError{
			errors.Errorf("failed |FinishCertificateRequest|: %s", reply.GetStatus().String()),
			reply.GetStatus(),
		}
	}
	return nil
}

// SignEnterpriseVAChallenge performs SPKAC for the challenge.
func (u *AttestationClient) SignEnterpriseVAChallenge(
	ctx context.Context,
	vaType VAType,
	username,
	label,
	domain,
	deviceID string,
	includeSignedPublicKey bool,
	challenge []byte) (string, error) {
	apbVAType := apb.VAType(vaType)
	reply, err := u.ac.SignEnterpriseChallenge(ctx, &apb.SignEnterpriseChallengeRequest{
		KeyLabel:               &label,
		Username:               &username,
		Domain:                 &domain,
		DeviceId:               []byte(deviceID),
		IncludeSignedPublicKey: &includeSignedPublicKey,
		Challenge:              challenge,
		VaType:                 &apbVAType,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to call |SignEnterpriseChallenge|")
	}
	if reply.GetStatus() != apb.AttestationStatus_STATUS_SUCCESS {
		return "", &AttestationError{
			errors.Errorf("failed |SignEnterpriseChallenge|: %s", reply.GetStatus().String()),
			reply.GetStatus(),
		}
	}
	return string(reply.GetChallengeResponse()), nil
}

// SignSimpleChallenge signs the challenge with the specified key.
func (u *AttestationClient) SignSimpleChallenge(
	ctx context.Context,
	username,
	label string,
	challenge []byte) (string, error) {
	reply, err := u.ac.SignSimpleChallenge(ctx, &apb.SignSimpleChallengeRequest{
		KeyLabel:  &label,
		Username:  &username,
		Challenge: challenge,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to call |SignSimpleChallenge|")
	}
	if reply.GetStatus() != apb.AttestationStatus_STATUS_SUCCESS {
		return "", &AttestationError{
			errors.Errorf("failed |SignSimpleChallenge|: %s", reply.GetStatus().String()),
			reply.GetStatus(),
		}
	}
	return string(reply.GetChallengeResponse()), nil
}

// GetPublicKey gets the public part of the key.
func (u *AttestationClient) GetPublicKey(
	ctx context.Context,
	username,
	label string) (string, error) {
	reply, err := u.ac.GetKeyInfo(ctx, &apb.GetKeyInfoRequest{
		KeyLabel: &label,
		Username: &username,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to call |GetKeyInfo|")
	}
	if reply.GetStatus() != apb.AttestationStatus_STATUS_SUCCESS {
		return "", &AttestationError{
			errors.Errorf("failed |GetKeyInfo|: %s", reply.GetStatus().String()),
			reply.GetStatus(),
		}
	}
	return string(hexEncode(reply.GetPublicKey())), nil
}

// GetEnrollmentID gets the enrollment ID.
func (u *AttestationClient) GetEnrollmentID(ctx context.Context) (string, error) {
	reply, err := u.ac.GetEnrollmentID(ctx, &apb.GetEnrollmentIdRequest{})
	if err != nil {
		return "", errors.Wrap(err, "failed to call |GetEnrollmentID|")
	}
	if reply.GetStatus() != apb.AttestationStatus_STATUS_SUCCESS {
		return "", &AttestationError{
			errors.Errorf("failed |GetEnrollmentID|: %s", reply.GetStatus().String()),
			reply.GetStatus(),
		}
	}
	return string(hexEncode([]byte(reply.GetEnrollmentId()))), nil
}
