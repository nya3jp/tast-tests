// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"

	cpb "chromiumos/system_api/cryptohome_proto"
	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	cryptohomeWrappedKeysetString          = "TPM_WRAPPED"
	installAttributesFinalizeSuccessOutput = "InstallAttributesFinalize(): 1"
	listKeysExLabelPrefix                  = "Label: "
	addKeyExSuccessMessage                 = "Key added."
	removeKeyExSuccessMessage              = "Key removed."
	migrateKeyExSucessMessage              = "Key migration succeeded."
	shadowHome                             = "/home/.shadow"
)

var (
	// userHashRegexp extracts the hash from a cryptohome dir's path.
	// Example: "/home/.shadow/118c4648065f5cd3660e17a53533ec7bc924d01f"
	userHashRegexp = regexp.MustCompile("^/home/user/([[:xdigit:]]+)$")

	// authSessionIDRegexp matches the auth session ID.
	// It would match "auth_session_id:*"
	authSessionIDRegexp = regexp.MustCompile(`(auth_session_id:)(.+)(\n)`)

	// authIntentsIDRegexp finds if
	// It would match "auth_session_id:*"
	authFactorPINRegexp = regexp.MustCompile(`(type: AUTH_FACTOR_TYPE_PIN)(\n)`)

	// recoveryRequestRegexp matches the recovery request value.
	// It would match "recovery_request:*"
	recoveryRequestRegexp = regexp.MustCompile(`recovery_request:(.+)\n`)
)

func getLastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

const (
	// OldCryptohomeMountAPI makes the client use old api.
	OldCryptohomeMountAPI = iota
	// NewCryptohomeMountAPI makes the client use new api.
	NewCryptohomeMountAPI
)

// CryptohomeMountAPI denotes Mount API type to use.
type CryptohomeMountAPI int64

// CryptohomeMountAPIParam is a helper struct to parametrize tests for old and
// new mount APIs.
type CryptohomeMountAPIParam struct {
	MountAPI CryptohomeMountAPI
}

// CryptohomeClient wraps and the functions of cryptohomeBinary and parses the outputs to
// structured data.
type CryptohomeClient struct {
	runner               CmdRunner
	binary               *cryptohomeBinary
	cryptohomePathBinary *cryptohomePathBinary
	daemonController     *DaemonController
	mountAPI             CryptohomeMountAPI
}

// NewCryptohomeClient creates a new CryptohomeClient.
func NewCryptohomeClient(r CmdRunner) *CryptohomeClient {
	return &CryptohomeClient{
		runner:               r,
		binary:               newCryptohomeBinary(r),
		cryptohomePathBinary: newCryptohomePathBinary(r),
		daemonController:     NewDaemonController(r),
		mountAPI:             OldCryptohomeMountAPI,
	}
}

// SetMountAPIParam sets Mount API type.
func (u *CryptohomeClient) SetMountAPIParam(param *CryptohomeMountAPIParam) {
	u.mountAPI = param.MountAPI
}

// InstallAttributesStatus retrieves the a status string from cryptohome. The status string is in JSON format and holds the various cryptohome related status.
func (u *CryptohomeClient) InstallAttributesStatus(ctx context.Context) (string, error) {
	out, err := u.binary.installAttributesGetStatus(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get Install Attributes status with the following output %q", out)
	}
	// Strip the ending new line.
	out = strings.TrimSuffix(out, "\n")

	return out, err
}

// InstallAttributesGet retrieves the install attributes with the name of attributeName, and returns the tuple (value, error), whereby value is the value of the attributes, and error is nil iff the operation is successful, otherwise error is the error that occurred.
func (u *CryptohomeClient) InstallAttributesGet(ctx context.Context, attributeName string) (string, error) {
	out, err := u.binary.installAttributesGet(ctx, attributeName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get Install Attributes with the following output %q", out)
	}
	// Strip the ending new line.
	out = strings.TrimSuffix(out, "\n")

	return out, err
}

// InstallAttributesSet sets the install attributes with the name of attributeName with the value attributeValue, and returns error, whereby error is nil iff the operation is successful, otherwise error is the error that occurred.
func (u *CryptohomeClient) InstallAttributesSet(ctx context.Context, attributeName, attributeValue string) error {
	out, err := u.binary.installAttributesSet(ctx, attributeName, attributeValue)
	if err != nil {
		return errors.Wrapf(err, "failed to set Install Attributes with the following output %q", out)
	}
	return nil
}

// InstallAttributesFinalize finalizes the install attributes, and returns error encountered if any. error is nil iff the operation completes successfully.
func (u *CryptohomeClient) InstallAttributesFinalize(ctx context.Context) error {
	out, err := u.binary.installAttributesFinalize(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to finalize Install Attributes with the following output %q", out)
	}
	if !strings.Contains(out, installAttributesFinalizeSuccessOutput) {
		return errors.Errorf("failed to finalize Install Attributes, incorrect output message %q", out)
	}
	return nil
}

