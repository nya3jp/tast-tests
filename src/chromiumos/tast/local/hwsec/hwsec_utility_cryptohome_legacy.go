// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
)

// hwsecUtilityCryptohomeLegacy implements HwsecUtility using
// |CryptohomeProxyLegacy|.
type hwsecUtilityCryptohomeLegacy struct {
	utilityCommon
	proxy *CryptohomeProxyLegacy
}

func (utility hwsecUtilityCryptohomeLegacy) IsTpmReady() (bool, error) {
	return utility.proxy.TpmIsReady()
}
func (utility hwsecUtilityCryptohomeLegacy) IsPreparedForEnrollment() (bool, error) {
	return utility.proxy.TpmIsAttestationPrepared()
}
func (utility hwsecUtilityCryptohomeLegacy) IsEnrolled() (bool, error) {
	return utility.proxy.TpmIsAttestationEnrolled()
}

func (utility hwsecUtilityCryptohomeLegacy) TakeOwnership() (bool, error) {
	if err := utility.proxy.TpmCanAttemptOwnership(); err != nil {
		return false, err
	}
	return true, nil
}

func (utility hwsecUtilityCryptohomeLegacy) CreateEnrollRequest(PCAType int) (string, error) {
	return utility.proxy.TpmAttestationCreateEnrollRequest(PCAType)
}

func (utility hwsecUtilityCryptohomeLegacy) FinishEnroll(PCAType int, resp string) error {
	result, err := utility.proxy.TpmAttestationEnroll(PCAType, resp)
	if err != nil {
		return errors.Wrap(err, "Failed to enroll")
	}
	if result != true {
		return errors.New("Failed to enroll")
	}
	return nil
}

func (utility hwsecUtilityCryptohomeLegacy) CreateCertRequest(PCAType int, profile apb.CertificateProfile, username string, origin string) (string, error) {
	return utility.proxy.TpmAttestationCreateCertRequest(PCAType, int(profile), username, origin)
}

func (utility hwsecUtilityCryptohomeLegacy) FinishCertRequest(resp string, username string, label string) error {
	cert, err := utility.proxy.TpmAttestationFinishCertRequest(resp, username != "", username, label)
	if err != nil {
		return errors.Wrap(err, "Failed to enroll")
	}
	if cert == "" {
		return errors.New("unexpected empty cert")
	}
	return nil
}

func (utility hwsecUtilityCryptohomeLegacy) SignEnterpriseVAChallenge(
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
