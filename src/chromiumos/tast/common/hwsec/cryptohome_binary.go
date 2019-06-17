// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"os"

	"chromiumos/tast/errors"
)

// CryptohomeBinary is used to interact with the cryptohomed process over
// 'cryptohome' executable. For more details of the arguments of the functions in this file,
// please check //src/platform2/cryptohome/cryptohome.cc.
// The arguments here are documented only when they are not directly
// mapped to the ones in so-mentioned cryptohome.cc.
type CryptohomeBinary struct {
	runner CmdRunner
}

func fromVATypeIntToString(VAType int) string {
	if VAType == 0 {
		return "default"
	}
	if VAType == 1 {
		return "test"
	}
	return "unknown"
}

// NewCryptohomeBinary is a factory function to create a
// CryptohomeBinary instance.
func NewCryptohomeBinary(r CmdRunner) (*CryptohomeBinary, error) {
	return &CryptohomeBinary{r}, nil
}

func (c *CryptohomeBinary) genAsyncModeFlag(async bool) string {
	if async {
		return "--async"
	}
	return ""
}

func (c *CryptohomeBinary) callCryptohome(ctx context.Context, args ...string) ([]byte, error) {
	return c.runner.Run(ctx, "cryptohome", args...)
}

// TPMStatus calls "cryptohome --action=tpm_status".
func (c *CryptohomeBinary) TPMStatus(ctx context.Context) (string, error) {
	out, err := c.callCryptohome(ctx, "--action=tpm_status")
	return string(out), err
}

// TPMAttestationStatus calls "cryptohome --action=tpm_attestation_status".
func (c *CryptohomeBinary) TPMAttestationStatus(ctx context.Context) (string, error) {
	out, err := c.callCryptohome(ctx, "--action=tpm_attestation_status")
	return string(out), err
}

// TPMTakeOwnership calls "cryptohome --action=tpm_take_ownership".
func (c *CryptohomeBinary) TPMTakeOwnership(ctx context.Context) error {
	_, err := c.callCryptohome(ctx, "--action=tpm_take_ownership")
	// We only care about the return code from cryptohome --action=tpm_take_ownership
	return err
}

// TPMWaitOwnership calls "cryptohome --action=tpm_wait_ownership".
func (c *CryptohomeBinary) TPMWaitOwnership(ctx context.Context) error {
	_, err := c.callCryptohome(ctx, "--action=tpm_wait_ownership")
	// We only care about the return code from cryptohome --action=tpm_wait_ownership
	return err
}

// TPMClearStoredPassword calls "cryptohome --action=tpm_clear_stored_password".
func (c *CryptohomeBinary) TPMClearStoredPassword(ctx context.Context) ([]byte, error) {
	return c.callCryptohome(ctx, "--action=tpm_clear_stored_password")
}

// TPMAttestationStartEnroll calls "cryptohome --action=enroll_request".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TPMAttestationStartEnroll(ctx context.Context, PCAType int, async bool) (string, error) {
	tmpFile, err := ioutil.TempFile("", "enroll_request")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file")
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	out, err := c.callCryptohome(
		ctx,
		"--action=tpm_attestation_start_enroll",
		"--output="+tmpFile.Name(),
		c.genAsyncModeFlag(async))
	if err != nil {
		return "", errors.Wrap(err, string(out))
	}
	out, err = ioutil.ReadFile(tmpFile.Name())
	if err != nil {
		return "", errors.Wrap(err, "failed to read enroll request from temp file")
	}
	return string(out), err
}

// TPMAttestationFinishEnroll calls "cryptohome --action=finish_enroll".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TPMAttestationFinishEnroll(ctx context.Context, PCAType int, resp string, async bool) (bool, error) {
	tmpFile, err := ioutil.TempFile("", "enroll_response")
	if err != nil {
		return false, errors.Wrap(err, "failed to create temp file")
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()
	tmpFile.Write([]byte(resp))
	out, err := c.callCryptohome(
		ctx,
		"--action=tpm_attestation_finish_enroll",
		"--input="+tmpFile.Name(),
		c.genAsyncModeFlag(async))
	if len(out) > 0 {
		return false, errors.New(string(out))
	}
	return true, nil
}