// InstallAttributesCount retrieves the number of entries in install attributes. It returns count and error. error is nil iff the operation completes successfully, and in this case count holds the number of entries in install attributes.
func (u *CryptohomeClient) InstallAttributesCount(ctx context.Context) (int, error) {
	out, err := u.binary.installAttributesCount(ctx)
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
func (u *CryptohomeClient) InstallAttributesIsReady(ctx context.Context) (bool, error) {
	out, err := u.binary.installAttributesIsReady(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsReady")
}

// InstallAttributesIsSecure checks if install attributes is secure, returns isSecure and error. error is nil iff the operation completes successfully, and in this case isSecure is whether install attributes is secure.
func (u *CryptohomeClient) InstallAttributesIsSecure(ctx context.Context) (bool, error) {
	out, err := u.binary.installAttributesIsSecure(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsSecure")
}

// InstallAttributesIsInvalid checks if install attributes is invalid, returns isInvalid and error. error is nil iff the operation completes successfully, and in this case isInvalid is whether install attributes is invalid.
func (u *CryptohomeClient) InstallAttributesIsInvalid(ctx context.Context) (bool, error) {
	out, err := u.binary.installAttributesIsInvalid(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsInvalid")
}

// InstallAttributesIsFirstInstall checks if install attributes is the first install state, returns isFirstInstall and error. error is nil iff the operation completes successfully, and in this case isFirstInstall is whether install attributes is in the first install state.
func (u *CryptohomeClient) InstallAttributesIsFirstInstall(ctx context.Context) (bool, error) {
	out, err := u.binary.installAttributesIsFirstInstall(ctx)
	return installAttributesBooleanHelper(out, err, "InstallAttributesIsFirstInstall")
}

// IsMounted checks if any vault is mounted.
func (u *CryptohomeClient) IsMounted(ctx context.Context) (bool, error) {
	out, err := u.binary.isMounted(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to check if mounted")
	}
	result, err := strconv.ParseBool(strings.TrimSuffix(string(out), "\n"))
	if err != nil {
		return false, errors.Wrap(err, "failed to parse output from cryptohome")
	}
	return result, nil
}

// Unmount unmounts the vault for username.
func (u *CryptohomeClient) Unmount(ctx context.Context, username string) (bool, error) {
	if err := u.UnmountAll(ctx); err != nil {
		return false, errors.Wrap(err, "failed to unmount all")
	}
	return true, nil
}

// UnmountAll unmounts all vault.
func (u *CryptohomeClient) UnmountAll(ctx context.Context) error {
	goal, _, _, err := u.daemonController.Status(ctx, UIDaemon)
	if err != nil {
		return errors.Wrap(err, "failed to get the status of ui")
	}
	if goal == startGoal {
		// Restart the UI to make sure nothing is still using cryptohome and unmount the mount point.
		if err := u.daemonController.Restart(ctx, UIDaemon); err != nil {
			return errors.Wrap(err, "failed to restart the UI")
		}
	} else {
		// Running the ui-post-stop directly if UI doesn't start.
		if _, err := u.runner.Run(ctx, "/usr/share/cros/init/ui-post-stop"); err != nil {
			return errors.Wrap(err, "failed to run ui-post-stop")
		}
	}
	return nil
}

// VaultConfig specifies the extra options to Mounting/Creating a vault.
type VaultConfig struct {
	// Ephemeral is set to true if the vault is ephemeral, that is, the vault is erased after the user logs out.
	Ephemeral bool

	// Ecryptfs is set to true if the vault should be backed by eCryptfs.
	Ecryptfs bool

	// CreateEmptyLabel is set to true if vault should be created with no label.
	CreateEmptyLabel bool

	// KioskUser is set to true if the vault should be mounted as a Vault.
	KioskUser bool
}

// NewVaultConfig creates a default vault config.
func NewVaultConfig() *VaultConfig {
	return &VaultConfig{}
}

// vaultConfigToExtraFlags converts VaultConfig to flags accepted by the cryptohome command line.
func vaultConfigToExtraFlags(config *VaultConfig) []string {
	const (
		// mountFlagEphemeral is the flag passed to cryptohome command line
		// when you want the vault to be ephemeral.
		mountFlagEphemeral = "--ensure_ephemeral"
		// mountFlagEcryptfs is the flag passed to cryptohome command line
		// when you want the vault to use ecryptfs.
		mountFlagEcryptfs = "--ecryptfs"
		// mountFlagCreateEmptyLabel is the flag passed to cryptohome command
		// line when you want the legacy behavior of using an empty label in
		// authorization request.
		mountFlagCreateEmptyLabel = "--create_empty_label"
		// kioskUser is the flag passed to cryptohome command line when you
		// want the to mount kiosk user.
		kioskUser = "--public_mount"
	)

	var extraFlags []string
	if config.Ephemeral {
		extraFlags = append(extraFlags, mountFlagEphemeral)
	}
	if config.Ecryptfs {
		extraFlags = append(extraFlags, mountFlagEcryptfs)
	}
	if config.CreateEmptyLabel {
		extraFlags = append(extraFlags, mountFlagCreateEmptyLabel)
	}
	if config.KioskUser {
		extraFlags = append(extraFlags, kioskUser)
	}
	return extraFlags
}

const (
	// PassAuth is the constant for AuthConfig.AuthType, representing password authentication.
	PassAuth = iota
	// ChallengeAuth is the constant for AuthConfig.AuthType, representing challenge-response authenticating.
	ChallengeAuth = iota
)

// AuthConfig represents the data required to authenticate a user.
// It could be password authentication or challenge-response authentication.
type AuthConfig struct {
	// AuthType is the type of authentication.
	AuthType int

	// Username is the username for authentication.
	Username string

	// Password is the user's password.
	// Used only when AuthType is PassAuth
	Password string

	// KeyDelegateName is the dbus service name for the authentication delegate.
	// Used only when AuthType is ChallengeAuth
	KeyDelegateName string

	// KeyDelegatePath is the dbus service path for the authentication delegate.
	// Used only when AuthType is ChallengeAuth
	KeyDelegatePath string

	// ChallengeSPKI is the SPKI that contains the public key for challenge response. It's in DER format.
	// Used only when AuthType is ChallengeAuth
	ChallengeSPKI []byte

	// ChallengeAlgs is the list of cryptographic algorithms to use for challenge response.
	// Used only when AuthType is ChallengeAuth
	ChallengeAlgs []cpb.ChallengeSignatureAlgorithm
}

// NewPassAuthConfig creates an AuthConfig for Password Authentication.
func NewPassAuthConfig(username, password string) *AuthConfig {
	config := &AuthConfig{}
	config.AuthType = PassAuth
	config.Username = username
	config.Password = password
	return config
}

// NewChallengeAuthConfig creates an AuthConfig for Challenge-Response Authentication.
func NewChallengeAuthConfig(username, keyDelegateName, keyDelegatePath string, challengeSPKI []byte, challengeAlgs []cpb.ChallengeSignatureAlgorithm) *AuthConfig {
	config := &AuthConfig{}
	config.AuthType = ChallengeAuth
	config.Username = username
	config.KeyDelegateName = keyDelegateName
	config.KeyDelegatePath = keyDelegatePath
	config.ChallengeSPKI = challengeSPKI
	config.ChallengeAlgs = challengeAlgs
	return config
}

// authConfigToExtraFlags converts AuthConfig to flags accepted by the cryptohome command line.
func authConfigToExtraFlags(config *AuthConfig) []string {
	var extraFlags []string
	if config.AuthType == PassAuth {
		extraFlags = append(extraFlags, "--password="+config.Password)
	} else if config.AuthType == ChallengeAuth {
		var algs = []string{}
		for _, a := range config.ChallengeAlgs {
			algs = append(algs, a.String())
		}
		extraFlags = append(extraFlags, "--challenge_alg="+strings.Join(algs, ","))
		extraFlags = append(extraFlags, "--challenge_spki="+hex.EncodeToString(config.ChallengeSPKI))
		extraFlags = append(extraFlags, "--key_delegate_name="+config.KeyDelegateName)
		extraFlags = append(extraFlags, "--key_delegate_path="+config.KeyDelegatePath)
	} else {
		panic("Invalid AuthType in CryptohomeClient")
	}
	return extraFlags
}

func (u *CryptohomeClient) createUserWithAuthSession(ctx context.Context, username, password, keyLabel string, isKioskUser bool) (string, error) {
	// Start an Auth session and get an authSessionID.
	authSessionID, err := u.StartAuthSession(ctx, username, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		return "", errors.Wrap(err, "failed to start Auth session")
	}
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	if err := u.AddCredentialsWithAuthSession(ctx, username, password, keyLabel, authSessionID, isKioskUser); err != nil {
		return "", errors.Wrap(err, "failed to add credentials with AuthSession")
	}
	testing.ContextLog(ctx, "Added credentials successfully")
	if err := u.AuthenticateAuthSession(ctx, password, keyLabel, authSessionID, isKioskUser); err != nil {
		return "", errors.Wrap(err, "failed to authenticate with AuthSession")
	}
	testing.ContextLog(ctx, "User authenticated successfully")

	// This is a no-op for now since AddCredentials.. above will already create
	// the user.
	if err := u.CreatePersistentUser(ctx, authSessionID); err != nil {
		return "", errors.Wrap(err, "failed to create persistent user")
	}
	return authSessionID, nil
}

func (u *CryptohomeClient) authenticateWithAuthSession(ctx context.Context, username, password, keyLabel string, isEphemeral, isKioskUser bool) (string, error) {
	// Start an Auth session and get an authSessionID.
	authSessionID, err := u.StartAuthSession(ctx, username, isEphemeral, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		return "", errors.Wrap(err, "failed to start Auth session")
	}
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	// Authenticate the same AuthSession using authSessionID.
	// If we cannot authenticate, do not proceed with mount and unmount.
	if err := u.AuthenticateAuthSession(ctx, password, keyLabel, authSessionID, isKioskUser); err != nil {
		return "", errors.Wrap(err, "failed to authenticate with AuthSession")
	}
	testing.ContextLog(ctx, "User authenticated successfully")

	return authSessionID, nil
}

func (u *CryptohomeClient) mountVaultWithNewAPI(ctx context.Context, label string, authConfig *AuthConfig, create bool, vaultConfig *VaultConfig) error {
	var authSessionID string
	var err error

	if create {
		authSessionID, err = u.createUserWithAuthSession(ctx, authConfig.Username, authConfig.Password, label, vaultConfig.KioskUser)
		if err != nil {
			return errors.Wrap(err, "failed to create user")
		}
	} else {
		authSessionID, err = u.authenticateWithAuthSession(ctx, authConfig.Username, authConfig.Password, label, vaultConfig.Ephemeral, vaultConfig.KioskUser)
		if err != nil {
			return errors.Wrap(err, "failed to authenticate user")
		}
	}

	if !vaultConfig.Ephemeral {
		if err := u.PreparePersistentVault(ctx, authSessionID, vaultConfig.Ecryptfs); err != nil {
			return errors.Wrap(err, "failed to prepare persistent vault")
		}
	} else {
		if err := u.PrepareEphemeralVault(ctx, authSessionID); err != nil {
			return errors.Wrap(err, "failed to prepare ephemeral vault")
		}
	}

	return nil
}

// MountVault mounts the vault for username; creates a new vault if no vault yet if create is true. error is nil if the operation completed successfully.
func (u *CryptohomeClient) MountVault(ctx context.Context, label string, authConfig *AuthConfig, create bool, vaultConfig *VaultConfig) error {
	if u.mountAPI == NewCryptohomeMountAPI {
		return u.mountVaultWithNewAPI(ctx, label, authConfig, create, vaultConfig)
	}

	extraFlags := vaultConfigToExtraFlags(vaultConfig)
	extraFlags = append(extraFlags, authConfigToExtraFlags(authConfig)...)

	if _, err := u.binary.mountEx(ctx, authConfig.Username, create, label, extraFlags); err != nil {
		return errors.Wrap(err, "failed to mount")
	}
	return nil
}

// MountGuest creates a mount point for a guest user; error is nil if the operation completed successfully.
func (u *CryptohomeClient) MountGuest(ctx context.Context) error {
	if _, err := u.binary.mountGuestEx(ctx); err != nil {
		return errors.Wrap(err, "failed to mount guest")
	}
	return nil
}

// MountKiosk creates a mount point for a kiosk; error is nil if the operation completed successfully.
func (u *CryptohomeClient) MountKiosk(ctx context.Context) error {
	extraFlags := []string{"--public_mount"}
	if _, err := u.binary.mountEx(ctx, "kiosk", true, "public_mount", extraFlags); err != nil {
		return errors.Wrap(err, "failed to mount kiosk")
	}
	return nil
}

// GetSanitizedUsername computes the sanitized username for the given user.
// If useDBus is true, the sanitized username will be computed by cryptohome (through dbus). Otherwise, it'll be computed directly by libbrillo (without dbus).
func (u *CryptohomeClient) GetSanitizedUsername(ctx context.Context, username string, useDBus bool) (string, error) {
	out, err := u.binary.getSanitizedUsername(ctx, username, useDBus)
	if err != nil {
		return "", errors.Wrap(err, "failed to call cryptohomeBinary.GetSanitizedUsername")
	}
	outs := strings.TrimSpace(string(out))
	exp := regexp.MustCompile("^[a-f0-9]{40}$")
	// A proper sanitized username should be a hex string of length 40.
	if !exp.MatchString(outs) {
		return "", errors.Errorf("invalid sanitized username %q", outs)
	}
	return outs, nil
}

// GetSystemSalt retrieves the system salt and return the hex encoded version of it.
// If useDBus is true, the system salt will be retrieved from cryptohome (through dbus). Otherwise, it'll be loaded directly by libbrillo (without dbus).
func (u *CryptohomeClient) GetSystemSalt(ctx context.Context, useDBus bool) (string, error) {
	out, err := u.binary.getSystemSalt(ctx, useDBus)
	if err != nil {
		return "", errors.Wrap(err, "failed to call cryptohomeBinary.GetSystemSalt")
	}
	outs := strings.TrimSpace(string(out))
	exp := regexp.MustCompile("^[a-f0-9]+$")
	// System salt should be a non-empty hex string.
	if !exp.MatchString(outs) {
		return "", errors.Errorf("invalid system salt %q", outs)
	}
	return outs, nil
}

// CheckVaultAndUnlockWebAuthnSecret checks the vault via |CheckKeyEx| dbus method, and set the unlock_webauthn_secret param to true.
func (u *CryptohomeClient) CheckVaultAndUnlockWebAuthnSecret(ctx context.Context, label string, authConfig *AuthConfig) (bool, error) {
	extraFlags := authConfigToExtraFlags(authConfig)
	_, err := u.binary.checkKeyEx(ctx, authConfig.Username, label, true, extraFlags)
	if err != nil {
		return false, errors.Wrap(err, "failed to check key")
	}
	return true, nil
}

// CheckVault checks the vault via |CheckKeyEx| dbus method.
func (u *CryptohomeClient) CheckVault(ctx context.Context, label string, authConfig *AuthConfig) (bool, error) {
	extraFlags := authConfigToExtraFlags(authConfig)
	_, err := u.binary.checkKeyEx(ctx, authConfig.Username, label, false, extraFlags)
	if err != nil {
		return false, errors.Wrap(err, "failed to check key")
	}
	return true, nil
}

// ListVaultKeys queries the vault associated with user username, and returns nil for error iff the operation is completed successfully, in that case, the returned slice of string contains the labels of keys belonging to that vault.
func (u *CryptohomeClient) ListVaultKeys(ctx context.Context, username string) ([]string, error) {
	binaryOutput, err := u.binary.listKeysEx(ctx, username)
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
func (u *CryptohomeClient) AddVaultKey(ctx context.Context, username, password, label, newPassword, newLabel string, lowEntropy bool) error {
	binaryOutput, err := u.binary.addKeyEx(ctx, username, password, label, newPassword, newLabel, lowEntropy)
	if err != nil {
		return errors.Wrap(err, "failed to call AddKeyEx")
	}

	output := string(binaryOutput)
	if !strings.Contains(output, addKeyExSuccessMessage) {
		testing.ContextLogf(ctx, "Incorrect AddKeyEx message; got %q, want %q", output, addKeyExSuccessMessage)
		return errors.Errorf("incorrect message from AddKeyEx; got %q, want %q", output, addKeyExSuccessMessage)
	}

	return nil
}

// RemoveVaultKey removes the key with label removeLabel from user specified by username's vault. password for username is supplied so the operation can be proceeded. nil is returned iff the operation is successful.
func (u *CryptohomeClient) RemoveVaultKey(ctx context.Context, username, password, removeLabel string) error {
	binaryOutput, err := u.binary.removeKeyEx(ctx, username, password, removeLabel)
	if err != nil {
		return errors.Wrap(err, "failed to call RemoveKeyEx")
	}

	output := string(binaryOutput)
	if !strings.Contains(output, removeKeyExSuccessMessage) {
		testing.ContextLogf(ctx, "Incorrect RemoveKeyEx message; got %q, want %q", output, removeKeyExSuccessMessage)
		return errors.Errorf("incorrect message from RemoveKeyEx; got %q, want %q", output, removeKeyExSuccessMessage)
	}

	return nil
}

// ChangeVaultPassword changes the vault for user username with label and password to newPassword. nil is returned iff the operation is successful.
func (u *CryptohomeClient) ChangeVaultPassword(ctx context.Context, username, password, label, newPassword string) error {
	binaryOutput, err := u.binary.migrateKeyEx(ctx, username, password, label, newPassword)
	if err != nil {
		return errors.Wrap(err, "failed to call MigrateKeyEx")
	}

	output := string(binaryOutput)
	if !strings.Contains(output, migrateKeyExSucessMessage) {
		testing.ContextLogf(ctx, "Incorrect MigrateKeyEx message; got %q, want %q", output, migrateKeyExSucessMessage)
		return errors.Errorf("incorrect message from MigrateKeyEx; got %q, want %q", output, migrateKeyExSucessMessage)
	}

	return nil
}

// RemoveVault remove the vault for username.
func (u *CryptohomeClient) RemoveVault(ctx context.Context, username string) (bool, error) {
	_, err := u.binary.remove(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to remove vault")
	}
	return true, nil
}

// UnmountAndRemoveVault attempts to unmount all vaults and remove the vault for username.
// This is a simple helper, and it's created because this is a commonly used combination.
func (u *CryptohomeClient) UnmountAndRemoveVault(ctx context.Context, username string) error {
	// Note: Vault must be unmounted to be removed.
	if err := u.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount all")
	}

	if _, err := u.RemoveVault(ctx, username); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}

	return nil
}

// LockToSingleUserMountUntilReboot will block users other than the specified from logging in if the call succeeds, and in that case, nil is returned.
func (u *CryptohomeClient) LockToSingleUserMountUntilReboot(ctx context.Context, username string) error {
	const successMessage = "Login disabled."
	binaryOutput, err := u.binary.lockToSingleUserMountUntilReboot(ctx, username)
	if err != nil {
		return errors.Wrap(err, "failed to call lock_to_single_user_mount_until_reboot")
	}
	output := strings.TrimSuffix(string(binaryOutput), "\n")
	// Note that we are checking the output message again because we want to
	// catch cases that return 0 but wasn't successful.
	if !strings.Contains(output, successMessage) {
		return errors.Errorf("incorrect message from LockToSingleUserMountUntilReboot; got %q, want %q", output, successMessage)
	}
	return nil
}

// IsTPMWrappedKeySet checks if the current user vault is TPM-backed.
func (u *CryptohomeClient) IsTPMWrappedKeySet(ctx context.Context, username string) (bool, error) {
	out, err := u.binary.dumpKeyset(ctx, username)
	if err != nil {
		return false, errors.Wrap(err, "failed to dump keyset")
	}
	return strings.Contains(string(out), cryptohomeWrappedKeysetString), nil
}

// CheckTPMWrappedUserKeyset checks if the given user's keyset is backed by TPM.
// Returns an error if the keyset is not TPM-backed or if there's anything wrong.
func (u *CryptohomeClient) CheckTPMWrappedUserKeyset(ctx context.Context, user string) error {
	if keysetTPMBacked, err := u.IsTPMWrappedKeySet(ctx, user); err != nil {
		return errors.Wrap(err, "failed to check user keyset")
	} else if !keysetTPMBacked {
		return errors.New("user keyset is not TPM-backed")
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

// GetTokenInfoForUser retrieve the token label, pin and slot for the user token if username is non-empty, or system token if username is empty.
func (u *CryptohomeClient) GetTokenInfoForUser(ctx context.Context, username string) (returnedLabel, returnedPin string, returnedSlot int, returnedErr error) {
	cmdOutput := ""
	if username == "" {
		// We want the system token.
		out, err := u.binary.pkcs11SystemTokenInfo(ctx)
		cmdOutput = string(out)
		if err != nil {
			return "", "", -1, errors.Wrapf(err, "failed to get system token info %q", cmdOutput)
		}
	} else {
		// We want the user token.
		out, err := u.binary.pkcs11UserTokenInfo(ctx, username)
		cmdOutput = string(out)
		if err != nil {
			return "", "", -1, errors.Wrapf(err, "failed to get user token info %q", cmdOutput)
		}
	}
	label, pin, slot, err := parseTokenStatus(cmdOutput)
	if err != nil {
		return "", "", -1, errors.Wrapf(err, "failed to parse token status %q", cmdOutput)
	}
	return label, pin, slot, nil
}

// GetTokenForUser retrieve the token slot for the user token if username is non-empty, or system token if username is empty.
func (u *CryptohomeClient) GetTokenForUser(ctx context.Context, username string) (int, error) {
	_, _, slot, err := u.GetTokenInfoForUser(ctx, username)
	return slot, err
}

// WaitForUserToken wait until the user token for the specified user is ready. Otherwise, return an error if the token is still unavailable.
func (u *CryptohomeClient) WaitForUserToken(ctx context.Context, username string) error {
	const waitForUserTokenTimeout = 15 * time.Second

	if username == "" {
		// This method is for user token, not system token.
		// Note: For those who want to wait for system token, system token is always ready, no need to wait.
		return errors.New("empty username in WaitForUserToken")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		slot, err := u.GetTokenForUser(ctx, username)
		if err != nil {
			// This is unexpected and shouldn't usually happen.
			return testing.PollBreak(errors.Wrapf(err, "failed to get slot ID for username %q", username))
		}
		if slot <= 0 {
			return errors.Wrapf(err, "invalid slot ID %d for username %q", slot, username)
		}
		return nil
	}, &testing.PollOptions{Timeout: waitForUserTokenTimeout}); err != nil {
		return errors.Wrap(err, "failed waiting for user token")
	}
	return nil
}

// FWMPError is a custom error type that conveys the error as well as parsed
// ErrorCode from cryptohome API.
type FWMPError struct {
	*errors.E

	// ErrorCode is the error code from FWMP methods.
	ErrorCode string
}

// GetFirmwareManagementParameters retrieves the firmware parameter flags and hash.
// It returns (flags, hash, msg, errorCode, err), whereby flags and hash is part of FWMP, and will be valid iff err is nil; msg is the message from the command line; errorCode is the error code from dbus call, if available.
// The operation is successful iff err is nil.
func (u *CryptohomeClient) GetFirmwareManagementParameters(ctx context.Context) (flags, hash string, returnedError *FWMPError) {
	binaryMsg, err := u.binary.getFirmwareManagementParameters(ctx)
	msg := string(binaryMsg)

	// Parse for the error code and stuffs because we might need the error code in the return error in case it failed.
	const flagsPrefix = "flags=0x"
	const hashPrefix = "hash="
	const errorPrefix = "error: "
	prefixes := []string{flagsPrefix, hashPrefix, errorPrefix}
	params := make(map[string]string, len(prefixes))
	for _, line := range strings.Split(msg, "\n") {
		line := strings.Trim(line, " ")
		for _, prefix := range prefixes {
			if strings.HasPrefix(line, prefix) {
				if _, existing := params[prefix]; existing {
					return "", "", &FWMPError{E: errors.Errorf("duplicate attribute %q found GetFirmwareManagementParameters output", prefix)}
				}
				params[prefix] = line[len(prefix):]
			}
		}
	}

	if err != nil {
		testing.ContextLogf(ctx, "GetFirmwareManagementParameters failed with %q", msg)
		errorCode, haveError := params[errorPrefix]
		if haveError {
			return "", "", &FWMPError{E: errors.Wrapf(err, "failed to call GetFirmwareManagementParameters command, error %q", errorCode), ErrorCode: errorCode}
		}
		return "", "", &FWMPError{E: errors.Wrap(err, "failed to call GetFirmwareManagementParameters with unknown error")}
	}

	// return the hash and flags if they exist, and return error otherwise.
	paramsFlags, haveFlags := params[flagsPrefix]
	paramsHash, haveHash := params[hashPrefix]
	if !haveFlags {
		return "", "", &FWMPError{E: errors.New("no flags in GetFirmwareManagementParameters output")}
	}
	if !haveHash {
		return "", "", &FWMPError{E: errors.New("no hash in GetFirmwareManagementParameters output")}
	}

	return paramsFlags, paramsHash, nil
}

// SetFirmwareManagementParameters sets the firmware management parameters flags and hash (both as a hex string), then returns (msg, error).
// msg is the command line output from cryptohome command; error is nil iff the operation is successful.
func (u *CryptohomeClient) SetFirmwareManagementParameters(ctx context.Context, flags, hash string) (string, error) {
	binaryMsg, err := u.binary.setFirmwareManagementParameters(ctx, "0x"+flags, hash)
	msg := string(binaryMsg)

	// Note that error code is not parsed because currently no tests requires it.

	if err != nil {
		testing.ContextLogf(ctx, "SetFirmwareManagementParameters failed with %q", msg)
		return msg, errors.Wrap(err, "failed to call SetFirmwareManagementParameters")
	}

	return msg, nil
}

// RemoveFirmwareManagementParameters removes the firmware management parameters.
// msg is the command line output from cryptohome command; error is nil iff the operation is successful.
func (u *CryptohomeClient) RemoveFirmwareManagementParameters(ctx context.Context) (string, error) {
	binaryMsg, err := u.binary.removeFirmwareManagementParameters(ctx)
	msg := string(binaryMsg)

	// Note that error code is not parsed because currently no tests requires it.

	if err != nil {
		testing.ContextLogf(ctx, "RemoveFirmwareManagementParameters failed with %q", msg)
		return msg, errors.Wrap(err, "failed to call RemoveFirmwareManagementParameters")
	}

	return msg, nil
}

// FirmwareManagementParametersInfo contains the information regarding FWMP, so that it can be backed up and restored.
type FirmwareManagementParametersInfo struct {
	// parametersExist is true iff the FWMP is set/exists on the DUT.
	parametersExist bool

	// flags contain the flags in the FWMP. This is valid iff parametersExist is true.
	flags string

	// hash contains the developer hash in the FWMP. This is valid iff parametersExist is true.
	hash string
}

// BackupFWMP backs up the current FWMP by returning the FWMP. The operation is successful iff error is nil.
func (u *CryptohomeClient) BackupFWMP(ctx context.Context) (*FirmwareManagementParametersInfo, error) {
	flags, hash, err := u.GetFirmwareManagementParameters(ctx)
	if err != nil {
		if err.ErrorCode != "CRYPTOHOME_ERROR_FIRMWARE_MANAGEMENT_PARAMETERS_INVALID" {
			return nil, errors.Wrap(err, "failed to get FWMP for backup")
		}
		// FWMP doesn't exist.
		return &FirmwareManagementParametersInfo{parametersExist: false}, nil
	}

	fwmp := FirmwareManagementParametersInfo{parametersExist: true, flags: flags, hash: hash}
	return &fwmp, nil
}

// RestoreFWMP restores the FWMP from fwmp in parameter, and return nil iff the operation is successful.
func (u *CryptohomeClient) RestoreFWMP(ctx context.Context, fwmp *FirmwareManagementParametersInfo) error {
	if !fwmp.parametersExist {
		// Parameters doesn't exist, so let's clear it.
		if _, err := u.RemoveFirmwareManagementParameters(ctx); err != nil {
			return errors.Wrap(err, "failed to clear FWMP")
		}
		return nil
	}

	// FWMP exists, so let's set the correct values.
	if _, err := u.SetFirmwareManagementParameters(ctx, fwmp.flags, fwmp.hash); err != nil {
		return errors.Wrap(err, "failed to set FWMP")
	}

	return nil
}

// GetAccountDiskUsage returns the disk space (in bytes) used by the username.
func (u *CryptohomeClient) GetAccountDiskUsage(ctx context.Context, username string) (diskUsage int64, returnedError error) {
	binaryMsg, err := u.binary.getAccountDiskUsage(ctx, username)
	msg := string(binaryMsg)
	if err != nil {
		testing.ContextLogf(ctx, "Failure to call GetAccountDiskUsage, got %q", msg)
		return -1, errors.Wrap(err, "failed to call GetAccountDiskUsage")
	}

	var result int64
	for _, s := range strings.Split(strings.TrimSpace(msg), "\n") {
		if n, err := fmt.Sscanf(s, "Account Disk Usage in bytes: %d", &result); err == nil && n == 1 {
			// We've found the output that we need.
			return result, nil
		}
	}

	testing.ContextLogf(ctx, "Unexpected GetAccountDiskUsage output, got %q", msg)
	return -1, errors.New("failed to parse GetAccountDiskUsage output")
}

// GetHomeUserPath retrieves the user home path, which contains a salted hash of the username.
func (u *CryptohomeClient) GetHomeUserPath(ctx context.Context, username string) (string, error) {
	binaryMsg, err := u.cryptohomePathBinary.userPath(ctx, username)
	msg := string(binaryMsg)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to call cryptohome-path user, got %q", msg)
		return "", errors.Wrap(err, "failed to call cryptohome-path user")
	}
	return strings.TrimSpace(msg), nil
}

// GetRootUserPath retrieves the user root path, which contains a salted hash of the username.
func (u *CryptohomeClient) GetRootUserPath(ctx context.Context, username string) (string, error) {
	binaryMsg, err := u.cryptohomePathBinary.systemPath(ctx, username)
	msg := string(binaryMsg)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to call cryptohome-path system, got %q", msg)
		return "", errors.Wrap(err, "failed to call cryptohome-path system")
	}
	return strings.TrimSpace(msg), nil
}

// GetUserHash returns user's cryptohome hash.
func (u *CryptohomeClient) GetUserHash(ctx context.Context, username string) (string, error) {
	binaryMsg, err := u.cryptohomePathBinary.userPath(ctx, username)
	msg := string(binaryMsg)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to call cryptohome-path user, got %q", msg)
		return "", errors.Wrap(err, "failed to call cryptohome-path user")
	}
	p := strings.TrimSpace(msg)
	m := userHashRegexp.FindStringSubmatch(p)
	if m == nil {
		return "", errors.Errorf("didn't find hash in path %q", p)
	}
	return m[1], nil
}

// GetUserShadowRoot returns the shadow root of the user.
func (u *CryptohomeClient) GetUserShadowRoot(ctx context.Context, username string) (string, error) {
	hash, err := u.GetUserHash(ctx, username)
	if err != nil {
		return "", errors.Wrap(err, "failed to retrieve the user hash")
	}
	return filepath.Join(shadowHome, hash), nil
}

// SupportsLECredentials calls GetSupportedKeyPolicies and parses the output for low entropy credential support.
func (u *CryptohomeClient) SupportsLECredentials(ctx context.Context) (bool, error) {
	binaryMsg, err := u.binary.getSupportedKeyPolicies(ctx)
	if err != nil {
		return false, errors.Wrap(err, "GetSupportedKeyPolicies failed")
	}

	// TODO(crbug.com/1187192): Parsing human readable output is error prone.
	// Parse the text output which looks something like:
	// low_entropy_credentials_supported: true
	// GetSupportedKeyPolicies success.
	return strings.Contains(string(binaryMsg), "low_entropy_credentials_supported: true"), nil
}

// GetKeyData returns the key data for the specified user and label.
func (u *CryptohomeClient) GetKeyData(ctx context.Context, user, keyLabel string) (string, error) {
	binaryMsg, err := u.binary.getKeyData(ctx, user, keyLabel)
	if err != nil {
		return "", errors.Wrap(err, "GetKeyData failed")
	}
	return string(binaryMsg), nil
}

// StartAuthSession starts an AuthSession for a given user.
func (u *CryptohomeClient) StartAuthSession(ctx context.Context, user string, isEphemeral bool, authIntent uda.AuthIntent) (string, error) {
	binaryMsg, err := u.binary.startAuthSession(ctx, user, isEphemeral, authIntent)
	if err != nil {
		return "", err
	}

	m := authSessionIDRegexp.FindSubmatch(binaryMsg)
	if m == nil {
		return "", errors.Errorf("didn't find auth session in output %q", string(binaryMsg))
	}
	return strings.TrimSpace(string(m[2])), nil
}

// StartAuthSessionAndCheckForPIN starts an AuthSession for a given user, checks for the presence of PIN AuthFactor in reply.
func (u *CryptohomeClient) StartAuthSessionAndCheckForPIN(ctx context.Context, user string, isEphemeral bool, authIntent uda.AuthIntent) (string, bool, error) {
	binaryMsg, err := u.binary.startAuthSession(ctx, user, isEphemeral, authIntent)
	if err != nil {
		return "", false, err
	}

	authSessionID := authSessionIDRegexp.FindSubmatch(binaryMsg)
	if authSessionID == nil {
		return "", false, errors.Errorf("didn't find auth session in output %q", string(binaryMsg))
	}

	reply := &uda.StartAuthSessionReply{}
	if err := proto.Unmarshal(binaryMsg, reply); err != nil {
		return strings.TrimSpace(string(authSessionID[2])), false, nil
	}

	// Search for PIN-based AuthFactor, if it is appears in StartAuthFactorReply
	for _, authFactor := range reply.AuthFactors {
		if authFactor.Type == uda.AuthFactorType_AUTH_FACTOR_TYPE_PIN {
			return strings.TrimSpace(string(authSessionID[2])), true, nil
		}
	}

	return strings.TrimSpace(string(authSessionID[2])), false, nil
}

// AuthenticateAuthSession authenticates an AuthSession with a given authSessionID.
// password is ignored if publicMount is set to true.
func (u *CryptohomeClient) AuthenticateAuthSession(ctx context.Context, password, keyLabel, authSessionID string, publicMount bool) error {
	_, err := u.binary.authenticateAuthSession(ctx, password, keyLabel, authSessionID, publicMount)
	return err
}

// AuthenticatePinWithAuthSession authenticates an AuthSession with a given authSessionID using a pin.
// password is ignored if publicMount is set to true.
func (u *CryptohomeClient) AuthenticatePinWithAuthSession(ctx context.Context, pin, label, authSessionID string) error {
	_, err := u.binary.authenticatePinWithAuthSession(ctx, pin, label, authSessionID)
	return err
}

// AuthenticateChallengeCredentialWithAuthSession authenticates an AuthSession with a given authSessionID,
// using a Challenge-Credential based backend dependent on flags provided through extraFlags.
func (u *CryptohomeClient) AuthenticateChallengeCredentialWithAuthSession(ctx context.Context, authSessionID, label string, authConfig *AuthConfig) error {
	extraFlags := authConfigToExtraFlags(authConfig)
	_, err := u.binary.authenticateChallengeCredentialWithAuthSession(ctx, authSessionID, label, extraFlags)
	return err
}

// UpdateCredentialWithAuthSession updated a credential using an AuthSession with a given authSessionID.
// password is ignored if publicMount is set to true.
func (u *CryptohomeClient) UpdateCredentialWithAuthSession(ctx context.Context, password, keyLabel, authSessionID string, publicMount bool) error {
	_, err := u.binary.updateCredentialWithAuthSession(ctx, password, keyLabel, authSessionID, publicMount)
	return err
}

// AuthenticateAuthFactor authenticates an AuthSession with a given authSessionID via an auth factor.
func (u *CryptohomeClient) AuthenticateAuthFactor(ctx context.Context, authSessionID, label, password string) (*uda.AuthenticateAuthFactorReply, error) {
	binaryMsg, err := u.binary.authenticateAuthFactor(ctx, authSessionID, label, password)
	if err != nil {
		return nil, errors.Wrap(err, "AuthenticateAuthFactor failed")
	}
	reply := &uda.AuthenticateAuthFactorReply{}
	if err := proto.Unmarshal(binaryMsg, reply); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal AuthenticateAuthFactor reply")
	}
	return reply, nil
}

