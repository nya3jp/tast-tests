// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"encoding/base64"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
)

// CryptohomeBinary is used to interact with the cryptohomed process over
// 'cryptohome' executable. For more details of the arguments of the functions in this file,
// please check //src/platform2/cryptohome/cryptohome.cc.
// The arguments here are documented only when they are not directly
// mapped to the ones in so-mentioned cryptohome.cc.
type CryptohomeBinary struct {
	runner CmdRunner
}

const asyncAttestationFlag = "--async"

func fromVATypeIntToString(vaType VAType) string {
	if vaType == DefaultVA {
		return "default"
	}
	if vaType == TestVA {
		return "test"
	}
	return "unknown"
}

// NewCryptohomeBinary is a factory function to create a
// CryptohomeBinary instance.
func NewCryptohomeBinary(r CmdRunner) (*CryptohomeBinary, error) {
	return &CryptohomeBinary{r}, nil
}

func (c *CryptohomeBinary) call(ctx context.Context, args ...string) ([]byte, error) {
	return c.runner.Run(ctx, "cryptohome", args...)
}

func (c *CryptohomeBinary) tempFile(ctx context.Context, prefix string) (string, error) {
	out, err := c.runner.Run(ctx, "mktemp", "/tmp/"+prefix+".XXXXX")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), err
}

func (c *CryptohomeBinary) readFile(ctx context.Context, filename string) ([]byte, error) {
	return c.runner.Run(ctx, "cat", "--", filename)
}

func (c *CryptohomeBinary) writeFile(ctx context.Context, filename string, data []byte) error {
	tmpFile, err := c.tempFile(ctx, "tast_cryptohome_write")
	if err != nil {
		return errors.Wrap(err, "failed to create temp file")
	}
	defer c.removeFile(ctx, tmpFile)
	b64String := base64.StdEncoding.EncodeToString(data)
	if _, err := c.runner.Run(ctx, "sh", "-c", "echo "+shutil.Escape(b64String)+">"+tmpFile); err != nil {
		return errors.Wrap(err, "failed to echo string")
	}
	_, err = c.runner.Run(ctx, "sh", "-c", "base64 -d "+tmpFile+">"+filename)
	return err
}

func (c *CryptohomeBinary) removeFile(ctx context.Context, filename string) error {
	_, err := c.runner.Run(ctx, "rm", "-f", "--", filename)
	return err
}

// TPMStatus calls "cryptohome --action=tpm_status".
func (c *CryptohomeBinary) TPMStatus(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=tpm_status")
	return string(out), err
}

// TPMAttestationStatus calls "cryptohome --action=tpm_attestation_status".
func (c *CryptohomeBinary) TPMAttestationStatus(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=tpm_attestation_status")
	return string(out), err
}

// GetStatusString calls "cryptohome --action=status".
func (c *CryptohomeBinary) GetStatusString(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=status")
	return string(out), err
}

// TPMTakeOwnership calls "cryptohome --action=tpm_take_ownership".
func (c *CryptohomeBinary) TPMTakeOwnership(ctx context.Context) error {
	_, err := c.call(ctx, "--action=tpm_take_ownership")
	// We only care about the return code from cryptohome --action=tpm_take_ownership
	return err
}

// TPMWaitOwnership calls "cryptohome --action=tpm_wait_ownership".
func (c *CryptohomeBinary) TPMWaitOwnership(ctx context.Context) error {
	_, err := c.call(ctx, "--action=tpm_wait_ownership")
	// We only care about the return code from cryptohome --action=tpm_wait_ownership
	return err
}

// TPMClearStoredPassword calls "cryptohome --action=tpm_clear_stored_password".
func (c *CryptohomeBinary) TPMClearStoredPassword(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=tpm_clear_stored_password")
}