// TPMAttestationStartCertRequest calls "cryptohome --action=tpm_attestation_start_cert_request".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TPMAttestationStartCertRequest(
	ctx context.Context,
	PCAType int,
	profile int,
	username string,
	origin string,
	async bool) (string, error) {
	tmpFile, err := ioutil.TempFile("", "cert_request")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file")
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()
	out, err := c.callCryptohome(
		ctx,
		"--action=tpm_attestation_start_cert_request",
		"--output="+tmpFile.Name(),
		c.genAsyncModeFlag(async))
	if err != nil {
		return "", errors.Wrap(err, string(out))
	}
	out, err = ioutil.ReadFile(tmpFile.Name())
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
	tmpFileIn, err := ioutil.TempFile("", "cert_response")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file for cert response")
	}
	tmpFileOut, err := ioutil.TempFile("", "cert_result")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file for cert")
	}
	defer func() {
		tmpFileIn.Close()
		os.Remove(tmpFileIn.Name())
		tmpFileOut.Close()
		os.Remove(tmpFileOut.Name())
	}()
	tmpFileIn.Write([]byte(resp))
	out, err := c.callCryptohome(
		ctx,
		"--action=tpm_attestation_finish_cert_request",
		"--user="+username,
		"--name="+label,
		"--input="+tmpFileIn.Name(),
		"--output="+tmpFileOut.Name(),
		c.genAsyncModeFlag(async))
	if err != nil {
		return "", errors.Wrap(err, string(out))
	}
	out, err = ioutil.ReadFile(tmpFileOut.Name())
	if err != nil {
		return "", errors.Wrap(err, "failed to read cert result from temp file")
	}
	return string(out), err
}

// TPMAttestationEnterpriseVaChallenge calls "cryptohome --action=tpm_attestation_enterprise_challenge".
func (c *CryptohomeBinary) TPMAttestationEnterpriseVaChallenge(
	ctx context.Context,
	VAType int,
	username string,
	label string,
	domain string,
	deviceID string,
	challenge []byte) (string, error) {
	tmpFile, err := ioutil.TempFile("", "challenge")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file for cert response")
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()
	tmpFile.Write(challenge)
	VATypeString := fromVATypeIntToString(VAType)
	out, err := c.callCryptohome(
		ctx,
		"--action=tpm_attestation_enterprise_challenge",
		"--va-server="+VATypeString,
		"--user="+username,
		"--name="+label,
		"--input="+tmpFile.Name())
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
	out, err := c.callCryptohome(
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
	out, err := c.callCryptohome(
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
	out, err := c.callCryptohome(
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
	out, err := c.callCryptohome(
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
	out, err := c.callCryptohome(
		ctx,
		"--action=tpm_attestation_set_key_payload",
		"--user="+username,
		"--name="+label,
		"--value="+payload)
	return string(out), err
}

// IsMounted calls "cryptohome --action=is_mounted".
func (c *CryptohomeBinary) IsMounted(ctx context.Context) ([]byte, error) {
	return c.callCryptohome(ctx, "--action=is_mounted")
}

// MountEx calls "cryptohome --action=mount_ex".
func (c *CryptohomeBinary) MountEx(ctx context.Context, username string, password string, doesCreate bool) ([]byte, error) {
	createFlag := ""
	if doesCreate {
		createFlag = "--create"
	}
	return c.callCryptohome(ctx, "--action=mount_ex", "--user="+username, "--password="+password, createFlag, "--key_label=dontcare")
}

// CheckKeyEx calls "cryptohome --action=check_key_ex".
func (c *CryptohomeBinary) CheckKeyEx(ctx context.Context, username string, password string) ([]byte, error) {
	return c.callCryptohome(ctx, "--action=check_key_ex", "--user="+username, "--password="+password, "--key_label=dontcare")
}

// Remove calls "cryptohome --action=remove".
func (c *CryptohomeBinary) Remove(ctx context.Context, username string) ([]byte, error) {
	return c.callCryptohome(ctx, "--action=remove", "--user="+username, "--force")
}

// Unmount calls "cryptohome --action=unmount".
func (c *CryptohomeBinary) Unmount(ctx context.Context, username string) ([]byte, error) {
	return c.callCryptohome(ctx, "--action=unmount", "--user="+username)
}

// DumpKeyset calls "cryptohome --action=dump_keyset".
func (c *CryptohomeBinary) DumpKeyset(ctx context.Context, username string) ([]byte, error) {
	return c.callCryptohome(ctx, "--action=dump_keyset", "--user="+username)
}

// GetEnrollmentID calls "cryptohome --action=get_enrollment_id".
func (c *CryptohomeBinary) GetEnrollmentID(ctx context.Context) ([]byte, error) {
	return c.callCryptohome(ctx, "--action=get_enrollment_id")
}

// TPMAttestationDelete calls "cryptohome --action=tpm_attestation_delete".
func (c *CryptohomeBinary) TPMAttestationDelete(ctx context.Context, username string, prefix string) ([]byte, error) {
	return c.callCryptohome(ctx, "--action=tpm_attestation_delete", "--user="+username, "--name="+prefix)
}
