// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
)

// utilityCryptohomeLegacy implements |Utility| using
// |CryptohomeProxyLegacy|.
type utilityCryptohomeLegacy struct {
	utilityCommon
	proxy *CryptohomeProxyLegacy
}

func (utility utilityCryptohomeLegacy) IsTpmReady() (bool, error) {
	return utility.proxy.TpmIsReady()
}

func (utility utilityCryptohomeLegacy) IsPreparedForEnrollment() (bool, error) {
	return utility.proxy.TpmIsAttestationPrepared()
}

func (utility utilityCryptohomeLegacy) IsEnrolled() (bool, error) {
	return utility.proxy.TpmIsAttestationEnrolled()
}

func (utility utilityCryptohomeLegacy) TakeOwnership() (bool, error) {
	if err := utility.proxy.TpmCanAttemptOwnership(); err != nil {
		return false, err
	}
	return true, nil
}

func (utility utilityCryptohomeLegacy) IsMounted() (bool, error) {
	return false, errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) Unmount(username string) (bool, error) {
	return false, errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) CreateVault(username string, pass string) (bool, error) {
	return false, errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) RemoveVault(username string) (bool, error) {
	return false, errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) IsTpmWrappedKeySet(username string) (bool, error) {
	return false, errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) CreateEnrollRequest(PCAType int) (string, error) {
	return utility.proxy.TpmAttestationCreateEnrollRequest(PCAType)
}

func (utility utilityCryptohomeLegacy) FinishEnroll(PCAType int, resp string) error {
	result, err := utility.proxy.TpmAttestationEnroll(PCAType, resp)
	if err != nil {
		return errors.Wrap(err, "Failed to enroll")
	}
	if result != true {
		return errors.New("Failed to enroll")
	}
	return nil
}

func (utility utilityCryptohomeLegacy) CreateCertRequest(
	PCAType int,
	profile apb.CertificateProfile,
	username string,
	origin string) (string, error) {
	return utility.proxy.TpmAttestationCreateCertRequest(PCAType, int(profile), username, origin)
}

func (utility utilityCryptohomeLegacy) FinishCertRequest(resp string, username string, label string) error {
	cert, err := utility.proxy.TpmAttestationFinishCertRequest(resp, username != "", username, label)
	if err != nil {
		return errors.Wrap(err, "Failed to enroll")
	}
	if cert == "" {
		return errors.New("unexpected empty cert")
	}
	return nil
}

func (utility utilityCryptohomeLegacy) SignEnterpriseVAChallenge(
	VAType int,
	username string,
	label string,
	domain string,
	deviceID string,
	includeSignedPublicKey bool,
	challenge []byte) (string, error) {
	return utility.proxy.TpmAttestationSignEnterpriseVaChallengeSync(
		VAType,
		username != "",
		username,
		label,
		domain,
		deviceID,
		includeSignedPublicKey,
		challenge)
}

func (utility utilityCryptohomeLegacy) SignSimpleChallenge(
	username string,
	label string,
	challenge []byte) (string, error) {
	return "", errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) GetPublicKey(
	username string,
	label string) (string, error) {
	return "", errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) GetKeyPayload(
	username string,
	label string) (string, error) {
	return "", errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) SetKeyPayload(
	username string,
	label string,
	payload string) (bool, error) {
	return false, errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) RegisterKeyWithChapsToken(
	username string,
	label string) (bool, error) {
	return false, errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) GetEnrollmentId() (string, error) {
	return "", errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) GetOwnerPassword() (string, error) {
	return "", errors.New("Not implemented")
}

func (utility utilityCryptohomeLegacy) DeleteKeys(username string, prefix string) error {
	return errors.New("Not implemented")
}