// TPMAttestationStartEnroll calls "cryptohome --action=enroll_request".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TPMAttestationStartEnroll(ctx context.Context, pcaType PCAType, async bool) (string, error) {
	if pcaType == TestPCA {
		return "", errors.New("test PCA doesn't support automated test")
	}
	tmpFile, err := c.tempFile(ctx, "enroll_request")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file")
	}
	defer c.removeFile(ctx, tmpFile)

	args := []string{
		"--action=tpm_attestation_start_enroll",
		"--output=" + tmpFile}
	if async {
		args = append(args, asyncAttestationFlag)
	}
	out, err := c.call(ctx, args...)
	if err != nil {
		return "", errors.Wrap(err, "error calling cryptohome binary")
	}
	out, err = c.readFile(ctx, tmpFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to read enroll request from temp file")
	}
	return string(out), err
}

// TPMAttestationFinishEnroll calls "cryptohome --action=finish_enroll".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TPMAttestationFinishEnroll(ctx context.Context, pcaType PCAType, resp string, async bool) (bool, error) {
	if pcaType == TestPCA {
		return false, errors.New("test PCA doesn't support automated test")
	}
	tmpFile, err := c.tempFile(ctx, "enroll_response")
	if err != nil {
		return false, errors.Wrap(err, "failed to create temp file")
	}
	defer c.removeFile(ctx, tmpFile)

	if err := c.writeFile(ctx, tmpFile, []byte(resp)); err != nil {
		return false, errors.Wrap(err, "failed to write response to temp file")
	}

	args := []string{
		"--action=tpm_attestation_finish_enroll",
		"--input=" + tmpFile}
	if async {
		args = append(args, asyncAttestationFlag)
	}
	out, err := c.call(
		ctx, args...)
	if len(out) > 0 {
		return false, errors.New(string(out))
	}
	return true, nil
}

// TPMAttestationStartCertRequest calls "cryptohome --action=tpm_attestation_start_cert_request".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TPMAttestationStartCertRequest(
	ctx context.Context,
	pcaType PCAType,
	profile int,
	username string,
	origin string,
	async bool) (string, error) {
	if pcaType == TestPCA {
		return "", errors.New("test PCA doesn't support automated test")
	}
	tmpFile, err := c.tempFile(ctx, "cert_request")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file")
	}
	defer c.removeFile(ctx, tmpFile)

	args := []string{
		"--action=tpm_attestation_start_cert_request",
		"--output=" + tmpFile}
	if async {
		args = append(args, asyncAttestationFlag)
	}
	out, err := c.call(ctx, args...)
	if err != nil {
		return "", errors.Wrap(err, string(out))
	}
	out, err = c.readFile(ctx, tmpFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to read cert request from temp file")
	}
	return string(out), err
}

// TPMAttestationFinishCertRequest calls "cryptohome --action=tpm_attestation_finish_cert_request".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TPMAttestationFinishCertRequest(
	ctx context.Context,
	resp string,
	username string,
	label string,
	async bool) (string, error) {
	tmpFileIn, err := c.tempFile(ctx, "cert_response")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file for cert response")
	}
	defer c.removeFile(ctx, tmpFileIn)
	tmpFileOut, err := c.tempFile(ctx, "cert_result")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file for cert")
	}
	defer c.removeFile(ctx, tmpFileOut)

	if err := c.writeFile(ctx, tmpFileIn, []byte(resp)); err != nil {
		return "", errors.Wrap(err, "failed to write cert response to temp file")
	}

	args := []string{
		"--action=tpm_attestation_finish_cert_request",
		"--user=" + username,
		"--name=" + label,
		"--input=" + tmpFileIn,
		"--output=" + tmpFileOut}
	if async {
		args = append(args, asyncAttestationFlag)
	}
	out, err := c.call(ctx, args...)
	if err != nil {
		return "", errors.Wrap(err, string(out))
	}
	out, err = c.readFile(ctx, tmpFileOut)
	if err != nil {
		return "", errors.Wrap(err, "failed to read cert result from temp file")
	}
	return string(out), err
}

// TPMAttestationEnterpriseVaChallenge calls "cryptohome --action=tpm_attestation_enterprise_challenge".
func (c *CryptohomeBinary) TPMAttestationEnterpriseVaChallenge(
	ctx context.Context,
	vaType VAType,
	username string,
	label string,
	domain string,
	deviceID string,
	challenge []byte) (string, error) {
	tmpFile, err := c.tempFile(ctx, "challenge")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file for cert response")
	}
	defer c.removeFile(ctx, tmpFile)
	if err := c.writeFile(ctx, tmpFile, challenge); err != nil {
		return "", errors.Wrap(err, "failed to write challenge to temp file")
	}
	vaTypeString := fromVATypeIntToString(vaType)
	out, err := c.call(
		ctx,
		"--action=tpm_attestation_enterprise_challenge",
		"--va-server="+vaTypeString,
		"--user="+username,
		"--name="+label,
		"--input="+tmpFile)
	return string(out), err
}