// RemoveAuthFactor removes an auth factor with provided label.
func (u *CryptohomeClient) RemoveAuthFactor(ctx context.Context, authSessionID, label string) error {
	_, err := u.binary.removeAuthFactor(ctx, authSessionID, label)
	return err
}

// AuthenticatePinAuthFactor authenticates an AuthSession with a given authSessionID via pin.
func (u *CryptohomeClient) AuthenticatePinAuthFactor(ctx context.Context, authSessionID, label, pin string) error {
	_, err := u.binary.authenticatePinAuthFactor(ctx, authSessionID, label, pin)
	return err
}

// AuthenticateKioskAuthFactor authenticates an AuthSession with a given authSessionID via a kiosk authfactor.
func (u *CryptohomeClient) AuthenticateKioskAuthFactor(ctx context.Context, authSessionID string) error {
	_, err := u.binary.authenticateKioskAuthFactor(ctx, authSessionID)
	return err
}

// AuthenticateRecoveryAuthFactor authenticates an AuthSession with a given authSessionID via recovery auth factor.
func (u *CryptohomeClient) AuthenticateRecoveryAuthFactor(ctx context.Context, authSessionID, label, epochResponseHex, recoveryResponseHex string) error {
	_, err := u.binary.authenticateRecoveryAuthFactor(ctx, authSessionID, label, epochResponseHex, recoveryResponseHex)
	return err
}

