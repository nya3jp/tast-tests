// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
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

func getLastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

// UtilityCryptohomeBinary wraps and the functions of CryptohomeBinary and parses the outputs to
// structured data.
type UtilityCryptohomeBinary struct {
	binary *CryptohomeBinary
	// attestationAsyncMode enables the asynchronous communication between cryptohome and attestation sevice.
	// Note that from the CryptohomeBinary, the command is always blocking.
	attestationAsyncMode bool
}

// NewUtilityCryptohomeBinary creates a new UtilityCryptohomeBinary.
func NewUtilityCryptohomeBinary(r CmdRunner) (*UtilityCryptohomeBinary, error) {
	binary, err := NewCryptohomeBinary(r)
	if err != nil {
		return nil, err
	}
	return &UtilityCryptohomeBinary{binary, true}, nil
}

// IsTPMReady checks if TPM is ready.
func (u *UtilityCryptohomeBinary) IsTPMReady(ctx context.Context) (bool, error) {
	out, err := u.binary.TPMStatus(ctx)
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

// IsPreparedForEnrollment checks if prepared for enrollment.
func (u *UtilityCryptohomeBinary) IsPreparedForEnrollment(ctx context.Context) (bool, error) {
	out, err := u.binary.TPMAttestationStatus(ctx)
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

// IsEnrolled checks if DUT is enrolled.
func (u *UtilityCryptohomeBinary) IsEnrolled(ctx context.Context) (bool, error) {
	out, err := u.binary.TPMAttestationStatus(ctx)
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

// EnsureOwnership takes TPM ownership if found unowned.
func (u *UtilityCryptohomeBinary) EnsureOwnership(ctx context.Context) (bool, error) {
	if err := u.binary.TPMTakeOwnership(ctx); err != nil {
		return false, errors.Wrap(err, "failed to take ownership")
	}
	if err := u.binary.TPMWaitOwnership(ctx); err != nil {
		return false, errors.Wrap(err, "failed to wait ownership")
	}
	return true, nil
}

// CreateEnrollRequest creates enroll request.
func (u *UtilityCryptohomeBinary) CreateEnrollRequest(ctx context.Context, pcaType PCAType) (string, error) {
	return u.binary.TPMAttestationStartEnroll(ctx, pcaType, u.attestationAsyncMode)
}

// FinishEnroll handles enroll response.
func (u *UtilityCryptohomeBinary) FinishEnroll(ctx context.Context, pcaType PCAType, resp string) error {
	result, err := u.binary.TPMAttestationFinishEnroll(ctx, pcaType, resp, u.attestationAsyncMode)
	if err != nil {
		return errors.Wrap(err, "failed to enroll")
	}
	if !result {
		return errors.New("failed to enroll")
	}
	return nil
}

// CreateCertRequest creates a cert request.
func (u *UtilityCryptohomeBinary) CreateCertRequest(
	ctx context.Context,
	pcaType PCAType,
	profile apb.CertificateProfile,
	username string,
	origin string) (string, error) {
	return u.binary.TPMAttestationStartCertRequest(ctx, pcaType, int(profile), username, origin, u.attestationAsyncMode)
}

// FinishCertRequest handles cert response.
func (u *UtilityCryptohomeBinary) FinishCertRequest(ctx context.Context, resp string, username string, label string) error {
	cert, err := u.binary.TPMAttestationFinishCertRequest(ctx, resp, username, label, u.attestationAsyncMode)
	if err != nil {
		return errors.Wrap(err, "failed to finish cert request")
	}
	if cert == "" {
		return errors.Errorf("unexpected empty cert for %s", username)
	}
	return nil
}

// SignEnterpriseVAChallenge performs SPKAC for the challenge.
func (u *UtilityCryptohomeBinary) SignEnterpriseVAChallenge(
	ctx context.Context,
	vaType VAType,
	username string,
	label string,
	domain string,
	deviceID string,
	includeSignedPublicKey bool,
	challenge []byte) (string, error) {
	if !includeSignedPublicKey {
		return "", errors.New("crytptohome binary always includes signed public key")
	}
	return u.binary.TPMAttestationEnterpriseVaChallenge(ctx,
		vaType,
		username,
		label,
		domain,
		deviceID,
		challenge)
}

// SignSimpleChallenge signs the challenge with the specified key.
func (u *UtilityCryptohomeBinary) SignSimpleChallenge(
	ctx context.Context,
	username string,
	label string,
	challenge []byte) (string, error) {
	return u.binary.TPMAttestationSimpleChallenge(ctx,
		username,
		label,
		challenge)
}

// GetPublicKey gets the public part of the key.
func (u *UtilityCryptohomeBinary) GetPublicKey(
	ctx context.Context,
	username string,
	label string) (string, error) {
	result, _, err := u.getKeyStatus(ctx, username, label)
	if err != nil {
		return "", errors.Wrap(err, "failed to get key status")
	}
	return result, nil
}

// getKeyStatus gets the status of the key in attestation database.
func (u *UtilityCryptohomeBinary) getKeyStatus(
	ctx context.Context,
	username string,
	label string) (string, string, error) {
	out, err := u.binary.TPMAttestationKeyStatus(ctx, username, label)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get key status")
	}
	arr := strings.Split(out, "\n")
	// Sanity check on output format
	if len(arr) < 5 || arr[0] != "Public Key:" || arr[3] != "Certificate:" {
		return "", "", errors.New("ill-formed output string format")
	}
	if len(arr[1]) == 0 {
		return "", "", errors.New("empty public key")
	}
	cert := strings.Join(arr[4:], "\n")
	if len(cert) == 0 {
		return "", "", errors.New("empty cert")
	}
	return arr[1], cert, nil
}

// IsMounted checks if any vault is mounted.
func (u *UtilityCryptohomeBinary) IsMounted(ctx context.Context) (bool, error) {
	out, err := u.binary.IsMounted(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to check if mounted")
	}
	result, err := strconv.ParseBool(string(out))
	if err != nil {
		return false, errors.Wrap(err, "failed to parse output from cryptohome")
	}
	return result, nil
}

// Unmount unmounts the vault for |username|.
func (u *UtilityCryptohomeBinary) Unmount(ctx context.Context, username string) (bool, error) {
	_, err := u.binary.Unmount(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to unmount")
	}
	return true, nil
}

// CreateVault mounts the vault for |username|; creates a new vault if no vault yet.
func (u *UtilityCryptohomeBinary) CreateVault(ctx context.Context, username string, password string) (bool, error) {
	out, err := u.binary.MountEx(ctx, username, password, true)
	if err != nil {
		return false, errors.Wrap(err, "failed to mount: "+string(out))
	}
	return true, nil
}

// CheckVault checks the vault via |CheckKeyEx| dbus mehod.
func (u *UtilityCryptohomeBinary) CheckVault(ctx context.Context, username string, password string) (bool, error) {
	out, err := u.binary.CheckKeyEx(ctx, username, password)
	if err != nil {
		return false, errors.Wrap(err, "failed to check key: "+string(out))
	}
	return true, nil
}

// RemoveVault remove the vault for |username|.
func (u *UtilityCryptohomeBinary) RemoveVault(ctx context.Context, username string) (bool, error) {
	out, err := u.binary.Remove(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to remove vault: "+string(out))
	}
	return true, nil
}

// IsTPMWrappedKeySet checks if the current user vault is TPM-backed.
func (u *UtilityCryptohomeBinary) IsTPMWrappedKeySet(ctx context.Context, username string) (bool, error) {
	out, err := u.binary.DumpKeyset(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to dump keyset: "+string(out))
	}
	return strings.Contains(string(out), cryptohomeWrappedKeysetString), nil
}

// GetEnrollmentID gets the enrollment ID.
func (u *UtilityCryptohomeBinary) GetEnrollmentID(ctx context.Context) (string, error) {
	out, err := u.binary.GetEnrollmentID(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get EID: "+string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// GetOwnerPassword gets the TPM owner password.
func (u *UtilityCryptohomeBinary) GetOwnerPassword(ctx context.Context) (string, error) {
	out, err := u.binary.TPMStatus(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get tpm status")
	}
	lastLine := getLastLine(out)
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

// ClearOwnerPassword clears TPM owner password in the best effort.
func (u *UtilityCryptohomeBinary) ClearOwnerPassword(ctx context.Context) error {
	out, err := u.binary.TPMClearStoredPassword(ctx)
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// GetKeyPayload gets the payload associated with the specified key.
func (u *UtilityCryptohomeBinary) GetKeyPayload(
	ctx context.Context,
	username string,
	label string) (string, error) {
	out, err := u.binary.TPMAttestationGetKeyPayload(ctx, username, label)
	if err != nil {
		return "", errors.Wrap(err, "failed to get key payload: "+string(out))
	}
	return string(out), nil
}

// SetKeyPayload sets the payload associated with the specified key.
func (u *UtilityCryptohomeBinary) SetKeyPayload(
	ctx context.Context,
	username string,
	label string,
	payload string) (bool, error) {
	out, err := u.binary.TPMAttestationSetKeyPayload(ctx, username, label, payload)
	if err != nil {
		return false, errors.Wrap(err, "failed to set key payload: "+string(out))
	}
	return true, nil
}

// RegisterKeyWithChapsToken registers the key into chaps.
func (u *UtilityCryptohomeBinary) RegisterKeyWithChapsToken(
	ctx context.Context,
	username string,
	label string) (bool, error) {
	out, err := u.binary.TPMAttestationRegisterKey(ctx, username, label)
	if err != nil {
		return false, errors.Wrap(err, "failed to register key: "+string(out))
	}
	if strings.Contains(string(out), resultIsSuccessString) {
		return true, nil
	}
	if strings.Contains(string(out), resultIsFailureString) {
		return false, nil
	}
	return false, errors.New("unexpected output from cryptohome binary")
}

// SetAttestationAsyncMode switches the attestation mothods to sync/async mode respectively.
func (u *UtilityCryptohomeBinary) SetAttestationAsyncMode(ctx context.Context, async bool) error {
	u.attestationAsyncMode = async
	return nil
}

// DeleteKeys delete all he |usernames|'s keys with label having |prefix|.
func (u *UtilityCryptohomeBinary) DeleteKeys(ctx context.Context, username string, prefix string) error {
	out, err := u.binary.TPMAttestationDelete(ctx, username, prefix)
	if err != nil {
		return errors.Wrap(err, "failed to delete keys: "+string(out))
	}
	return nil
}
