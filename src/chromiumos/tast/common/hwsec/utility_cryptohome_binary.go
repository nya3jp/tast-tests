// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
)

const (
	tpmIsReadyString                       = "TPM Ready: true"
	tpmIsNotReadyString                    = "TPM Ready: false"
	tpmIsAttestationPreparedString         = "Attestation Prepared: true"
	tpmIsNotAttestationPreparedString      = "Attestation Prepared: false"
	tpmIsAttestationEnrolledString         = "Attestation Enrolled: true"
	tpmIsNotAttestationEnrolledString      = "Attestation Enrolled: false"
	resultIsSuccessString                  = "Result: Success"
	resultIsFailureString                  = "Result: Failure"
	cryptohomeWrappedKeysetString          = "TPM_WRAPPED"
	installAttributesFinalizeSuccessOutput = "InstallAttributesFinalize(): 1"
)

// UtilityCryptohomeBinary wraps and the functions of CryptohomeBinary and parses the outputs to
// structured data.
type UtilityCryptohomeBinary struct {
	proxy *CryptohomeBinary
	// attestationAsyncMode enables the asynchronous communication between cryptohome and attestation sevice.
	// Note that from the CryptohomeBinary, the command is always blocking.
	attestationAsyncMode bool
}

// NewUtilityCryptohomeBinary creates a new UtilityCryptohomeBinary.
func NewUtilityCryptohomeBinary(r CmdRunner) (*UtilityCryptohomeBinary, error) {
	proxy, err := NewCryptohomeBinary(r)
	if err != nil {
		return nil, err
	}
	return &UtilityCryptohomeBinary{proxy, true}, nil
}

// GetStatusJSON retrieves the a status string from cryptohome. The status string is in JSON format and holds the various cryptohome related status.
func (utility *UtilityCryptohomeBinary) GetStatusJSON(ctx context.Context) (map[string]interface{}, error) {
	s, err := utility.proxy.GetStatusString(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to call GetStatusString()")
	}

	var obj map[string]interface{}
	err = json.Unmarshal([]byte(s), &obj)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse JSON from GetStatusString(): '"+s+"'; ")
	}
	return obj, nil
}