// AuthenticateSmartCardAuthFactor authenticates an AuthSession with a given authSessionID via smart card.
func (u *CryptohomeClient) AuthenticateSmartCardAuthFactor(ctx context.Context, authSessionID, label string, authConfig *AuthConfig) error {
	extraFlags := authConfigToExtraFlags(authConfig)
	_, err := u.binary.authenticateSmartCardAuthFactor(ctx, authSessionID, label, extraFlags)
	return err
}

// AddCredentialsWithAuthSession creates the credentials for the user with given password.
// password is ignored if publicMount is set to true.
func (u *CryptohomeClient) AddCredentialsWithAuthSession(ctx context.Context, user, password, keyLabel, authSessionID string, publicMount bool) error {
	_, err := u.binary.addCredentialsWithAuthSession(ctx, user, password, keyLabel, authSessionID, publicMount)
	return err
}

// AddPinCredentialsWithAuthSession creates a pin credentials for the user with given pin.
func (u *CryptohomeClient) AddPinCredentialsWithAuthSession(ctx context.Context, label, pin, authSessionID string) error {
	_, err := u.binary.addPinCredentialsWithAuthSession(ctx, label, pin, authSessionID)
	return err
}

// AddChallengeCredentialsWithAuthSession creates the credentials for the user,
// using a Challenge-Credential based backend dependent on flags provided through extraFlags.
func (u *CryptohomeClient) AddChallengeCredentialsWithAuthSession(ctx context.Context, user, authSessionID, label string, authConfig *AuthConfig) error {
	extraFlags := authConfigToExtraFlags(authConfig)
	_, err := u.binary.addChallengeCredentialsWithAuthSession(ctx, user, authSessionID, label, extraFlags)
	return err
}

