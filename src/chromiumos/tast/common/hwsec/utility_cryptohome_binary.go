// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"strconv"
	"strings"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
)

const (
	tpmIsReadyString                  = "TPM Ready: true"
	tpmIsNotReadyString               = "TPM Ready: false"
	tpmIsAttestationPreparedString    = "Attestation Prepared: true"
	tpmIsNotAttestationPreparedString = "Attestation Prepared: false"
	tpmIsAttestationEnrolledString    = "Attestation Enrolled: true"
	tpmIsNotAttestationEnrolledString = "Attestation Enrolled: false"
	resultIsSuccessString             = "Result: Success"
	resultIsFailureString             = "Result: Failure"
	cryptohomeWrappedKeysetString     = "TPM_WRAPPED"
)

// utilityCryptohomeBinary implements |Utility| using
// |CryptohomeProxyLegacy|.
type utilityCryptohomeBinary struct {
	utilityCommon
	proxy                *CryptohomeBinary
	attestationAsyncMode *bool
}

func (utility utilityCryptohomeBinary) IsTpmReady() (bool, error) {
	out, err := utility.proxy.TpmStatus()
	if err != nil {
		return false, errors.Wrap(err, "failed to call tpm_status")
	}
	if strings.Contains(out, tpmIsReadyString) {
		return true, nil
	}
	if strings.Contains(out, tpmIsNotReadyString) {
		return false, nil
	}
	return false, errors.New("unexpected output from |cryptohome|")
}
func (utility utilityCryptohomeBinary) IsPreparedForEnrollment() (bool, error) {
	out, err := utility.proxy.TpmAttestationStatus()
	if err != nil {
		return false, errors.Wrap(err, "failed to call cryptohome")
	}
	if strings.Contains(string(out), tpmIsAttestationPreparedString) {
		return true, nil
	}
	if strings.Contains(string(out), tpmIsNotAttestationPreparedString) {
		return false, nil
	}
	return false, errors.New("unexpected output from |cryptohome|")
}
func (utility utilityCryptohomeBinary) IsEnrolled() (bool, error) {
	out, err := utility.proxy.TpmAttestationStatus()
	if err != nil {
		return false, errors.Wrap(err, "failed to call cryptohome")
	}
	if strings.Contains(string(out), tpmIsAttestationEnrolledString) {
		return true, nil
	}
	if strings.Contains(string(out), tpmIsNotAttestationEnrolledString) {
		return false, nil
	}
	return false, errors.New("unexpected output from |cryptohome|")
}

func (utility utilityCryptohomeBinary) TakeOwnership() (bool, error) {
	if err := utility.proxy.TpmTakeOwnership(); err != nil {
		return false, errors.Wrap(err, "failed to take ownership")
	}
	if err := utility.proxy.TpmWaitOwnership(); err != nil {
		return false, errors.Wrap(err, "failed to wait for ownership")
	}
	return true, nil
}

func (utility utilityCryptohomeBinary) CreateEnrollRequest(PCAType int) (string, error) {
	return utility.proxy.TpmAttestationStartEnroll(PCAType, *utility.attestationAsyncMode)
}

func (utility utilityCryptohomeBinary) FinishEnroll(PCAType int, resp string) error {
	result, err := utility.proxy.TpmAttestationFinishEnroll(PCAType, resp, *utility.attestationAsyncMode)
	if err != nil {
		return errors.Wrap(err, "Failed to enroll")
	}
	if result != true {
		return errors.New("Failed to enroll")
	}
	return nil
}

func (utility utilityCryptohomeBinary) CreateCertRequest(
	PCAType int,
	profile apb.CertificateProfile,
	username string,
	origin string) (string, error) {
	return utility.proxy.TpmAttestationStartCertRequest(PCAType, int(profile), username, origin, *utility.attestationAsyncMode)
}

func (utility utilityCryptohomeBinary) FinishCertRequest(resp string, username string, label string) error {
	cert, err := utility.proxy.TpmAttestationFinishCertRequest(resp, username != "", username, label, *utility.attestationAsyncMode)
	if err != nil {
		return errors.Wrap(err, "Failed to finish cert request")
	}
	if cert == "" {
		return errors.New("unexpected empty cert")
	}
	return nil
}

func (utility utilityCryptohomeBinary) SignEnterpriseVAChallenge(
	VAType int,
	username string,
	label string,
	domain string,
	deviceID string,
	includeSignedPublicKey bool,
	challenge []byte) (string, error) {
	if !includeSignedPublicKey {
		return "", errors.New("crytptohome binary always includes signed public key")
	}
	return utility.proxy.TpmAttestationEnterpriseVaChallenge(
		VAType,
		username != "",
		username,
		label,
		domain,
		deviceID,
		challenge)
}

