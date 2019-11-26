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

// IsTpmReady implements Utility interface using CryptohomeBinary; see utility.go for more information.
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

// IsPreparedForEnrollment implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) IsPreparedForEnrollment() (bool, error) {
	out, err := utility.proxy.TpmAttestationStatus()
	if err != nil {
		return false, errors.Wrap(err, "failed to call cryptohome")
	}
	if strings.Contains(out, tpmIsAttestationPreparedString) {
		return true, nil
	}
	if strings.Contains(out, tpmIsNotAttestationPreparedString) {
		return false, nil
	}
	return false, errors.New("unexpected output from |cryptohome|")
}

// IsEnrolled implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) IsEnrolled() (bool, error) {
	out, err := utility.proxy.TpmAttestationStatus()
	if err != nil {
		return false, errors.Wrap(err, "failed to call cryptohome")
	}
	if strings.Contains(out, tpmIsAttestationEnrolledString) {
		return true, nil
	}
	if strings.Contains(out, tpmIsNotAttestationEnrolledString) {
		return false, nil
	}
	return false, errors.New("unexpected output from |cryptohome|")
}

// EnsureOwnership implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) EnsureOwnership() (bool, error) {
	if err := utility.proxy.TpmTakeOwnership(); err != nil {
		return false, errors.Wrap(err, "failed to take ownership")
	}
	if err := utility.proxy.TpmWaitOwnership(); err != nil {
		return false, errors.Wrap(err, "failed to wait ownership")
	}
	return true, nil
}

// CreateEnrollRequest implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) CreateEnrollRequest(PCAType int) (string, error) {
	return utility.proxy.TpmAttestationStartEnroll(PCAType, *utility.attestationAsyncMode)
}

// FinishEnroll implements Utility interface using CryptohomeBinary; see utility.go for more information.
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

// CreateCertRequest implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) CreateCertRequest(
	PCAType int,
	profile apb.CertificateProfile,
	username string,
	origin string) (string, error) {
	return utility.proxy.TpmAttestationStartCertRequest(PCAType, int(profile), username, origin, *utility.attestationAsyncMode)
}

// FinishCertRequest implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) FinishCertRequest(resp string, username string, label string) error {
	cert, err := utility.proxy.TpmAttestationFinishCertRequest(resp, username, label, *utility.attestationAsyncMode)
	if err != nil {
		return errors.Wrap(err, "Failed to finish cert request")
	}
	if cert == "" {
		return errors.New("unexpected empty cert")
	}
	return nil
}

// SignEnterpriseVAChallenge implements Utility interface using CryptohomeBinary; see utility.go for more information.
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
		username,
		label,
		domain,
		deviceID,
		challenge)
}

// SignSimpleChallenge implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) SignSimpleChallenge(
	username string,
	label string,
	challenge []byte) (string, error) {
	return utility.proxy.TpmAttestationSimpleChallenge(
		username,
		label,
		challenge)
}

// GetPublicKey implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) GetPublicKey(
	username string,
	label string) (string, error) {
	result, _, err := utility.getKeyStatus(username, label)
	if err != nil {
		return "", errors.Wrap(err, "failed to get key status")
	}
	return result, nil
}

func (utility utilityCryptohomeBinary) getKeyStatus(
	username string,
	label string) (string, string, error) {
	out, err := utility.proxy.TpmAttestationKeyStatus(
		username,
		label)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get key status")
	}
	arr := strings.Split(out, "\n")
	// Sanity check on output format
	if len(arr) != 6 || arr[0] != "Public Key:" || arr[3] != "Certificate:" {
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

// IsMounted implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) IsMounted() (bool, error) {
	out, err := utility.proxy.IsMounted()
	if err != nil {
		return false, errors.Wrap(err, "failed to check if mounted")
	}
	result, err := strconv.ParseBool(string(out))
	if err != nil {
		return false, errors.Wrap(err, "failed to parse output from cryptohome: "+string(out))
	}
	return result, nil
}

// Unmount implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) Unmount(username string) (bool, error) {
	out, err := utility.proxy.Unmount(username)
	if err != nil {
		return false, errors.Wrap(err, "failed to unmount: "+string(out))
	}
	return true, nil
}

// CreateVault implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) CreateVault(username string, password string) (bool, error) {
	out, err := utility.proxy.MountEx(username, password, true)
	if err != nil {
		return false, errors.Wrap(err, "failed to mount: "+string(out))
	}
	return true, nil
}

// RemoveVault implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) RemoveVault(username string) (bool, error) {
	out, err := utility.proxy.Remove(username)
	if err != nil {
		return false, errors.Wrap(err, "failed to remove vault: "+string(out))
	}
	return true, nil
}

