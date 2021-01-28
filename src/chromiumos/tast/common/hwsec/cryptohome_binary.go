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
func NewCryptohomeBinary(r CmdRunner) *CryptohomeBinary {
	return &CryptohomeBinary{r}
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

// GetStatusString calls "cryptohome --action=status".
func (c *CryptohomeBinary) GetStatusString(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=status")
	return string(out), err
}

// TPMClearStoredPassword calls "cryptohome --action=tpm_clear_stored_password".
func (c *CryptohomeBinary) TPMClearStoredPassword(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=tpm_clear_stored_password")
}

// TPMAttestationGetKeyPayload calls "cryptohome --action=tpm_attestation_get_key_payload".
func (c *CryptohomeBinary) TPMAttestationGetKeyPayload(
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

// TPMAttestationRegisterKey calls "cryptohome --action=tpm_attestation_register_key".
func (c *CryptohomeBinary) TPMAttestationRegisterKey(
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

// TPMAttestationSetKeyPayload calls "cryptohome --action=tpm_attestation_set_key_payload".
func (c *CryptohomeBinary) TPMAttestationSetKeyPayload(
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

// InstallAttributesGet calls "cryptohome --action=install_attributes_get".
func (c *CryptohomeBinary) InstallAttributesGet(ctx context.Context, attributeName string) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_get", "--name="+attributeName)
	return string(out), err
}

// InstallAttributesSet calls "cryptohome --action=install_attributes_set".
func (c *CryptohomeBinary) InstallAttributesSet(ctx context.Context, attributeName, attributeValue string) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_set", "--name="+attributeName, "--value="+attributeValue)
	return string(out), err
}

// InstallAttributesFinalize calls "cryptohome --action=install_attributes_finalize".
func (c *CryptohomeBinary) InstallAttributesFinalize(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_finalize")
	return string(out), err
}

// InstallAttributesCount calls "cryptohome --action=install_attributes_count".
func (c *CryptohomeBinary) InstallAttributesCount(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_count")
	return string(out), err
}

// InstallAttributesIsReady calls "cryptohome --action=install_attributes_is_ready".
func (c *CryptohomeBinary) InstallAttributesIsReady(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_is_ready")
	return string(out), err
}

// InstallAttributesIsSecure calls "cryptohome --action=install_attributes_is_secure".
func (c *CryptohomeBinary) InstallAttributesIsSecure(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_is_secure")
	return string(out), err
}

// InstallAttributesIsInvalid calls "cryptohome --action=install_attributes_is_invalid".
func (c *CryptohomeBinary) InstallAttributesIsInvalid(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_is_invalid")
	return string(out), err
}

// InstallAttributesIsFirstInstall calls "cryptohome --action=install_attributes_is_first_install".
func (c *CryptohomeBinary) InstallAttributesIsFirstInstall(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_is_first_install")
	return string(out), err
}

// IsMounted calls "cryptohome --action=is_mounted".
func (c *CryptohomeBinary) IsMounted(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=is_mounted")
}

// MountEx calls "cryptohome --action=mount_ex".
func (c *CryptohomeBinary) MountEx(ctx context.Context, username, password string, doesCreate bool, label string, extraFlags []string) ([]byte, error) {
	args := []string{"--action=mount_ex", "--user=" + username, "--password=" + password, "--key_label=" + label}
	if doesCreate {
		args = append(args, "--create")
	}
	args = append(args, extraFlags...)
	return c.call(ctx, args...)
}

// GetSanitizedUsername calls "cryptohome --action=obfuscate_user".
func (c *CryptohomeBinary) GetSanitizedUsername(ctx context.Context, username string, useDBus bool) ([]byte, error) {
	args := []string{"--action=obfuscate_user", "--user=" + username}
	if useDBus {
		args = append(args, "--use_dbus")
	}
	return c.call(ctx, args...)
}

// GetSystemSalt calls "cryptohome --action=get_system_salt".
func (c *CryptohomeBinary) GetSystemSalt(ctx context.Context, useDBus bool) ([]byte, error) {
	args := []string{"--action=get_system_salt"}
	if useDBus {
		args = append(args, "--use_dbus")
	}
	return c.call(ctx, args...)
}

// CheckKeyEx calls "cryptohome --action=check_key_ex".
func (c *CryptohomeBinary) CheckKeyEx(ctx context.Context, username, password, label string) ([]byte, error) {
	return c.call(ctx, "--action=check_key_ex", "--user="+username, "--password="+password, "--key_label="+label)
}

// ListKeysEx calls "cryptohome --action=list_keys_ex".
func (c *CryptohomeBinary) ListKeysEx(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=list_keys_ex", "--user="+username)
}

// AddKeyEx calls "cryptohome --action=add_key_ex".
func (c *CryptohomeBinary) AddKeyEx(ctx context.Context, username, password, label, newPassword, newLabel string, lowEntropy bool) ([]byte, error) {
	args := []string{"--action=add_key_ex", "--user=" + username, "--password=" + password, "--key_label=" + label, "--new_password=" + newPassword, "--new_key_label=" + newLabel}
	if lowEntropy {
		args = append(args, "--key_policy=le")
	}
	return c.call(ctx, args...)
}

// RemoveKeyEx calls "cryptohome --action=remove_key_ex".
func (c *CryptohomeBinary) RemoveKeyEx(ctx context.Context, username, password, removeLabel string) ([]byte, error) {
	return c.call(ctx, "--action=remove_key_ex", "--user="+username, "--password="+password, "--remove_key_label="+removeLabel)
}

// MigrateKeyEx calls "cryptohome --action=migrate_key_ex".
func (c *CryptohomeBinary) MigrateKeyEx(ctx context.Context, username, password, label, newPassword string) ([]byte, error) {
	return c.call(ctx, "--action=migrate_key_ex", "--user="+username, "--old_password="+password, "--key_label="+label, "--password="+newPassword)
}

// Remove calls "cryptohome --action=remove".
func (c *CryptohomeBinary) Remove(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=remove", "--user="+username, "--force")
}

// Unmount calls "cryptohome --action=unmount".
func (c *CryptohomeBinary) Unmount(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=unmount", "--user="+username)
}

// UnmountAll calls "cryptohome --action=unmount", but without the username.
func (c *CryptohomeBinary) UnmountAll(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=unmount")
}

// LockToSingleUserMountUntilReboot calls "cryptohome --action=lock_to_single_user_mount_until_reboot"
func (c *CryptohomeBinary) LockToSingleUserMountUntilReboot(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=lock_to_single_user_mount_until_reboot", "--user="+username)
}

// DumpKeyset calls "cryptohome --action=dump_keyset".
func (c *CryptohomeBinary) DumpKeyset(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=dump_keyset", "--user="+username)
}

// TPMAttestationDeleteKeys calls "cryptohome --action=tpm_attestation_delete_keys".
func (c *CryptohomeBinary) TPMAttestationDeleteKeys(ctx context.Context, username, prefix string) ([]byte, error) {
	return c.call(ctx, "--action=tpm_attestation_delete_keys", "--user="+username, "--prefix="+prefix)
}

// Pkcs11SystemTokenInfo calls "cryptohome --action=pkcs11_get_system_token_info".
func (c *CryptohomeBinary) Pkcs11SystemTokenInfo(ctx context.Context) ([]byte, error) {
	out, err := c.call(ctx, "--action=pkcs11_get_system_token_info")
	return out, err
}

// Pkcs11UserTokenInfo calls "cryptohome --action=pkcs11_get_user_token_info". (and gets the user token status)
func (c *CryptohomeBinary) Pkcs11UserTokenInfo(ctx context.Context, username string) ([]byte, error) {
	out, err := c.call(ctx, "--action=pkcs11_get_user_token_info", "--user="+username)
	return out, err
}

// GetFirmwareManagementParameters calls "cryptohome --action=get_firmware_management_parameters".
func (c *CryptohomeBinary) GetFirmwareManagementParameters(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=get_firmware_management_parameters")
}

// SetFirmwareManagementParameters calls "cryptohome --action=set_firmware_management_parameters".
func (c *CryptohomeBinary) SetFirmwareManagementParameters(ctx context.Context, flags, hash string) ([]byte, error) {
	return c.call(ctx, "--action=set_firmware_management_parameters", "--flags="+flags, "--developer_key_hash="+hash)
}

// RemoveFirmwareManagementParameters calls "cryptohome --action=remove_firmware_management_parameters".
func (c *CryptohomeBinary) RemoveFirmwareManagementParameters(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=remove_firmware_management_parameters")
}

// GetAccountDiskUsage calls "cryptohome --action=get_account_disk_usage".
func (c *CryptohomeBinary) GetAccountDiskUsage(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "--action=get_account_disk_usage", "--user="+username)
}
