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

// utilityCryptohomeBinary implements |Utility| using
// |CryptohomeProxyLegacy|.
type utilityCryptohomeBinary struct {
	proxy                *CryptohomeBinary
	attestationAsyncMode bool
}

// NewUtilityCryptohomeBinary creates a new utilityCryptohomeBinary.
func NewUtilityCryptohomeBinary(r CmdRunner) (*utilityCryptohomeBinary, error) {
	proxy, err := NewCryptohomeBinary(r)
	if err != nil {
		return nil, err
	}
	return &utilityCryptohomeBinary{proxy, true}, nil
}

// IsTPMReady checks if TPM is ready.
func (utility *utilityCryptohomeBinary) IsTPMReady(ctx context.Context) (bool, error) {
	out, err := utility.proxy.TPMStatus(ctx)
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
func (utility *utilityCryptohomeBinary) IsPreparedForEnrollment(ctx context.Context) (bool, error) {
	out, err := utility.proxy.TPMAttestationStatus(ctx)
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
func (utility *utilityCryptohomeBinary) IsEnrolled(ctx context.Context) (bool, error) {
	out, err := utility.proxy.TPMAttestationStatus(ctx)
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
func (utility *utilityCryptohomeBinary) EnsureOwnership(ctx context.Context) (bool, error) {
	if err := utility.proxy.TPMTakeOwnership(ctx); err != nil {
		return false, errors.Wrap(err, "failed to take ownership")
	}
	if err := utility.proxy.TPMWaitOwnership(ctx); err != nil {
		return false, errors.Wrap(err, "failed to wait ownership")
	}
	return true, nil
}

// CreateEnrollRequest creates enroll request.
func (utility *utilityCryptohomeBinary) CreateEnrollRequest(ctx context.Context, pcaType int) (string, error) {
	return utility.proxy.TPMAttestationStartEnroll(ctx, pcaType, utility.attestationAsyncMode)
}

// FinishEnroll handles enroll response.
func (utility *utilityCryptohomeBinary) FinishEnroll(ctx context.Context, pcaType int, resp string) error {
	result, err := utility.proxy.TPMAttestationFinishEnroll(ctx, pcaType, resp, utility.attestationAsyncMode)
	if err != nil {
		return errors.Wrap(err, "failed to enroll")
	}
	if result != true {
		return errors.New("failed to enroll")
	}
	return nil
}

// CreateCertRequest creates a cert request.
func (utility *utilityCryptohomeBinary) CreateCertRequest(
	ctx context.Context,
	pcaType int,
	profile apb.CertificateProfile,
	username string,
	origin string) (string, error) {
	return utility.proxy.TPMAttestationStartCertRequest(ctx, pcaType, int(profile), username, origin, utility.attestationAsyncMode)
}

// FinishCertRequest handles cert response.
func (utility *utilityCryptohomeBinary) FinishCertRequest(ctx context.Context, resp string, username string, label string) error {
	cert, err := utility.proxy.TPMAttestationFinishCertRequest(ctx, resp, username, label, utility.attestationAsyncMode)
	if err != nil {
		return errors.Wrap(err, "failed to finish cert request")
	}
	if cert == "" {
		return errors.New("unexpected empty cert for " + username)
	}
	return nil
}

// SignEnterpriseVAChallenge performs SPKAC for the challenge.
func (utility *utilityCryptohomeBinary) SignEnterpriseVAChallenge(
	ctx context.Context,
	vaType int,
	username string,
	label string,
	domain string,
	deviceID string,
	includeSignedPublicKey bool,
	challenge []byte) (string, error) {
	if !includeSignedPublicKey {
		return "", errors.New("crytptohome binary always includes signed public key")
	}
	return utility.proxy.TPMAttestationEnterpriseVaChallenge(ctx,
		vaType,
		username,
		label,
		domain,
		deviceID,
		challenge)
}

// SignSimpleChallenge signs the challenge with the specified key.
func (utility *utilityCryptohomeBinary) SignSimpleChallenge(
	ctx context.Context,
	username string,
	label string,
	challenge []byte) (string, error) {
	return utility.proxy.TPMAttestationSimpleChallenge(ctx,
		username,
		label,
		challenge)
}

// GetPublicKey gets the public part of the key.
func (utility *utilityCryptohomeBinary) GetPublicKey(
	ctx context.Context,
	username string,
	label string) (string, error) {
	result, _, err := utility.getKeyStatus(ctx, username, label)
	if err != nil {
		return "", errors.Wrap(err, "failed to get key status")
	}
	return result, nil
}

// getKeyStatus gets the status of the key in attestation database.
func (utility *utilityCryptohomeBinary) getKeyStatus(
	ctx context.Context,
	username string,
	label string) (string, string, error) {
	out, err := utility.proxy.TPMAttestationKeyStatus(ctx,
		username,
		label)
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
func (utility *utilityCryptohomeBinary) IsMounted(ctx context.Context) (bool, error) {
	out, err := utility.proxy.IsMounted(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to check if mounted")
	}
	result, err := strconv.ParseBool(string(out))
	if err != nil {
		return false, errors.Wrap(err, "failed to parse output from cryptohome: "+string(out))
	}
	return result, nil
}

// Unmount unmounts the vault for |username|.
func (utility *utilityCryptohomeBinary) Unmount(ctx context.Context, username string) (bool, error) {
	out, err := utility.proxy.Unmount(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to unmount: "+string(out))
	}
	return true, nil
}

// CreateVault mounts the vault for |username|; creates a new vault if no vault yet.
func (utility *utilityCryptohomeBinary) CreateVault(ctx context.Context, username string, password string) (bool, error) {
	out, err := utility.proxy.MountEx(ctx, username, password, true)
	if err != nil {
		return false, errors.Wrap(err, "failed to mount: "+string(out))
	}
	return true, nil
}

// CheckVault checks the vault via |CheckKeyEx| dbus mehod.
func (utility *utilityCryptohomeBinary) CheckVault(ctx context.Context, username string, password string) (bool, error) {
	out, err := utility.proxy.CheckKeyEx(ctx, username, password)
	if err != nil {
		return false, errors.Wrap(err, "failed to check key: "+string(out))
	}
	return true, nil
}

// RemoveVault remove the vault for |username|.
func (utility *utilityCryptohomeBinary) RemoveVault(ctx context.Context, username string) (bool, error) {
	out, err := utility.proxy.Remove(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to remove vault: "+string(out))
	}
	return true, nil
}

// IsTPMWrappedKeySet checks if the current user vault is TPM-backed.
func (utility *utilityCryptohomeBinary) IsTPMWrappedKeySet(ctx context.Context, username string) (bool, error) {
	out, err := utility.proxy.DumpKeyset(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to dump keyset: "+string(out))
	}
	return strings.Contains(string(out), cryptohomeWrappedKeysetString), nil
}

// GetEnrollmentID gets the enrollment ID.
func (utility *utilityCryptohomeBinary) GetEnrollmentID(ctx context.Context) (string, error) {
	out, err := utility.proxy.GetEnrollmentID(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get EID: "+string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// GetOwnerPassword gets the TPM owner password.
func (utility *utilityCryptohomeBinary) GetOwnerPassword(ctx context.Context) (string, error) {
	out, err := utility.proxy.TPMStatus(ctx)
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

// ClearOwnerPassword clears TPM owner password in the best effort.
func (utility *utilityCryptohomeBinary) ClearOwnerPassword(ctx context.Context) error {
	out, err := utility.proxy.TPMClearStoredPassword(ctx)
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// GetKeyPayload gets the payload associated with the specified key.
func (utility *utilityCryptohomeBinary) GetKeyPayload(
	ctx context.Context,
	username string,
	label string) (string, error) {
	out, err := utility.proxy.TPMAttestationGetKeyPayload(ctx, username, label)
	if err != nil {
		return "", errors.Wrap(err, "failed to get key payload: "+string(out))
	}
	return string(out), nil
}

// SetKeyPayload sets the payload associated with the specified key.
func (utility *utilityCryptohomeBinary) SetKeyPayload(
	ctx context.Context,
	username string,
	label string,
	payload string) (bool, error) {
	out, err := utility.proxy.TPMAttestationSetKeyPayload(ctx, username, label, payload)
	if err != nil {
		return false, errors.Wrap(err, "failed to set key payload: "+string(out))
	}
	return true, nil
}

// RegisterKeyWithChapsToken registers the key into chaps.
func (utility *utilityCryptohomeBinary) RegisterKeyWithChapsToken(
	ctx context.Context,
	username string,
	label string) (bool, error) {
	out, err := utility.proxy.TPMAttestationRegisterKey(ctx, username, label)
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
func (utility *utilityCryptohomeBinary) SetAttestationAsyncMode(ctx context.Context, async bool) error {
	utility.attestationAsyncMode = async
	return nil
}

// DeleteKeys delete all he |usernames|'s keys with label having |prefix|.
func (utility *utilityCryptohomeBinary) DeleteKeys(ctx context.Context, username string, prefix string) error {
	out, err := utility.proxy.TPMAttestationDelete(ctx, username, prefix)
	if err != nil {
		return errors.Wrap(err, "failed to delete keys: "+string(out))
	}
	return nil
}
