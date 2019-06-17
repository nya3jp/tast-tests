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
	attestationAsyncMode *bool
}

// NewUtilityCryptohomeBinary creates a new utilityCryptohomeBinary.
func NewUtilityCryptohomeBinary(r CmdRunner) (*utilityCryptohomeBinary, error) {
	proxy, err := NewCryptohomeBinary(r)
	if err != nil {
		return nil, err
	}
	defaultAsynAttestationMode := true
	return &utilityCryptohomeBinary{proxy, &defaultAsynAttestationMode}, nil
}

// IsTPMReady implements Utility interface using CryptohomeBinary; see utility.go for more information.
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

// IsPreparedForEnrollment implements Utility interface using CryptohomeBinary; see utility.go for more information.
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

// IsEnrolled implements Utility interface using CryptohomeBinary; see utility.go for more information.
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

// EnsureOwnership implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) EnsureOwnership(ctx context.Context) (bool, error) {
	if err := utility.proxy.TPMTakeOwnership(ctx); err != nil {
		return false, errors.Wrap(err, "failed to take ownership")
	}
	if err := utility.proxy.TPMWaitOwnership(ctx); err != nil {
		return false, errors.Wrap(err, "failed to wait ownership")
	}
	return true, nil
}

// CreateEnrollRequest implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) CreateEnrollRequest(ctx context.Context, PCAType int) (string, error) {
	return utility.proxy.TPMAttestationStartEnroll(ctx, PCAType, *utility.attestationAsyncMode)
}

// FinishEnroll implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) FinishEnroll(ctx context.Context, PCAType int, resp string) error {
	result, err := utility.proxy.TPMAttestationFinishEnroll(ctx, PCAType, resp, *utility.attestationAsyncMode)
	if err != nil {
		return errors.Wrap(err, "Failed to enroll")
	}
	if result != true {
		return errors.New("Failed to enroll")
	}
	return nil
}

// CreateCertRequest implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) CreateCertRequest(
	ctx context.Context,
	PCAType int,
	profile apb.CertificateProfile,
	username string,
	origin string) (string, error) {
	return utility.proxy.TPMAttestationStartCertRequest(ctx, PCAType, int(profile), username, origin, *utility.attestationAsyncMode)
}

// FinishCertRequest implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) FinishCertRequest(ctx context.Context, resp string, username string, label string) error {
	cert, err := utility.proxy.TPMAttestationFinishCertRequest(ctx, resp, username, label, *utility.attestationAsyncMode)
	if err != nil {
		return errors.Wrap(err, "Failed to finish cert request")
	}
	if cert == "" {
		return errors.New("unexpected empty cert for " + username)
	}
	return nil
}

// SignEnterpriseVAChallenge implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) SignEnterpriseVAChallenge(
	ctx context.Context,
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
	return utility.proxy.TPMAttestationEnterpriseVaChallenge(ctx,
		VAType,
		username,
		label,
		domain,
		deviceID,
		challenge)
}

// SignSimpleChallenge implements Utility interface using CryptohomeBinary; see utility.go for more information.
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

// GetPublicKey implements Utility interface using CryptohomeBinary; see utility.go for more information.
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
		return "", "", errors.New("Ill-formed output string format")
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

// IsMounted implements Utility interface using CryptohomeBinary; see utility.go for more information.
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

// Unmount implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) Unmount(ctx context.Context, username string) (bool, error) {
	out, err := utility.proxy.Unmount(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to unmount: "+string(out))
	}
	return true, nil
}

// CreateVault implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) CreateVault(ctx context.Context, username string, password string) (bool, error) {
	out, err := utility.proxy.MountEx(ctx, username, password, true)
	if err != nil {
		return false, errors.Wrap(err, "failed to mount: "+string(out))
	}
	return true, nil
}

// CheckVault implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) CheckVault(ctx context.Context, username string, password string) (bool, error) {
	out, err := utility.proxy.CheckKeyEx(ctx, username, password)
	if err != nil {
		return false, errors.Wrap(err, "failed to check key: "+string(out))
	}
	return true, nil
}

// RemoveVault implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) RemoveVault(ctx context.Context, username string) (bool, error) {
	out, err := utility.proxy.Remove(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to remove vault: "+string(out))
	}
	return true, nil
}

// IsTPMWrappedKeySet implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) IsTPMWrappedKeySet(ctx context.Context, username string) (bool, error) {
	out, err := utility.proxy.DumpKeyset(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to dump keyset: "+string(out))
	}
	return strings.Contains(string(out), cryptohomeWrappedKeysetString), nil
}

// GetEnrollmentID implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) GetEnrollmentID(ctx context.Context) (string, error) {
	out, err := utility.proxy.GetEnrollmentID(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get EID: "+string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// GetOwnerPassword implements Utility interface using CryptohomeBinary; see utility.go for more information.
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

// ClearOwnerPassword implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) ClearOwnerPassword(ctx context.Context) error {
	out, err := utility.proxy.TPMClearStoredPassword(ctx)
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// GetKeyPayload implements Utility interface using CryptohomeBinary; see utility.go for more information.
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

// SetKeyPayload implements Utility interface using CryptohomeBinary; see utility.go for more information.
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

// RegisterKeyWithChapsToken implements Utility interface using CryptohomeBinary; see utility.go for more information.
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
	return false, errors.New("Unexpected output from cryptohome binary")
}

// SetAttestationAsyncMode implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) SetAttestationAsyncMode(ctx context.Context, async bool) error {
	*utility.attestationAsyncMode = async
	return nil
}

// DeleteKeys implements Utility interface using CryptohomeBinary; see utility.go for more information.
func (utility *utilityCryptohomeBinary) DeleteKeys(ctx context.Context, username string, prefix string) error {
	out, err := utility.proxy.TPMAttestationDelete(ctx, username, prefix)
	if err != nil {
		return errors.Wrap(err, "failed to delete keys: "+string(out))
	}
	return nil
}
