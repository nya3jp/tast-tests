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
	"chromiumos/tast/testing"
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
	listKeysExLabelPrefix                  = "Label: "
	addKeyExSuccessMessage                 = "Key added."
	removeKeyExSuccessMessage              = "Key removed."
	migrateKeyExSucessMessage              = "Key migration succeeded."
	updateKeyExSuccessMessage              = "Key updated."
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

// GetStatusJSON retrieves the a status string from cryptohome. The status string is in JSON format and holds the various cryptohome related status.
func (u *UtilityCryptohomeBinary) GetStatusJSON(ctx context.Context) (map[string]interface{}, error) {
	s, err := u.binary.GetStatusString(ctx)
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

// InstallAttributesGet retrieves the install attributes with the name of attributeName, and returns the tuple (value, error), whereby value is the value of the attributes, and error is nil iff the operation is successful, otherwise error is the error that occurred.
func (u *UtilityCryptohomeBinary) InstallAttributesGet(ctx context.Context, attributeName string) (string, error) {
	out, err := u.binary.InstallAttributesGet(ctx, attributeName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get Install Attributes with the following output %q", out)
	}
	// Strip the ending new line.
	out = strings.TrimSuffix(out, "\n")

	return out, err
}

// InstallAttributesSet sets the install attributes with the name of attributeName with the value attributeValue, and returns error, whereby error is nil iff the operation is successful, otherwise error is the error that occurred.
func (u *UtilityCryptohomeBinary) InstallAttributesSet(ctx context.Context, attributeName, attributeValue string) error {
	out, err := u.binary.InstallAttributesSet(ctx, attributeName, attributeValue)
	if err != nil {
		return errors.Wrapf(err, "failed to set Install Attributes with the following output %q", out)
	}
	return nil
}

// InstallAttributesFinalize finalizes the install attributes, and returns error encountered if any. error is nil iff the operation completes successfully.
func (u *UtilityCryptohomeBinary) InstallAttributesFinalize(ctx context.Context) error {
	out, err := u.binary.InstallAttributesFinalize(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to finalize Install Attributes with the following output %q", out)
	}
	if !strings.Contains(out, installAttributesFinalizeSuccessOutput) {
		return errors.Errorf("failed to finalize Install Attributes, incorrect output message %q", out)
	}
	return nil
}

// InstallAttributesCount retrieves the number of entries in install attributes. It returns count and error. error is nil iff the operation completes successfully, and in this case count holds the number of entries in install attributes.
func (u *UtilityCryptohomeBinary) InstallAttributesCount(ctx context.Context) (int, error) {
	out, err := u.binary.InstallAttributesCount(ctx)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to query install attributes count with the following output %q", out)
	}
	var result int
	n, err := fmt.Sscanf(out, "InstallAttributesCount(): %d", &result)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to parse InstallAttributesCount output %q", out)
	}
	if n != 1 {
		return -1, errors.Errorf("invalid InstallAttributesCount output %q", out)
	}
	return result, nil
}

// installAttributesBooleanHelper is a helper function that helps to parse the output of install attribute series of command that returns a boolean.
func installAttributesBooleanHelper(out string, err error, methodName string) (bool, error) {
	if err != nil {
		return false, errors.Wrapf(err, "failed to run %s(), output %q", methodName, out)
	}
	var result int
	n, err := fmt.Sscanf(out, methodName+"(): %d", &result)
	if err != nil {
		return false, errors.Wrapf(err, "failed to parse %s(), output %q", methodName, out)
	}
	if n != 1 {
		return false, errors.Errorf("invalid %s() output %q", methodName, out)
	}
	return result != 0, nil
}