// AddAuthFactor creates an auth factor for the user with given password.
func (u *CryptohomeClient) AddAuthFactor(ctx context.Context, authSessionID, label, password string) error {
	_, err := u.binary.addAuthFactor(ctx, authSessionID, label, password)
	return err
}

// AddPinAuthFactor creates an auth factor for the user with given password.
func (u *CryptohomeClient) AddPinAuthFactor(ctx context.Context, authSessionID, label, pin string) error {
	_, err := u.binary.addPinAuthFactor(ctx, authSessionID, label, pin)
	return err
}

// AddRecoveryAuthFactor creates a recovery auth factor for the user.
func (u *CryptohomeClient) AddRecoveryAuthFactor(ctx context.Context, authSessionID, label, mediatorPubKeyHex string) error {
	_, err := u.binary.addRecoveryAuthFactor(ctx, authSessionID, label, mediatorPubKeyHex)
	return err
}

// AddKioskAuthFactor creates an auth factor for kiosk user.
func (u *CryptohomeClient) AddKioskAuthFactor(ctx context.Context, authSessionID string) error {
	_, err := u.binary.addKioskAuthFactor(ctx, authSessionID)
	return err
}

// AddSmartCardAuthFactor creates an auth factor for the user with given smart card.
func (u *CryptohomeClient) AddSmartCardAuthFactor(ctx context.Context, authSessionID, label string, authConfig *AuthConfig) error {
	extraFlags := authConfigToExtraFlags(authConfig)
	_, err := u.binary.addSmartCardAuthFactor(ctx, authSessionID, label, extraFlags)
	return err
}