func (utility utilityCryptohomeBinary) SignSimpleChallenge(
	username string,
	label string,
	challenge []byte) (string, error) {
	return utility.proxy.TpmAttestationSimpleChallenge(
		username != "",
		username,
		label,
		challenge)
}

func (utility utilityCryptohomeBinary) GetPublicKey(
	username string,
	label string) (string, error) {
	if result, _, err := utility.getKeyStatus(username, label); err != nil {
		return "", errors.Wrap(err, "failed to get key status")
	} else {
		return result, nil
	}
}

func (utility utilityCryptohomeBinary) getKeyStatus(
	username string,
	label string) (string, string, error) {
	out, err := utility.proxy.TpmAttestationKeyStatus(
		username != "",
		username,
		label)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get key status")
	}
	arr := strings.Split(string(out), "\n")
	if len(arr) != 4 || arr[0] != "Public Key:" {
	}
	// Sanity check on output format
	if arr[0] != "Public Key:" || arr[3] != "Certificate:" {
		return "", "", errors.New("Ill-formed output string format")
	}
	if len(arr[1]) == 0 {
		return "", "", errors.New("empty public key")
	}
	if len(arr[4]) == 0 {
		return "", "", errors.New("empty cert")
	}
	return arr[1], arr[4], nil
}

func (utility utilityCryptohomeBinary) IsMounted() (bool, error) {
	out, err := utility.proxy.IsMounted()
	if err != nil {
		return false, errors.Wrap(err, "failed to check if mounted")
	}
	if result, err := strconv.ParseBool(string(out)); err != nil {
		return false, errors.Wrap(err, "failed to parse output from cryptohome: "+string(out))
	} else {
		return result, nil
	}
}
func (utility utilityCryptohomeBinary) Unmount(username string) (bool, error) {
	out, err := utility.proxy.Unmount(username)
	if err != nil {
		return false, errors.Wrap(err, "failed to unmount: "+string(out))
	}
	return true, nil
}

func (utility utilityCryptohomeBinary) CreateVault(username string, pass string) (bool, error) {
	out, err := utility.proxy.MountEx(username, pass, true)
	if err != nil {
		return false, errors.Wrap(err, "failed to mount: "+string(out))
	}
	return true, nil
}

func (utility utilityCryptohomeBinary) RemoveVault(username string) (bool, error) {
	out, err := utility.proxy.Remove(username)
	if err != nil {
		return false, errors.Wrap(err, "failed to remove vault: "+string(out))
	}
	return true, nil
}

func (utility utilityCryptohomeBinary) IsTpmWrappedKeySet(username string) (bool, error) {
	out, err := utility.proxy.DumpKeySet(username)
	if err != nil {
		return false, errors.Wrap(err, "failed to dump keyset: "+string(out))
	}
	return strings.Contains(string(out), cryptohomeWrappedKeysetString), nil
}

func (utility utilityCryptohomeBinary) GetEnrollmentId() (string, error) {
	out, err := utility.proxy.GetEnrollmentId()
	if err != nil {
		return "", errors.Wrap(err, "failed to get EID: "+string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func (utility utilityCryptohomeBinary) GetKeyPayload(
	username string,
	label string) (string, error) {
	out, err := utility.proxy.TpmAttestationGetKeyPayload(username != "", username, label)
	if err != nil {
		return "", errors.Wrap(err, "failed to get key payload: "+string(out))
	}
	return string(out), nil
}

func (utility utilityCryptohomeBinary) SetKeyPayload(
	username string,
	label string,
	payload string) (bool, error) {
	out, err := utility.proxy.TpmAttestationSetKeyPayload(username != "", username, label, payload)
	if err != nil {
		return false, errors.Wrap(err, "failed to set key payload: "+string(out))
	}
	return true, nil
}

func (utility utilityCryptohomeBinary) RegisterKeyWithChapsToken(
	username string,
	label string) (bool, error) {
	out, err := utility.proxy.TpmAttestationRegisterKey(username != "", username, label)
	if err != nil {
		return false, errors.Wrap(err, "failed to register key: "+string(out))
	}
	if strings.Contains(string(out), resultIsSuccessString) {
		return true, nil
	}
	if strings.Contains(string(out), resultIsFailureString) {
		return false, nil
	}
	return false, errors.New("Unexpected output from cryptohome binary")
}

func (utility utilityCryptohomeBinary) SetAttestationAsyncMode(async bool) error {
	*utility.attestationAsyncMode = async
	return nil
}

func (utility utilityCryptohomeBinary) DeleteKeys(username string, prefix string) error {
	out, err := utility.proxy.TpmAttestationDelete(username, prefix)
	if err != nil {
		return errors.Wrap(err, "failed to delete keys: "+string(out))
	}
	return nil
}
