// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// CryptohomeBinary is used to interact with the cryptohomed process over
// 'cryptohome' execuable. For more details of the arguments of the functions in this file,
// please check //src/platform2/cryptohome/cryptohome.cc.
// The arguments here are documented only when they are not directly
// mapped to the ones in so-mentioned cryptohome.cc.
type CryptohomeBinary struct {
	ctx            context.Context
	state          *testing.State
	doesCallRemote bool
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
func NewCryptohomeBinary(ctx context.Context, s *testing.State) (*CryptohomeBinary, error) {
	out, err := testexec.CommandContext(ctx, "which", "cryptohome").Output()
	return &CryptohomeBinary{ctx, s, err != nil || len(out) == 0}, nil
}

func (c *CryptohomeBinary) genAsyncModeFlag(async bool) string {
	if async {
		return "--async"
	}
	return ""
}

func (c *CryptohomeBinary) call(cmd string, args ...string) ([]byte, error) {
	if c.doesCallRemote {
		d := c.state.DUT()
		cmd := cmd + " " + strings.Join([]string(args), " ")
		testing.ContextLog(c.ctx, "Running "+cmd)
		return d.Command("sh", "-c", cmd).CombinedOutput(c.ctx)
	}
	cmdToRun := cmd + " " + strings.Join([]string(args), " ")
	testing.ContextLog(c.ctx, "Running "+cmdToRun)
	return testexec.CommandContext(c.ctx, cmd, args...).Output()
}

func (c *CryptohomeBinary) callCryptohome(args ...string) ([]byte, error) {
	return c.call("cryptohome", args...)
}

// TpmStatus calls "cryptohome --action=tpm_status".
func (c *CryptohomeBinary) TpmStatus() (string, error) {
	out, err := c.callCryptohome("--action=tpm_status")
	return string(out), err
}

// TpmAttestationStatus calls "cryptohome --action=tpm_attestation_status".
func (c *CryptohomeBinary) TpmAttestationStatus() (string, error) {
	out, err := c.callCryptohome("--action=tpm_attestation_status")
	return string(out), err
}

// TpmTakeOwnership calls "cryptohome --action=tpm_take_ownership".
func (c *CryptohomeBinary) TpmTakeOwnership() error {
	_, err := c.callCryptohome("--action=tpm_take_ownership")
	// We only care about the return code from cryptohome --action=tpm_take_ownership
	return err
}

// TpmWaitOwnership calls "cryptohome --action=tpm_wait_ownership".
func (c *CryptohomeBinary) TpmWaitOwnership() error {
	_, err := c.callCryptohome("--action=tpm_wait_ownership")
	// We only care about the return code from cryptohome --action=tpm_wait_ownership
	return err
}

// TpmAttestationStartEnroll calls "cryptohome --action=enroll_request".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TpmAttestationStartEnroll(PCAType int, async bool) (string, error) {
	tmpFile, err := ioutil.TempFile("", "enroll_request")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file")
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	out, err := c.callCryptohome(
		"--action=tpm_attestation_start_enroll",
		"--output="+tmpFile.Name(),
		c.genAsyncModeFlag(async))
	if len(out) > 0 {
		return "", errors.New(string(out))
	}
	out, err = ioutil.ReadFile(tmpFile.Name())
	if err != nil {
		return "", errors.Wrap(err, "failed to read enroll request from temp file")
	}
	return string(out), err
}

// TpmAttestationFinishEnroll calls "cryptohome --action=finish_enroll".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TpmAttestationFinishEnroll(PCAType int, resp string, async bool) (bool, error) {
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
		"--action=tpm_attestation_finish_enroll",
		"--input="+tmpFile.Name(),
		c.genAsyncModeFlag(async))
	if len(out) > 0 {
		return false, errors.New(string(out))
	}
	return true, nil
}

// TpmAttestationStartCertRequest calls "cryptohome --action=tpm_attestation_start_cert_request".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TpmAttestationStartCertRequest(
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
		"--action=tpm_attestation_start_cert_request",
		"--output="+tmpFile.Name(),
		c.genAsyncModeFlag(async))
	if len(out) > 0 {
		return "", errors.New(string(out))
	}
	out, err = ioutil.ReadFile(tmpFile.Name())
	if err != nil {
		return "", errors.Wrap(err, "failed to read cert request from temp file")
	}
	return string(out), err
}

// TpmAttestationFinishCertRequest calls "cryptohome --action=tpm_attestation_finish_cert_request".
// If |async| is set, calls it long with "--async" flag.
func (c *CryptohomeBinary) TpmAttestationFinishCertRequest(
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
		"--action=tpm_attestation_finish_cert_request",
		"--user="+username,
		"--name="+label,
		"--input="+tmpFileIn.Name(),
		"--output="+tmpFileOut.Name(),
		c.genAsyncModeFlag(async))
	out, err = ioutil.ReadFile(tmpFileOut.Name())
	return string(out), err
}