// UpdatePasswordAuthFactor creates an auth factor for the user with given password.
func (u *CryptohomeClient) UpdatePasswordAuthFactor(ctx context.Context, authSessionID, label, newKeyLabel, password string) error {
	_, err := u.binary.updatePasswordAuthFactor(ctx, authSessionID, label, newKeyLabel, password)
	return err
}

// UpdateRecoveryAuthFactor updates the recovery auth factor for the user.
func (u *CryptohomeClient) UpdateRecoveryAuthFactor(ctx context.Context, authSessionID, label, mediatorPubKeyHex string) error {
	_, err := u.binary.updateRecoveryAuthFactor(ctx, authSessionID, label, mediatorPubKeyHex)
	return err
}

// UpdatePinAuthFactor updates the pin auth factor for the user.
func (u *CryptohomeClient) UpdatePinAuthFactor(ctx context.Context, authSessionID, label, pin string) error {
	_, err := u.binary.updatePinAuthFactor(ctx, authSessionID, label, pin)
	return err
}

// PrepareGuestVault prepares vault for guest session.
func (u *CryptohomeClient) PrepareGuestVault(ctx context.Context) error {
	_, err := u.binary.prepareGuestVault(ctx)
	return err
}

// PrepareEphemeralVault prepares vault for ephemeral session.
func (u *CryptohomeClient) PrepareEphemeralVault(ctx context.Context, authSessionID string) error {
	_, err := u.binary.prepareEphemeralVault(ctx, authSessionID)
	return err
}