// IsTpmWrappedKeySet implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) IsTpmWrappedKeySet(username string) (bool, error) {
	out, err := utility.proxy.DumpKeyset(username)
	if err != nil {
		return false, errors.Wrap(err, "failed to dump keyset: "+string(out))
	}
	return strings.Contains(string(out), cryptohomeWrappedKeysetString), nil
}

// GetEnrollmentID implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) GetEnrollmentID() (string, error) {
	out, err := utility.proxy.GetEnrollmentID()
	if err != nil {
		return "", errors.Wrap(err, "failed to get EID: "+string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// GetOwnerPassword implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) GetOwnerPassword() (string, error) {
	out, err := utility.proxy.TpmStatus()
	if err != nil {
		return "", errors.Wrap(err, "failed to get tpm status")
	}
	lastLine := func() string {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		return lines[len(lines)-1]
	}()
	fields := strings.Fields(lastLine)
	// Output doesn't match our expectation.
	if len(fields) < 2 || len(fields) > 3 || fields[0] != "TPM" || fields[1] != "Password:" {
		return "", errors.New("bad form of owner password: " + lastLine)
	}
	// Special case when the password is empty.
	if len(fields) == 2 {
		return "", nil
	}
	return fields[2], nil
}

// GetKeyPayload implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) GetKeyPayload(
	username string,
	label string) (string, error) {
	out, err := utility.proxy.TpmAttestationGetKeyPayload(username, label)
	if err != nil {
		return "", errors.Wrap(err, "failed to get key payload: "+string(out))
	}
	return string(out), nil
}

// SetKeyPayload implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) SetKeyPayload(
	username string,
	label string,
	payload string) (bool, error) {
	out, err := utility.proxy.TpmAttestationSetKeyPayload(username, label, payload)
	if err != nil {
		return false, errors.Wrap(err, "failed to set key payload: "+string(out))
	}
	return true, nil
}

// RegisterKeyWithChapsToken implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) RegisterKeyWithChapsToken(
	username string,
	label string) (bool, error) {
	out, err := utility.proxy.TpmAttestationRegisterKey(username, label)
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

// SetAttestationAsyncMode implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) SetAttestationAsyncMode(async bool) error {
	*utility.attestationAsyncMode = async
	return nil
}

// DeleteKeys implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility utilityCryptohomeBinary) DeleteKeys(username string, prefix string) error {
	out, err := utility.proxy.TpmAttestationDelete(username, prefix)
	if err != nil {
		return errors.Wrap(err, "failed to delete keys: "+string(out))
	}
	return nil
}

// parseTokenStatus parse the output of cryptohome --action=pkcs11_system_token_status or cryptohome --action=pkcs11_token_status and return the label, pin, slot and error (in that order).
func parseTokenStatus(cmdOutput string) (string, string, int, error) {
	arr := strings.Split(cmdOutput, "\n")
	if len(arr) != 5 {
		return "", "", -1, errors.New("Incorrect number of lines in token status, expected 5, got: " + strconv.Itoa(len(arr)))
	}

	// Parse label.
	const labelPrefix = "Label = "
	if !strings.HasPrefix(arr[1], labelPrefix) {
		return "", "", -1, errors.New("Cannot find label in token status")
	}
	label := arr[1][len(labelPrefix):]

	// Parse pin.
	const pinPrefix = "Pin = "
	if !strings.HasPrefix(arr[2], pinPrefix) {
		return "", "", -1, errors.New("Cannot find pin in token status")
	}
	pin := arr[2][len(pinPrefix):]

	// Parse slot.
	const slotPrefix = "Slot = "
	if !strings.HasPrefix(arr[3], slotPrefix) {
		return "", "", -1, errors.New("Cannot find slot in token status")
	}
	slot, err := strconv.Atoi(arr[3][len(slotPrefix):])
	if err != nil {
		return "", "", -1, errors.Wrap(err, "Token slot not integer")
	}

	return label, pin, slot, nil
}

func (utility utilityCryptohomeBinary) GetTokenForUser(username string) (int, error) {
	cmdOutput := ""
	if username == "" {
		// We want the system token.
		out, err := utility.proxy.Pkcs11SystemTokenStatus()
		if err != nil {
			return -1, errors.Wrap(err, "failed to get system token info: "+string(out))
		}
		cmdOutput = out
	} else {
		// We want the user token.
		out, err := utility.proxy.Pkcs11TokenStatus(username)
		if err != nil {
			return -1, errors.Wrap(err, "failed to get user token info: "+string(out))
		}
		cmdOutput = out
	}
	_, _, slot, err := parseTokenStatus(cmdOutput)
	if err != nil {
		return -1, errors.Wrap(err, "failed to parse token status: "+cmdOutput)
	}
	return slot, nil
}
