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

// newCryptohomeBinary is a factory function to create a
// cryptohomeBinary instance.
func newCryptohomeBinary(r CmdRunner) *cryptohomeBinary {
	return &cryptohomeBinary{r}
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

// installAttributesGetStatus calls "cryptohome --action=install_attributes_get_status".
func (c *cryptohomeBinary) installAttributesGetStatus(ctx context.Context) (string, error) {
	out, err := c.call(ctx, "--action=install_attributes_get_status")
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
func (c *cryptohomeBinary) mountEx(ctx context.Context, username string, doesCreate bool, label string, extraFlags []string) ([]byte, error) {
	args := []string{"--action=mount_ex", "--user=" + username, "--key_label=" + label}
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
func (c *cryptohomeBinary) checkKeyEx(ctx context.Context, username, label string, extraFlags []string) ([]byte, error) {
	args := []string{"--action=check_key_ex", "--user=" + username, "--key_label=" + label}
	args = append(args, extraFlags...)
	return c.call(ctx, args...)
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

// getSupportedKeyPolicies calls "cryptohome --action=get_supported_key_policies".
func (c *cryptohomeBinary) getSupportedKeyPolicies(ctx context.Context) ([]byte, error) {
	return c.call(ctx, "--action=get_supported_key_policies")
}

// getKeyData calls "cryptohome --action=get_key_data_ex".
func (c *cryptohomeBinary) getKeyData(ctx context.Context, username, keyLabel string) ([]byte, error) {
	return c.call(ctx, "--action=get_key_data_ex", "--user="+username, "--key_label="+keyLabel)
}