// PreparePersistentVault prepares vault for persistent user session.
func (u *CryptohomeClient) PreparePersistentVault(ctx context.Context, authSessionID string, ecryptfs bool) error {
	_, err := u.binary.preparePersistentVault(ctx, authSessionID, ecryptfs)
	return err
}

// PrepareVaultForMigration prepares vault for migration.
func (u *CryptohomeClient) PrepareVaultForMigration(ctx context.Context, authSessionID string) error {
	_, err := u.binary.prepareVaultForMigration(ctx, authSessionID)
	return err
}

// CreatePersistentUser creates persistent user.
func (u *CryptohomeClient) CreatePersistentUser(ctx context.Context, authSessionID string) error {
	_, err := u.binary.createPersistentUser(ctx, authSessionID)
	return err
}

// MigrateToDircrypto migrates vault to dircrypto. Must be mounted for migration first.
func (u *CryptohomeClient) MigrateToDircrypto(ctx context.Context, userName string) error {
	_, err := u.binary.migrateToDircrypto(ctx, userName)
	return err
}

// MountWithAuthSession mounts a user with AuthSessionID.
func (u *CryptohomeClient) MountWithAuthSession(ctx context.Context, authSessionID string, publicMount bool) error {
	_, err := u.binary.mountWithAuthSession(ctx, authSessionID, publicMount)
	return err
}

