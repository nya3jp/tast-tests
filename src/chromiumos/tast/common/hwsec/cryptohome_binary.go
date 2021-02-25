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

// cryptohomeBinary is used to interact with the cryptohomed process over
// 'cryptohome' executable. For more details of the arguments of the functions in this file,
// please check //src/platform2/cryptohome/cryptohome.cc.
// The arguments here are documented only when they are not directly
// mapped to the ones in so-mentioned cryptohome.cc.
type cryptohomeBinary struct {
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
// cryptohomeBinary instance.
func NewCryptohomeBinary(r CmdRunner) (*cryptohomeBinary, error) {
	return &cryptohomeBinary{r}, nil
}

func (c *cryptohomeBinary) call(ctx context.Context, args ...string) ([]byte, error) {
	return c.runner.Run(ctx, "cryptohome", args...)
}

func (c *cryptohomeBinary) tempFile(ctx context.Context, prefix string) (string, error) {
	out, err := c.runner.Run(ctx, "mktemp", "/tmp/"+prefix+".XXXXX")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), err
}

func (c *cryptohomeBinary) readFile(ctx context.Context, filename string) ([]byte, error) {
	return c.runner.Run(ctx, "cat", "--", filename)
}

func (c *cryptohomeBinary) writeFile(ctx context.Context, filename string, data []byte) error {
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

func (c *cryptohomeBinary) removeFile(ctx context.Context, filename string) error {
	_, err := c.runner.Run(ctx, "rm", "-f", "--", filename)
	return err
}

// tpmStatus calls "cryptohome --action=tpm_status".
func (c *cryptohomeBinary) tpmStatus(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=tpm_status")
	return string(out), err
}

// tpmMoreStatus calls "cryptohome --action=tpm_more_status".
func (c *cryptohomeBinary) tpmMoreStatus(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=tpm_more_status")
	return string(out), err
}

// tpmAttestationStatus calls "cryptohome --action=tpm_attestation_status".
func (c *cryptohomeBinary) tpmAttestationStatus(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=tpm_attestation_status")
	return string(out), err
}

// getStatusString calls "cryptohome --action=status".
func (c *cryptohomeBinary) getStatusString(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=status")
	return string(out), err
}

// tpmTakeOwnership calls "cryptohome --action=tpm_take_ownership".
func (c *cryptohomeBinary) tpmTakeOwnership(ctx context.Context) error {
	_, err := c.call(ctx, "--action=tpm_take_ownership")
	// We only care about the return code from cryptohome --action=tpm_take_ownership
	return err
}

// tpmWaitOwnership calls "cryptohome --action=tpm_wait_ownership".
func (c *cryptohomeBinary) tpmWaitOwnership(ctx context.Context) error {
	_, err := c.call(ctx, "--action=tpm_wait_ownership")
	// We only care about the return code from cryptohome --action=tpm_wait_ownership
	return err
}

// tpmClearStoredPassword calls "cryptohome --action=tpm_clear_stored_password".
func (c *cryptohomeBinary) tpmClearStoredPassword(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=tpm_clear_stored_password")
}

// tpmAttestationStartEnroll calls "cryptohome --action=enroll_request".
// If async is set, calls it long with "--async" flag.
func (c *cryptohomeBinary) tpmAttestationStartEnroll(ctx context.Context, pcaType PCAType, async bool) (string, error) {
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

// tpmAttestationFinishEnroll calls "cryptohome --action=finish_enroll".
// If async is set, calls it long with "--async" flag.
func (c *cryptohomeBinary) tpmAttestationFinishEnroll(ctx context.Context, pcaType PCAType, resp string, async bool) (bool, error) {
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

// tpmAttestationStartCertRequest calls "cryptohome --action=tpm_attestation_start_cert_request".
// If |async| is set, calls it long with "--async" flag.
func (c *cryptohomeBinary) tpmAttestationStartCertRequest(
	ctx context.Context,
	pcaType PCAType,
	profile int,
	username,
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

// tpmAttestationFinishCertRequest calls "cryptohome --action=tpm_attestation_finish_cert_request".
// If |async| is set, calls it long with "--async" flag.
func (c *cryptohomeBinary) tpmAttestationFinishCertRequest(
	ctx context.Context,
	resp,
	username,
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

// tpmAttestationEnterpriseVaChallenge calls "cryptohome --action=tpm_attestation_enterprise_challenge".
func (c *cryptohomeBinary) tpmAttestationEnterpriseVaChallenge(
	ctx context.Context,
	vaType VAType,
	username,
	label,
	domain,
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

// tpmAttestationSimpleChallenge calls "cryptohome --action=tpm_attestation_simple_challenge".
func (c *cryptohomeBinary) tpmAttestationSimpleChallenge(
	ctx context.Context,
	username,
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

// tpmAttestationKeyStatus calls "cryptohome --action=tpm_attestation_key_status".
func (c *cryptohomeBinary) tpmAttestationKeyStatus(
	ctx context.Context,
	username,
	label string) (string, error) {
	out, err := c.call(
		ctx,
		"--action=tpm_attestation_key_status",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// tpmAttestationGetKeyPayload calls "cryptohome --action=tpm_attestation_get_key_payload".
func (c *cryptohomeBinary) tpmAttestationGetKeyPayload(
	ctx context.Context,
	username,
	label string) (string, error) {
	out, err := c.call(
		ctx,
		"--action=tpm_attestation_get_key_payload",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// tpmAttestationRegisterKey calls "cryptohome --action=tpm_attestation_register_key".
func (c *cryptohomeBinary) tpmAttestationRegisterKey(
	ctx context.Context,
	username,
	label string) (string, error) {
	out, err := c.call(
		ctx,
		"--action=tpm_attestation_register_key",
		"--user="+username,
		"--name="+label)
	return string(out), err
}

// tpmAttestationSetKeyPayload calls "cryptohome --action=tpm_attestation_set_key_payload".
func (c *cryptohomeBinary) tpmAttestationSetKeyPayload(
	ctx context.Context,
	username,
	label,
	payload string) (string, error) {
	out, err := c.call(
		ctx,
		"--action=tpm_attestation_set_key_payload",
		"--user="+username,
		"--name="+label,
		"--value="+payload)
	return string(out), err
}

// installAttributesGet calls "cryptohome --action=install_attributes_get".
func (c *cryptohomeBinary) installAttributesGet(ctx context.Context, attributeName string) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_get", "--name="+attributeName)
	return string(out), err
}

// installAttributesSet calls "cryptohome --action=install_attributes_set".
func (c *cryptohomeBinary) installAttributesSet(ctx context.Context, attributeName, attributeValue string) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_set", "--name="+attributeName, "--value="+attributeValue)
	return string(out), err
}

// installAttributesFinalize calls "cryptohome --action=install_attributes_finalize".
func (c *cryptohomeBinary) installAttributesFinalize(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_finalize")
	return string(out), err
}

// installAttributesCount calls "cryptohome --action=install_attributes_count".
func (c *cryptohomeBinary) installAttributesCount(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_count")
	return string(out), err
}

// installAttributesIsReady calls "cryptohome --action=install_attributes_is_ready".
func (c *cryptohomeBinary) installAttributesIsReady(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_is_ready")
	return string(out), err
}

// installAttributesIsSecure calls "cryptohome --action=install_attributes_is_secure".
func (c *cryptohomeBinary) installAttributesIsSecure(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_is_secure")
	return string(out), err
}

// installAttributesIsInvalid calls "cryptohome --action=install_attributes_is_invalid".
func (c *cryptohomeBinary) installAttributesIsInvalid(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_is_invalid")
	return string(out), err
}

// installAttributesIsFirstInstall calls "cryptohome --action=install_attributes_is_first_install".
func (c *cryptohomeBinary) installAttributesIsFirstInstall(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_is_first_install")
	return string(out), err
}

// isMounted calls "cryptohome --action=is_mounted".
func (c *cryptohomeBinary) isMounted(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=is_mounted")
}

// mountEx calls "cryptohome --action=mount_ex".
func (c *cryptohomeBinary) mountEx(ctx context.Context, username, password string, doesCreate bool, label string, extraFlags []string) ([]byte, error) {
	args := []string{"--action=mount_ex", "--user=" + username, "--password=" + password, "--key_label=" + label}
	if doesCreate {
		args = append(args, "--create")
	}
	args = append(args, extraFlags...)
	return c.call(ctx, args...)
}

// getSanitizedUsername calls "cryptohome --action=obfuscate_user".
func (c *cryptohomeBinary) getSanitizedUsername(ctx context.Context, username string, useDBus bool) ([]byte, error) {
	args := []string{"--action=obfuscate_user", "--user=" + username}
	if useDBus {
		args = append(args, "--use_dbus")
	}
	return c.call(ctx, args...)
}

// getSystemSalt calls "cryptohome --action=get_system_salt".
func (c *cryptohomeBinary) getSystemSalt(ctx context.Context, useDBus bool) ([]byte, error) {
	args := []string{"--action=get_system_salt"}
	if useDBus {
		args = append(args, "--use_dbus")
	}
	return c.call(ctx, args...)
}

// checkKeyEx calls "cryptohome --action=check_key_ex".
func (c *cryptohomeBinary) checkKeyEx(ctx context.Context, username, password, label string) ([]byte, error) {
	return c.call(ctx, "--action=check_key_ex", "--user="+username, "--password="+password, "--key_label="+label)
}

// listKeysEx calls "cryptohome --action=list_keys_ex".
func (c *cryptohomeBinary) listKeysEx(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=list_keys_ex", "--user="+username)
}

// addKeyEx calls "cryptohome --action=add_key_ex".
func (c *cryptohomeBinary) addKeyEx(ctx context.Context, username, password, label, newPassword, newLabel string, lowEntropy bool) ([]byte, error) {
	args := []string{"--action=add_key_ex", "--user=" + username, "--password=" + password, "--key_label=" + label, "--new_password=" + newPassword, "--new_key_label=" + newLabel}
	if lowEntropy {
		args = append(args, "--key_policy=le")
	}
	return c.call(ctx, args...)
}

// removeKeyEx calls "cryptohome --action=remove_key_ex".
func (c *cryptohomeBinary) removeKeyEx(ctx context.Context, username, password, removeLabel string) ([]byte, error) {
	return c.call(ctx, "--action=remove_key_ex", "--user="+username, "--password="+password, "--remove_key_label="+removeLabel)
}

// migrateKeyEx calls "cryptohome --action=migrate_key_ex".
func (c *cryptohomeBinary) migrateKeyEx(ctx context.Context, username, password, label, newPassword string) ([]byte, error) {
	return c.call(ctx, "--action=migrate_key_ex", "--user="+username, "--old_password="+password, "--key_label="+label, "--password="+newPassword)
}

// remove calls "cryptohome --action=remove".
func (c *cryptohomeBinary) remove(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=remove", "--user="+username, "--force")
}

// unmount calls "cryptohome --action=unmount".
func (c *cryptohomeBinary) unmount(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=unmount", "--user="+username)
}

// unmountAll calls "cryptohome --action=unmount", but without the username.
func (c *cryptohomeBinary) unmountAll(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=unmount")
}

// lockToSingleUserMountUntilReboot calls "cryptohome --action=lock_to_single_user_mount_until_reboot"
func (c *cryptohomeBinary) lockToSingleUserMountUntilReboot(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=lock_to_single_user_mount_until_reboot", "--user="+username)
}

// dumpKeyset calls "cryptohome --action=dump_keyset".
func (c *cryptohomeBinary) dumpKeyset(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=dump_keyset", "--user="+username)
}

// getEnrollmentID calls "cryptohome --action=get_enrollment_id".
func (c *cryptohomeBinary) getEnrollmentID(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=get_enrollment_id")
}

// tpmAttestationDeleteKeys calls "cryptohome --action=tpm_attestation_delete_keys".
func (c *cryptohomeBinary) tpmAttestationDeleteKeys(ctx context.Context, username, prefix string) ([]byte, error) {
	return c.call(ctx, "--action=tpm_attestation_delete_keys", "--user="+username, "--prefix="+prefix)
}

// pkcs11SystemTokenInfo calls "cryptohome --action=pkcs11_get_system_token_info".
func (c *cryptohomeBinary) pkcs11SystemTokenInfo(ctx context.Context) ([]byte, error) {
	out, err := c.call(ctx, "--action=pkcs11_get_system_token_info")
	return out, err
}

// pkcs11UserTokenInfo calls "cryptohome --action=pkcs11_get_user_token_info". (and gets the user token status)
func (c *cryptohomeBinary) pkcs11UserTokenInfo(ctx context.Context, username string) ([]byte, error) {
	out, err := c.call(ctx, "--action=pkcs11_get_user_token_info", "--user="+username)
	return out, err
}

// getFirmwareManagementParameters calls "cryptohome --action=get_firmware_management_parameters".
func (c *cryptohomeBinary) getFirmwareManagementParameters(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=get_firmware_management_parameters")
}

// setFirmwareManagementParameters calls "cryptohome --action=set_firmware_management_parameters".
func (c *cryptohomeBinary) setFirmwareManagementParameters(ctx context.Context, flags, hash string) ([]byte, error) {
	return c.call(ctx, "--action=set_firmware_management_parameters", "--flags="+flags, "--developer_key_hash="+hash)
}

// removeFirmwareManagementParameters calls "cryptohome --action=remove_firmware_management_parameters".
func (c *cryptohomeBinary) removeFirmwareManagementParameters(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=remove_firmware_management_parameters")
}

// getAccountDiskUsage calls "cryptohome --action=get_account_disk_usage".
func (c *cryptohomeBinary) getAccountDiskUsage(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=get_account_disk_usage", "--user="+username)
}