// TpmAttestationEnterpriseVaChallenge calls "cryptohome --action=tpm_attestation_enterprise_challenge".
func (c *CryptohomeBinary) TpmAttestationEnterpriseVaChallenge(
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
		"--action=tpm_attestation_enterprise_challenge",
		"--va-server="+VATypeString,
		"--user="+username,
		"--name="+label,
		"--input="+tmpFile.Name())
	return string(out), err
}

// TpmAttestationSimpleChallenge calls "cryptohome --action=tpm_attestation_simple_challenge".
func (c *CryptohomeBinary) TpmAttestationSimpleChallenge(
	username string,
	label string,
	challenge []byte) (string, error) {
	if len(challenge) > 0 {
		return "", errors.New("currently arbitrary challenge is not supported and requires to be empty")
	}
	out, err := c.callCryptohome(
		"--action=tpm_attestation_simple_challenge",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// TpmAttestationKeyStatus calls "cryptohome --action=tpm_attestation_key_status".
func (c *CryptohomeBinary) TpmAttestationKeyStatus(
	username string,
	label string) (string, error) {
	out, err := c.callCryptohome(
		"--action=tpm_attestation_key_status",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// TpmAttestationGetKeyPayload calls "cryptohome --action=tpm_attestation_get_key_payload".
func (c *CryptohomeBinary) TpmAttestationGetKeyPayload(
	username string,
	label string) (string, error) {
	out, err := c.callCryptohome(
		"--action=tpm_attestation_get_key_payload",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// TpmAttestationRegisterKey calls "cryptohome --action=tpm_attestation_register_key".
func (c *CryptohomeBinary) TpmAttestationRegisterKey(
	username string,
	label string) (string, error) {
	out, err := c.callCryptohome(
		"--action=tpm_attestation_register_key",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// TpmAttestationSetKeyPayload calls "cryptohome --action=tpm_attestation_set_key_payload".
func (c *CryptohomeBinary) TpmAttestationSetKeyPayload(
	username string,
	label string,
	payload string) (string, error) {
	out, err := c.callCryptohome(
		"--action=tpm_attestation_set_key_payload",
		"--user="+username,
		"--name="+label,
		"--value="+payload)
	return string(out), err
}

// IsMounted calls "cryptohome --action=is_mounted".
func (c *CryptohomeBinary) IsMounted() ([]byte, error) {
	return c.callCryptohome("--action=is_mounted")
}

// MountEx calls "cryptohome --action=mount_ex".
func (c *CryptohomeBinary) MountEx(username string, password string, doesCreate bool) ([]byte, error) {
	createFlag := ""
	if doesCreate {
		createFlag = "--create"
	}
	return c.callCryptohome("--action=mount_ex", "--user="+username, "--password="+password, createFlag, "--key_label=dontcare")
}

// Remove calls "cryptohome --action=remove".
func (c *CryptohomeBinary) Remove(username string) ([]byte, error) {
	return c.callCryptohome("--action=remove", "--user="+username, "--force")
}

// Unmount calls "cryptohome --action=unmount".
func (c *CryptohomeBinary) Unmount(username string) ([]byte, error) {
	return c.callCryptohome("--action=unmount", "--user="+username)
}

// DumpKeyset calls "cryptohome --action=dump_keyset".
func (c *CryptohomeBinary) DumpKeyset(username string) ([]byte, error) {
	return c.callCryptohome("--action=dump_keyset", "--user="+username)
}

// GetEnrollmentID calls "cryptohome --action=get_enrollment_id".
func (c *CryptohomeBinary) GetEnrollmentID() ([]byte, error) {
	return c.callCryptohome("--action=get_enrollment_id")
}

// TpmAttestationDelete calls "cryptohome --action=tpm_attestation_delete".
func (c *CryptohomeBinary) TpmAttestationDelete(username string, prefix string) ([]byte, error) {
	return c.callCryptohome("--action=tpm_attestation_delete", "--user="+username, "--name="+prefix)
}

// Pkcs11SystemTokenStatus calls "cryptohome --action=pkcs11_system_token_status".
func (c *CryptohomeBinary) Pkcs11SystemTokenStatus() (string, error) {
	out, err := c.callCryptohome("--action=pkcs11_system_token_status")
	return string(out), err
}

// Pkcs11TokenStatus calls "cryptohome --action=pkcs11_token_status". (and gets the user token status)
func (c *CryptohomeBinary) Pkcs11TokenStatus(username string) (string, error) {
	out, err := c.callCryptohome("--action=pkcs11_token_status", "--user="+username)
	return string(out), err
}