// TPMAttestationSimpleChallenge calls "cryptohome --action=tpm_attestation_simple_challenge".
func (c *CryptohomeBinary) TPMAttestationSimpleChallenge(
	ctx context.Context,
	username string,
	label string,
	challenge []byte) (string, error) {
	if len(challenge) > 0 {
		return "", errors.New("currently arbitrary challenge is not supported and requires to be empty")
	}
	out, err := c.call(
		ctx,
		"--action=tpm_attestation_simple_challenge",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// TPMAttestationKeyStatus calls "cryptohome --action=tpm_attestation_key_status".
func (c *CryptohomeBinary) TPMAttestationKeyStatus(
	ctx context.Context,
	username string,
	label string) (string, error) {
	out, err := c.call(
		ctx,
		"--action=tpm_attestation_key_status",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// TPMAttestationGetKeyPayload calls "cryptohome --action=tpm_attestation_get_key_payload".
func (c *CryptohomeBinary) TPMAttestationGetKeyPayload(
	ctx context.Context,
	username string,
	label string) (string, error) {
	out, err := c.call(
		ctx,
		"--action=tpm_attestation_get_key_payload",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// TPMAttestationRegisterKey calls "cryptohome --action=tpm_attestation_register_key".
func (c *CryptohomeBinary) TPMAttestationRegisterKey(
	ctx context.Context,
	username string,
	label string) (string, error) {
	out, err := c.call(
		ctx,
		"--action=tpm_attestation_register_key",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// TPMAttestationSetKeyPayload calls "cryptohome --action=tpm_attestation_set_key_payload".
func (c *CryptohomeBinary) TPMAttestationSetKeyPayload(
	ctx context.Context,
	username string,
	label string,
	payload string) (string, error) {
	out, err := c.call(
		ctx,
		"--action=tpm_attestation_set_key_payload",
		"--user="+username,
		"--name="+label,
		"--value="+payload)
	return string(out), err
}

// IsMounted calls "cryptohome --action=is_mounted".
func (c *CryptohomeBinary) IsMounted(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=is_mounted")
}

// MountEx calls "cryptohome --action=mount_ex".
func (c *CryptohomeBinary) MountEx(ctx context.Context, username string, password string, doesCreate bool, label string) ([]byte, error) {
	createFlag := ""
	if doesCreate {
		createFlag = "--create"
	}
	return c.call(ctx, "--action=mount_ex", "--user="+username, "--password="+password, createFlag, "--key_label="+label)
}

// CheckKeyEx calls "cryptohome --action=check_key_ex".
func (c *CryptohomeBinary) CheckKeyEx(ctx context.Context, username string, password string, label string) ([]byte, error) {
	return c.call(ctx, "--action=check_key_ex", "--user="+username, "--password="+password, "--key_label="+label)
}

// Remove calls "cryptohome --action=remove".
func (c *CryptohomeBinary) Remove(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=remove", "--user="+username, "--force")
}

// Unmount calls "cryptohome --action=unmount".
func (c *CryptohomeBinary) Unmount(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=unmount", "--user="+username)
}

// DumpKeyset calls "cryptohome --action=dump_keyset".
func (c *CryptohomeBinary) DumpKeyset(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=dump_keyset", "--user="+username)
}

// GetEnrollmentID calls "cryptohome --action=get_enrollment_id".
func (c *CryptohomeBinary) GetEnrollmentID(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=get_enrollment_id")
}

// TPMAttestationDelete calls "cryptohome --action=tpm_attestation_delete".
func (c *CryptohomeBinary) TPMAttestationDelete(ctx context.Context, username string, prefix string) ([]byte, error) {
	return c.call(ctx, "--action=tpm_attestation_delete", "--user="+username, "--name="+prefix)
}