// InvalidateAuthSession invalidates a user with AuthSessionID.
func (u *CryptohomeClient) InvalidateAuthSession(ctx context.Context, authSessionID string) error {
	_, err := u.binary.invalidateAuthSession(ctx, authSessionID)
	return err
}

// FetchRecoveryRequest creates recovery request, returns the value of the request.
func (u *CryptohomeClient) FetchRecoveryRequest(ctx context.Context, authSessionID, label, epochResponseHex string) (string, error) {
	response, err := u.binary.fetchRecoveryRequest(ctx, authSessionID, label, epochResponseHex)
	if err != nil {
		return "", errors.Wrap(err, "FetchRecoveryRequest failed")
	}

	m := recoveryRequestRegexp.FindSubmatch(response)
	if m == nil {
		return "", errors.Errorf("didn't find recovery request in output %q", string(response))
	}
	return strings.TrimSpace(string(m[1])), nil
}

// ListAuthFactors lists the auth factors for a given user.
func (u *CryptohomeClient) ListAuthFactors(ctx context.Context, user string) (*uda.ListAuthFactorsReply, error) {
	reply := &uda.ListAuthFactorsReply{}

	binaryMsg, err := u.binary.listAuthFactors(ctx, user)
	if err != nil {
		return reply, errors.Wrap(err, "ListAuthFactors failed")
	}
	if err := proto.Unmarshal(binaryMsg, reply); err != nil {
		return reply, errors.Wrap(err, "failed to unmarshal ListAuthFactors reply")
	}

	return reply, nil
}