// IsTPMReady checks if TPM is ready.
func (utility *UtilityCryptohomeBinary) IsTPMReady(ctx context.Context) (bool, error) {
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
func (utility *UtilityCryptohomeBinary) IsPreparedForEnrollment(ctx context.Context) (bool, error) {
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
func (utility *UtilityCryptohomeBinary) IsEnrolled(ctx context.Context) (bool, error) {
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
func (utility *UtilityCryptohomeBinary) EnsureOwnership(ctx context.Context) (bool, error) {
	if err := utility.proxy.TPMTakeOwnership(ctx); err != nil {
		return false, errors.Wrap(err, "failed to take ownership")
	}
	if err := utility.proxy.TPMWaitOwnership(ctx); err != nil {
		return false, errors.Wrap(err, "failed to wait ownership")
	}
	return true, nil
}

// CreateEnrollRequest creates enroll request.
func (utility *UtilityCryptohomeBinary) CreateEnrollRequest(ctx context.Context, pcaType int) (string, error) {
	return utility.proxy.TPMAttestationStartEnroll(ctx, pcaType, utility.attestationAsyncMode)
}

// FinishEnroll handles enroll response.
func (utility *UtilityCryptohomeBinary) FinishEnroll(ctx context.Context, pcaType int, resp string) error {
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
func (utility *UtilityCryptohomeBinary) CreateCertRequest(
	ctx context.Context,
	pcaType int,
	profile apb.CertificateProfile,
	username string,
	origin string) (string, error) {
	return utility.proxy.TPMAttestationStartCertRequest(ctx, pcaType, int(profile), username, origin, utility.attestationAsyncMode)
}

// FinishCertRequest handles cert response.
func (utility *UtilityCryptohomeBinary) FinishCertRequest(ctx context.Context, resp string, username string, label string) error {
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
func (utility *UtilityCryptohomeBinary) SignEnterpriseVAChallenge(
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
func (utility *UtilityCryptohomeBinary) SignSimpleChallenge(
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
func (utility *UtilityCryptohomeBinary) GetPublicKey(
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
func (utility *UtilityCryptohomeBinary) getKeyStatus(
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

// InstallAttributesGet retrieves the install attributes with the name of attributeName, and returns the tuple (value, error), whereby value is the value of the attributes, and error is nil iff the operation is successful, otherwise error is the error that occurred.
func (utility *UtilityCryptohomeBinary) InstallAttributesGet(ctx context.Context, attributeName string) (string, error) {
	out, err := utility.proxy.InstallAttributesGet(ctx, attributeName)
	if err != nil {
		return "", errors.Wrap(err, "failed to get Install Attributes: "+out)
	}
	// Strip the ending new line
	if out[len(out)-1] == '\n' {
		out = out[0 : len(out)-1]
	}
	return out, err
}

// InstallAttributesSet sets the install attributes with the name of attributeName with the value attributeValue, and returns error, whereby error is nill iff the operation is successful, otherwise error is the error that occurred.
func (utility *UtilityCryptohomeBinary) InstallAttributesSet(ctx context.Context, attributeName string, attributeValue string) error {
	out, err := utility.proxy.InstallAttributesSet(ctx, attributeName, attributeValue)
	if err != nil {
		return errors.Wrap(err, "failed to set Install Attributes: "+out)
	}
	return nil
}

// InstallAttributesFinalize finalizes the install attributes, and returns error encountered if any. error is nil iff the operation completes successfully.
func (utility *UtilityCryptohomeBinary) InstallAttributesFinalize(ctx context.Context) error {
	out, err := utility.proxy.InstallAttributesFinalize(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to finalize Install Attributes: "+out)
	}
	if !strings.Contains(out, installAttributesFinalizeSuccessOutput) {
		return errors.New("failed to finalize Install Attributes, incorrect output message: " + out)
	}
	return nil
}

// InstallAttributesCount retrieves the number of entries in install attributes. It returns count and error. error is nil iff the operation completes successfully, and in this case count holds the number of entries in install attributes.
func (utility *UtilityCryptohomeBinary) InstallAttributesCount(ctx context.Context) (int, error) {
	out, err := utility.proxy.InstallAttributesCount(ctx)
	if err != nil {
		return -1, errors.Wrap(err, "failed to query install attributes count: "+out)
	}
	var result int
	n, err := fmt.Sscanf(out, "InstallAttributesCount(): %d", &result)
	if err != nil {
		return -1, errors.Wrap(err, "failed to parse InstallAttributesCount output: "+out)
	}
	if n != 1 {
		return -1, errors.New("invalid InstallAttributesCount output: " + out)
	}
	return result, nil
}

// installAttributesBooleanHelper is a helper function that helps to parse the output of install attribute series of command that returns a boolean.
func installAttributesBooleanHelper(out string, err error, methodName string) (bool, error) {
	if err != nil {
		return false, errors.Wrap(err, "failed to run "+methodName+"(): "+out)
	}
	var result int
	n, err := fmt.Sscanf(out, methodName+"(): %d", &result)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse "+methodName+"() output: "+out)
	}
	if n != 1 {
		return false, errors.New("invalid " + methodName + "() output: " + out)
	}
	return result != 0, nil
}

// InstallAttributesIsReady checks if install attributes is ready, returns isReady and error. error is nil iff the operation completes successfully, and in this case isReady is whether install attributes is ready.
func (utility *UtilityCryptohomeBinary) InstallAttributesIsReady(ctx context.Context) (bool, error) {
	out, err := utility.proxy.InstallAttributesIsReady(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsReady")
}

// InstallAttributesIsSecure checks if install attributes is secure, returns isSecure and error. error is nil iff the operation completes successfully, and in this case isSecure is whether install attributes is secure.
func (utility *UtilityCryptohomeBinary) InstallAttributesIsSecure(ctx context.Context) (bool, error) {
	out, err := utility.proxy.InstallAttributesIsSecure(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsSecure")
}

// InstallAttributesIsInvalid checks if install attributes is invalid, returns isInvalid and error. error is nil iff the operation completes successfully, and in this case isInvalid is whether install attributes is invalid.
func (utility *UtilityCryptohomeBinary) InstallAttributesIsInvalid(ctx context.Context) (bool, error) {
	out, err := utility.proxy.InstallAttributesIsInvalid(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsInvalid")
}

// InstallAttributesIsFirstInstall checks if install attributes is the first install state, returns isFirstInstall and error. error is nil iff the operation completes successfully, and in this case isFirstInstall is whether install attributes is in the first install state.
func (utility *UtilityCryptohomeBinary) InstallAttributesIsFirstInstall(ctx context.Context) (bool, error) {
	out, err := utility.proxy.InstallAttributesIsFirstInstall(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsFirstInstall")
}

// IsMounted checks if any vault is mounted.
func (utility *UtilityCryptohomeBinary) IsMounted(ctx context.Context) (bool, error) {
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
func (utility *UtilityCryptohomeBinary) Unmount(ctx context.Context, username string) (bool, error) {
	out, err := utility.proxy.Unmount(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to unmount: "+string(out))
	}
	return true, nil
}

// CreateVault mounts the vault for |username|; creates a new vault if no vault yet.
func (utility *UtilityCryptohomeBinary) CreateVault(ctx context.Context, username string, password string) (bool, error) {
	out, err := utility.proxy.MountEx(ctx, username, password, true)
	if err != nil {
		return false, errors.Wrap(err, "failed to mount: "+string(out))
	}
	return true, nil
}

// CheckVault checks the vault via |CheckKeyEx| dbus mehod.
func (utility *UtilityCryptohomeBinary) CheckVault(ctx context.Context, username string, password string) (bool, error) {
	out, err := utility.proxy.CheckKeyEx(ctx, username, password)
	if err != nil {
		return false, errors.Wrap(err, "failed to check key: "+string(out))
	}
	return true, nil
}

// RemoveVault remove the vault for |username|.
func (utility *UtilityCryptohomeBinary) RemoveVault(ctx context.Context, username string) (bool, error) {
	out, err := utility.proxy.Remove(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to remove vault: "+string(out))
	}
	return true, nil
}

// IsTPMWrappedKeySet checks if the current user vault is TPM-backed.
func (utility *UtilityCryptohomeBinary) IsTPMWrappedKeySet(ctx context.Context, username string) (bool, error) {
	out, err := utility.proxy.DumpKeyset(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to dump keyset: "+string(out))
	}
	return strings.Contains(string(out), cryptohomeWrappedKeysetString), nil
}

// GetEnrollmentID gets the enrollment ID.
func (utility *UtilityCryptohomeBinary) GetEnrollmentID(ctx context.Context) (string, error) {
	out, err := utility.proxy.GetEnrollmentID(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get EID: "+string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// GetOwnerPassword gets the TPM owner password.
func (utility *UtilityCryptohomeBinary) GetOwnerPassword(ctx context.Context) (string, error) {
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
func (utility *UtilityCryptohomeBinary) ClearOwnerPassword(ctx context.Context) error {
	out, err := utility.proxy.TPMClearStoredPassword(ctx)
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// GetKeyPayload gets the payload associated with the specified key.
func (utility *UtilityCryptohomeBinary) GetKeyPayload(
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
func (utility *UtilityCryptohomeBinary) SetKeyPayload(
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
func (utility *UtilityCryptohomeBinary) RegisterKeyWithChapsToken(
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
func (utility *UtilityCryptohomeBinary) SetAttestationAsyncMode(ctx context.Context, async bool) error {
	utility.attestationAsyncMode = async
	return nil
}

// DeleteKeys delete all he |usernames|'s keys with label having |prefix|.
func (utility *UtilityCryptohomeBinary) DeleteKeys(ctx context.Context, username string, prefix string) error {
	out, err := utility.proxy.TPMAttestationDelete(ctx, username, prefix)
	if err != nil {
		return errors.Wrap(err, "failed to delete keys: "+string(out))
	}
	return nil
}