// InstallAttributesIsReady checks if install attributes is ready, returns isReady and error. error is nil iff the operation completes successfully, and in this case isReady is whether install attributes is ready.
func (u *UtilityCryptohomeBinary) InstallAttributesIsReady(ctx context.Context) (bool, error) {
	out, err := u.binary.InstallAttributesIsReady(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsReady")
}

// InstallAttributesIsSecure checks if install attributes is secure, returns isSecure and error. error is nil iff the operation completes successfully, and in this case isSecure is whether install attributes is secure.
func (u *UtilityCryptohomeBinary) InstallAttributesIsSecure(ctx context.Context) (bool, error) {
	out, err := u.binary.InstallAttributesIsSecure(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsSecure")
}

// InstallAttributesIsInvalid checks if install attributes is invalid, returns isInvalid and error. error is nil iff the operation completes successfully, and in this case isInvalid is whether install attributes is invalid.
func (u *UtilityCryptohomeBinary) InstallAttributesIsInvalid(ctx context.Context) (bool, error) {
	out, err := u.binary.InstallAttributesIsInvalid(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsInvalid")
}

// InstallAttributesIsFirstInstall checks if install attributes is the first install state, returns isFirstInstall and error. error is nil iff the operation completes successfully, and in this case isFirstInstall is whether install attributes is in the first install state.
func (u *UtilityCryptohomeBinary) InstallAttributesIsFirstInstall(ctx context.Context) (bool, error) {
	out, err := u.binary.InstallAttributesIsFirstInstall(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsFirstInstall")
}

// IsMounted checks if any vault is mounted.
func (u *UtilityCryptohomeBinary) IsMounted(ctx context.Context) (bool, error) {
	out, err := u.binary.IsMounted(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to check if mounted")
	}
	result, err := strconv.ParseBool(strings.TrimSuffix(string(out), "\n"))
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

// UnmountAll unmounts all vault.
func (u *UtilityCryptohomeBinary) UnmountAll(ctx context.Context) error {
	if _, err := u.binary.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	return nil
}

// MountVault mounts the vault for username; creates a new vault if no vault yet if create is true. error is nil if the operation completed successfully.
func (u *UtilityCryptohomeBinary) MountVault(ctx context.Context, username string, password string, label string, create bool) error {
	if _, err := u.binary.MountEx(ctx, username, password, create, label); err != nil {
		return errors.Wrap(err, "failed to mount")
	}
	return nil
}

// CheckVault checks the vault via |CheckKeyEx| dbus mehod.
func (u *UtilityCryptohomeBinary) CheckVault(ctx context.Context, username string, password string, label string) (bool, error) {
	_, err := u.binary.CheckKeyEx(ctx, username, password, label)
	if err != nil {
		return false, errors.Wrap(err, "failed to check key")
	}
	return true, nil
}

// ListVaultKeys queries the vault associated with user username and password password, and returns nil for error iff the operation is completed successfully, in that case, the returned slice of string contains the labels of keys belonging to that vault.
func (u *UtilityCryptohomeBinary) ListVaultKeys(ctx context.Context, username string) ([]string, error) {
	binaryOutput, err := u.binary.ListKeysEx(ctx, username)
	if err != nil {
		return []string{}, errors.Wrap(err, "failed to call list keys")
	}

	output := string(binaryOutput)
	lines := strings.Split(output, "\n")
	var result []string
	for _, s := range lines {
		if strings.HasPrefix(s, listKeysExLabelPrefix) {
			result = append(result, s[len(listKeysExLabelPrefix):])
		}
	}
	return result, nil
}

// AddVaultKey adds the key with newLabel and newPassword to the user specified by username, with password password and label label. nil is returned iff the operation is successful.
func (u *UtilityCryptohomeBinary) AddVaultKey(ctx context.Context, username, password, label, newPassword, newLabel string, lowEntropy bool) error {
	binaryOutput, err := u.binary.AddKeyEx(ctx, username, password, label, newPassword, newLabel, lowEntropy)
	if err != nil {
		return errors.Wrap(err, "failed to call AddKeyEx")
	}

	output := strings.TrimSuffix(string(binaryOutput), "\n")
	if output != addKeyExSuccessMessage {
		testing.ContextLogf(ctx, "Incorrect AddKeyEx message; got %q, want %q", output, addKeyExSuccessMessage)
		return errors.Errorf("incorrect message from AddKeyEx; got %q, want %q", output, addKeyExSuccessMessage)
	}

	return nil
}

// RemoveVaultKey removes the key with label removeLabel from user specified by username's vault. password for username is supplied so the operation can be proceeded. nil is returned iff the operation is successful.
func (u *UtilityCryptohomeBinary) RemoveVaultKey(ctx context.Context, username, password, removeLabel string) error {
	binaryOutput, err := u.binary.RemoveKeyEx(ctx, username, password, removeLabel)
	if err != nil {
		return errors.Wrap(err, "failed to call RemoveKeyEx")
	}

	output := strings.TrimSuffix(string(binaryOutput), "\n")
	if output != removeKeyExSuccessMessage {
		testing.ContextLogf(ctx, "Incorrect RemoveKeyEx message; got %q, want %q", output, removeKeyExSuccessMessage)
		return errors.Errorf("incorrect message from RemoveKeyEx; got %q, want %q", output, removeKeyExSuccessMessage)
	}

	return nil
}

// ChangeVaultPassword changes the vault for user username with label and password to newPassword. nil is returned iff the operation is successful.
func (u *UtilityCryptohomeBinary) ChangeVaultPassword(ctx context.Context, username, password, label, newPassword string) error {
	binaryOutput, err := u.binary.MigrateKeyEx(ctx, username, password, label, newPassword)
	if err != nil {
		return errors.Wrap(err, "failed to call MigrateKeyEx")
	}

	output := strings.TrimSuffix(string(binaryOutput), "\n")
	if output != migrateKeyExSucessMessage {
		testing.ContextLogf(ctx, "Incorrect MigrateKeyEx message; got %q, want %q", output, migrateKeyExSucessMessage)
		return errors.Errorf("incorrect message from MigrateKeyEx; got %q, want %q", output, migrateKeyExSucessMessage)
	}

	return nil
}

// ChangeVaultLabel changes the vault label for the user username with label and password, to newLabel. nil is returned iff the operation is successful.
func (u *UtilityCryptohomeBinary) ChangeVaultLabel(ctx context.Context, username, password, label, newLabel string) error {
	binaryOutput, err := u.binary.UpdateKeyEx(ctx, username, password, label, newLabel)
	if err != nil {
		return errors.Wrap(err, "failed to call UpdateKeyEx")
	}

	output := strings.TrimSuffix(string(binaryOutput), "\n")
	if output != updateKeyExSuccessMessage {
		testing.ContextLogf(ctx, "Incorrect UpdateKeyEx message; got %q, want %q", output, updateKeyExSuccessMessage)
		return errors.Errorf("incorrect message from UpdateKeyEx; got %q, want %q", updateKeyExSuccessMessage)
	}

	return nil
}

// RemoveVault remove the vault for |username|.
func (u *UtilityCryptohomeBinary) RemoveVault(ctx context.Context, username string) (bool, error) {
	_, err := u.binary.Remove(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to remove vault")
	}
	return true, nil
}

// IsTPMWrappedKeySet checks if the current user vault is TPM-backed.
func (u *UtilityCryptohomeBinary) IsTPMWrappedKeySet(ctx context.Context, username string) (bool, error) {
	out, err := u.binary.DumpKeyset(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to dump keyset")
	}
	return strings.Contains(string(out), cryptohomeWrappedKeysetString), nil
}

// GetEnrollmentID gets the enrollment ID.
func (u *UtilityCryptohomeBinary) GetEnrollmentID(ctx context.Context) (string, error) {
	out, err := u.binary.GetEnrollmentID(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get EID")
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
	_, err := u.binary.TPMClearStoredPassword(ctx)
	return err
}

// GetKeyPayload gets the payload associated with the specified key.
func (u *UtilityCryptohomeBinary) GetKeyPayload(
	ctx context.Context,
	username string,
	label string) (string, error) {
	out, err := u.binary.TPMAttestationGetKeyPayload(ctx, username, label)
	if err != nil {
		return "", errors.Wrap(err, "failed to get key payload")
	}
	return string(out), nil
}

// SetKeyPayload sets the payload associated with the specified key.
func (u *UtilityCryptohomeBinary) SetKeyPayload(
	ctx context.Context,
	username string,
	label string,
	payload string) (bool, error) {
	_, err := u.binary.TPMAttestationSetKeyPayload(ctx, username, label, payload)
	if err != nil {
		return false, errors.Wrap(err, "failed to set key payload")
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
		return false, errors.Wrap(err, "failed to register key")
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
	_, err := u.binary.TPMAttestationDelete(ctx, username, prefix)
	if err != nil {
		return errors.Wrap(err, "failed to delete keys")
	}
	return nil
}

// parseTokenStatus parse the output of cryptohome --action=pkcs11_system_token_status or cryptohome --action=pkcs11_token_status and return the label, pin, slot and error (in that order).
func parseTokenStatus(cmdOutput string) (returnedLabel, returnedPin string, returnedSlot int, returnedErr error) {
	arr := strings.Split(cmdOutput, "\n")

	labels := []string{"Label", "Pin", "Slot"}
	params := make(map[string]string)
	for _, str := range arr {
		for _, label := range labels {
			labelPrefix := label + " = "
			if strings.HasPrefix(str, labelPrefix) {
				params[label] = str[len(labelPrefix):]
			}
		}
	}

	// Check that we've got all the parameters
	if len(params) != len(labels) {
		return "", "", -1, errors.Errorf("missing parameters in token status output, got: %v", params)
	}

	// Slot should be an integer
	slot, err := strconv.Atoi(params["Slot"])
	if err != nil {
		return "", "", -1, errors.Wrap(err, "token slot not integer")
	}

	// Fill up the return values
	return params["Label"], params["Pin"], slot, nil
}

// GetTokenForUser retrieve the token slot for the user token if |username| is non-empty, or system token if |username| is empty.
func (u *UtilityCryptohomeBinary) GetTokenForUser(ctx context.Context, username string) (int, error) {
	cmdOutput := ""
	if username == "" {
		// We want the system token.
		out, err := u.binary.Pkcs11SystemTokenInfo(ctx)
		cmdOutput = string(out)
		if err != nil {
			return -1, errors.Wrapf(err, "failed to get system token info %q", cmdOutput)
		}
	} else {
		// We want the user token.
		out, err := u.binary.Pkcs11UserTokenInfo(ctx, username)
		cmdOutput = string(out)
		if err != nil {
			return -1, errors.Wrapf(err, "failed to get user token info %q", cmdOutput)
		}
	}
	_, _, slot, err := parseTokenStatus(cmdOutput)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to parse token status %q", cmdOutput)
	}
	return slot, nil
}
