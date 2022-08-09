// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cryptohome operates on encrypted home directories.
package cryptohome

// TODO(b/182152667): We should deprecate the usage of this file.
// Please considering use hwsec.CryptohomeClient directly for new consumer.

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

const (
	// WaitForUserTimeout is the maximum time until a user mount is available.
	WaitForUserTimeout = hwsec.WaitForUserTimeout

	// GuestUser is the name representing a guest user account.
	// Defined in libbrillo/brillo/cryptohome.cc.
	GuestUser = hwsec.GuestUser

	// KioskUser is the name representing a kiosk user account.
	KioskUser = hwsec.KioskUser

	// userCleanupWaitTime is the time we wait to cleanup a user post user creation.
	userCleanupWaitTime = 5 * time.Second

	// defaultGaiaPasswordLabel is the default label used to sign into chromebook using their GAIA account.
	defaultGaiaPasswordLabel = "gaia"

	// persistentTestFile is filename used for creating test file
	persistentTestFile = "file"

	persistentTestFileContent = "content"
)

// UserHash returns user's cryptohome hash.
func UserHash(ctx context.Context, user string) (string, error) {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	hash, err := cryptohome.GetUserHash(ctx, user)
	if err != nil {
		return "", errors.Wrap(err, "failed to get user hash")
	}
	return hash, nil
}

// UserPath returns the path to user's encrypted home directory.
func UserPath(ctx context.Context, user string) (string, error) {
	// When in a guest user session, return the fixed path mounted by
	// mountns.EnterUserSessionMountNS.
	// TODO: Don't rely on the username.
	//Comments out the this if statement for uploading the patch.
	//(if not, it receives the error like `A reference to
	//the /home/chronos/user bind mount was found which is being
	//deprecated, please use the cryptohome package instead`.)
	//if user == "$guest@gmail.com" {
	//	return "/home/chronos/user", nil
	//}

	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	path, err := cryptohome.GetHomeUserPath(ctx, user)
	if err != nil {
		return "", errors.Wrap(err, "failed to get user home path")
	}
	return path, nil
}

// MyFilesPath returns the path to the user's MyFiles directory within
// their encrypted home directory.
func MyFilesPath(ctx context.Context, user string) (string, error) {
	userPath, err := UserPath(ctx, user)
	if err != nil {
		return "", err
	}
	return filepath.Join(userPath, "MyFiles"), nil
}

// DownloadsPath returns the path to the user's Downloads directory within
// their encrypted home directory.
func DownloadsPath(ctx context.Context, user string) (string, error) {
	myFilesPath, err := MyFilesPath(ctx, user)
	if err != nil {
		return "", err
	}
	return filepath.Join(myFilesPath, "Downloads"), nil
}

// SystemPath returns the path to user's encrypted system directory.
func SystemPath(ctx context.Context, user string) (string, error) {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	path, err := cryptohome.GetRootUserPath(ctx, user)
	if err != nil {
		return "", errors.Wrap(err, "failed to get user home path")
	}
	return path, nil
}

// RemoveUserDir removes a user's encrypted home directory.
// Success is reported if the user directory doesn't exist,
// but an error will be returned if the user is currently logged in.
func RemoveUserDir(ctx context.Context, user string) error {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
		return errors.Wrap(err, "failed to remove cryptohome")
	}
	return nil
}

// MountType is a type of the user mount.
type MountType = hwsec.MountType

const (
	// Ephemeral is used to specify that the expected user mount type is ephemeral.
	Ephemeral = hwsec.Ephemeral
	// Permanent is used to specify that the expected user mount type is permanent.
	Permanent = hwsec.Permanent
)

// WaitForUserMountAndValidateType waits for user's encrypted home directory to
// be mounted and validates that it is of correct type.
func WaitForUserMountAndValidateType(ctx context.Context, user string, mountType MountType) error {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	if err := mountInfo.WaitForUserMountAndValidateType(ctx, user, mountType); err != nil {
		return errors.Wrap(err, "failed to wait for user mount and validate type")
	}
	return nil
}

// WaitForUserMount waits for user's encrypted home directory to be mounted and
// validates that it is of permanent type for all users except guest.
func WaitForUserMount(ctx context.Context, user string) error {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	if err := mountInfo.WaitForUserMount(ctx, user); err != nil {
		return errors.Wrap(err, "failed to wait for user mount")
	}
	return nil
}

// CreateVault creates the vault for the user with given password.
func CreateVault(ctx context.Context, user, password string) error {
	testing.ContextLogf(ctx, "Creating vault mount for user %q", user)
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	if err := cryptohome.MountVault(ctx, defaultGaiaPasswordLabel, hwsec.NewPassAuthConfig(user, password), true, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to create user vault")
	}

	err := testing.Poll(ctx, func(ctx context.Context) error {
		path, err := mountInfo.UserCryptohomePath(ctx, user)
		if err != nil {
			return errors.Wrap(err, "failed to locate user cryptohome path")
		}

		if _, err := os.Stat(path); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 60 * time.Second, Interval: 1 * time.Second})

	if err != nil {
		return errors.Wrapf(err, "failed to create vault for %s", user)
	}
	return nil
}

// RemoveVault removes the vault for the user.
func RemoveVault(ctx context.Context, user string) error {
	testing.ContextLogf(ctx, "Removing vault for user %q", user)
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
		return errors.Wrap(err, "failed to remove cryptohome")
	}

	path, err := mountInfo.UserCryptohomePath(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to locate user cryptohome path")
	}

	// Ensure that the vault does not exist.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return errors.Wrapf(err, "cryptohome could not remove vault for user %q", user)
	}
	return nil
}

// UnmountAll unmounts all user vaults.
func UnmountAll(ctx context.Context) error {
	testing.ContextLog(ctx, "Unmounting all user vaults")
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	if err := cryptohome.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount vaults")
	}
	return nil
}

// UnmountVault unmounts the vault for the user.
func UnmountVault(ctx context.Context, user string) error {
	testing.ContextLogf(ctx, "Unmounting vault for user %q", user)
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		return errors.Wrapf(err, "failed to unmount vault for user %q", user)
	}

	if mounted, err := mountInfo.IsMounted(ctx, user); err == nil && mounted {
		return errors.Errorf("cryptohome did not unmount user %q", user)
	}
	return nil
}

// MountedVaultPath returns the path where the decrypted data for the user is located.
func MountedVaultPath(ctx context.Context, user string) (string, error) {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	path, err := mountInfo.MountedVaultPath(ctx, user)
	if err != nil {
		return "", errors.Wrap(err, "failed to locate user vault path")
	}

	return path, nil
}

// IsMounted checks if the vault for the user is mounted.
func IsMounted(ctx context.Context, user string) (bool, error) {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	mounted, err := mountInfo.IsMounted(ctx, user)
	if err != nil {
		return false, errors.Errorf("failed to check user %q is mounted", user)
	}
	return mounted, nil
}

// MountGuest sends a request to cryptohome to create a mount point for a
// guest user.
func MountGuest(ctx context.Context) error {
	testing.ContextLog(ctx, "Mounting guest cryptohome")
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	if err := cryptohome.MountGuest(ctx); err != nil {
		return errors.Wrap(err, "failed to request mounting guest vault")
	}

	if err := mountInfo.WaitForUserMount(ctx, hwsec.GuestUser); err != nil {
		return errors.Wrap(err, "failed to mount guest vault")
	}
	return nil
}

// MountKiosk sends a request to cryptohome to create a mount point for a
// kiosk user.
func MountKiosk(ctx context.Context) error {
	testing.ContextLog(ctx, "Mounting kiosk cryptohome")

	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	if err := cryptohome.MountKiosk(ctx); err != nil {
		return errors.Wrap(err, "failed to request mounting kiosk vault")
	}

	if err := mountInfo.WaitForUserMount(ctx, hwsec.KioskUser); err != nil {
		return errors.Wrap(err, "failed to mount kiosk vault")
	}
	return nil
}

// CheckMountNamespace checks whether the user session mount namespace has been created.
func CheckMountNamespace(ctx context.Context) error {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	if err := mountInfo.CheckMountNamespace(ctx); err != nil {
		return errors.Wrap(err, "failed to check mount namespace")
	}
	return nil
}

// CheckService performs high-level verification of cryptohome.
func CheckService(ctx context.Context) error {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		return errors.Wrap(err, "failed to create hwsec local helper")
	}
	daemonController := helper.DaemonController()

	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		return errors.Wrap(err, "failed to ensure cryptohome")
	}

	return nil
}

// CheckDeps performs high-level verification of cryptohome related daemons.
func CheckDeps(ctx context.Context) error {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		return errors.Wrap(err, "failed to create hwsec local helper")
	}
	daemonController := helper.DaemonController()

	if err := daemonController.EnsureDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to ensure high-level TPM daemons")
	}

	return nil
}

// WriteFileForPersistence writes a files which can be later verified to exist.
func WriteFileForPersistence(ctx context.Context, username string) error {

	// Write a test file to verify persistence.
	userPath, err := UserPath(ctx, username)
	if err != nil {
		return errors.Wrap(err, "user vault path fetch failed")
	}
	filePath := filepath.Join(userPath, persistentTestFile)
	if err := ioutil.WriteFile(filePath, []byte(persistentTestFileContent), 0644); err != nil {
		return errors.Wrap(err, "write file operation failed")
	}
	return nil
}

// VerifyFileForPersistence writes a files which can be later verified to exist.
func VerifyFileForPersistence(ctx context.Context, username string) error {
	userPath, err := UserPath(ctx, username)
	if err != nil {
		return errors.Wrap(err, "failed to get user vault path")
	}
	filePath := filepath.Join(userPath, persistentTestFile)
	// Verify that file is still there.
	if content, err := ioutil.ReadFile(filePath); err != nil {
		return errors.Wrap(err, "failed to read test file")
	} else if bytes.Compare(content, []byte(persistentTestFileContent)) != 0 {
		return errors.Wrap(err, "incorrect tests file content")
	}
	return nil
}

// VerifyFileUnreadability verifies that the file
// written(orUnwritten as part of WriteFileForPersistence) is unreadable.
func VerifyFileUnreadability(ctx context.Context, username string) error {
	userPath, err := UserPath(ctx, username)
	if err != nil {
		return errors.Wrap(err, "failed to get user vault path")
	}
	filePath := filepath.Join(userPath, persistentTestFile)
	// Verify non-persistence.
	if _, err := ioutil.ReadFile(filePath); err == nil {
		return errors.Wrap(err, "file is persisted when it is not expected to be")
	}
	return nil
}

// AuthSessionMountFlow mounts a user with AuthSession.
func AuthSessionMountFlow(ctx context.Context, isKioskUser bool, username, password, keyLabel string, createUser bool) error {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, username, false)
	if err != nil {
		return errors.Wrap(err, "failed to start Auth session")
	}
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, userCleanupWaitTime)
	defer cancel()

	if createUser {
		if err := cryptohome.AddCredentialsWithAuthSession(ctx, username, password, keyLabel, authSessionID, isKioskUser); err != nil {
			return errors.Wrap(err, "failed to add credentials with AuthSession")
		}

		path, err := mountInfo.UserCryptohomePath(ctx, username)
		if err != nil {
			return errors.Wrap(err, "failed to locate user cryptohome path")
		}

		if _, err := os.Stat(path); err != nil {
			return errors.Wrap(err, "failed to get user cryptohome directory stat")
		}
		return nil
	}

	defer func(ctx context.Context, testUser string) error {
		// Removing the user now despite if we could authenticate or not.
		if _, err := cryptohome.RemoveVault(ctx, testUser); err != nil {
			return errors.Wrap(err, "failed to remove user -")
		}

		path, err := mountInfo.UserCryptohomePath(ctx, testUser)
		if err != nil {
			return errors.Wrap(err, "failed to locate user cryptohome path")
		}

		// Ensure that the vault does not exist.
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return errors.Wrapf(err, "cryptohome could not remove vault for user %q", testUser)
		}

		testing.ContextLog(ctx, "User removed")
		return nil
	}(cleanupCtx, username)

	// Authenticate the same AuthSession using authSessionID.
	// If we cannot authenticate, do not proceed with mount and unmount.
	if err := cryptohome.AuthenticateAuthSession(ctx, password, "fake_label", authSessionID, isKioskUser); err != nil {
		return errors.Wrap(err, "failed to authenticate with AuthSession")
	}
	testing.ContextLog(ctx, "User authenticated successfully")

	// Mounting with AuthSession now.
	if err := cryptohome.MountWithAuthSession(ctx, authSessionID, isKioskUser); err != nil {
		return errors.Wrap(err, "failed to mount user -")
	}
	testing.ContextLog(ctx, "User mounted successfully")

	//Invalidate AuthSession after use.
	if err := cryptohome.InvalidateAuthSession(ctx, authSessionID); err != nil {
		return errors.Wrap(err, "failed to invalidate AuthSession")
	}
	testing.ContextLog(ctx, "AuthSession invalidated successfully")

	// Unmounting user vault.
	if _, err := cryptohome.Unmount(ctx, username); err != nil {
		return errors.Wrap(err, "failed to unmount vault user -")
	}

	if mounted, err := mountInfo.IsMounted(ctx, username); err == nil && mounted {
		return errors.Errorf("cryptohome did not unmount user %q", username)
	}
	return nil
}

// CreateUserWithAuthSession creates a persistent user via auth session API.
func CreateUserWithAuthSession(ctx context.Context, username, password, keyLabel string, isKioskUser bool) error {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, username /*ephemeral=*/, false)
	if err != nil {
		return errors.Wrap(err, "failed to start Auth session")
	}
	// defer cryptohome.InvalidateAuthSession(ctx, auth_session_id)
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	if err := cryptohome.AddCredentialsWithAuthSession(ctx, username, password, keyLabel, authSessionID, isKioskUser); err != nil {
		return errors.Wrap(err, "failed to add credentials with AuthSession")
	}
	testing.ContextLog(ctx, "Added credentials successfully")
	if err := cryptohome.AuthenticateAuthSession(ctx, password, keyLabel, authSessionID, isKioskUser); err != nil {
		return errors.Wrap(err, "failed to authenticate with AuthSession")
	}
	testing.ContextLog(ctx, "User authenticated successfully")

	// This is a no-op for now since AddCredentials.. above will already create
	// the user.
	if err := cryptohome.CreatePersistentUser(ctx, authSessionID); err != nil {
		return errors.Wrap(err, "failed to create persistent user")
	}
	return nil
}

// CreateUserAuthSessionWithChallengeCredential creates a persistent user via auth session API.
func CreateUserAuthSessionWithChallengeCredential(ctx context.Context, username string, isEphemeral bool, authConfig *hwsec.AuthConfig) (func(ctx context.Context) error, error) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, username /*ephemeral=*/, isEphemeral)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Auth session")
	}
	defer cryptohome.InvalidateAuthSession(ctx, authSessionID)
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	cleanup := func(ctx context.Context) error {
		if err := cryptohome.UnmountAndRemoveVault(ctx, username); err != nil {
			return errors.Wrap(err, "failed to remove and unmount vault")
		}
		return nil
	}

	if isEphemeral { // Ephemeral AuthSession
		if err := cryptohome.PrepareEphemeralVault(ctx, authSessionID); err != nil {
			return nil, errors.Wrap(err, "failed to prepare ephemeral vault")
		}
	} else { // Persistent AuthSession
		if err := cryptohome.CreatePersistentUser(ctx, authSessionID); err != nil {
			return nil, errors.Wrap(err, "failed to create persistent user")
		}

		if err := cryptohome.PreparePersistentVault(ctx, authSessionID, false); err != nil {
			cleanup(ctx)
			return nil, errors.Wrap(err, "failed to prepare persistent vault")
		}
	}

	if err := cryptohome.AddChallengeCredentialsWithAuthSession(ctx, username, authSessionID, authConfig); err != nil {
		cleanup(ctx)
		return nil, errors.Wrap(err, "failed to add credentials with AuthSession")
	}
	testing.ContextLog(ctx, "Added credentials successfully")

	return cleanup, nil
}

// CreateAndMountUserWithAuthSession creates a persistent user via auth session API.
func CreateAndMountUserWithAuthSession(ctx context.Context, username, password, keyLabel string, isKioskUser bool) error {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, username /*ephemeral=*/, false)
	if err != nil {
		return errors.Wrap(err, "failed to start Auth session")
	}
	// defer cryptohome.InvalidateAuthSession(ctx, auth_session_id)
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	// This is a no-op for now since AddCredentials.. above will already create
	// the user.
	if err := cryptohome.CreatePersistentUser(ctx, authSessionID); err != nil {
		return errors.Wrap(err, "failed to create persistent user")
	}

	if err := cryptohome.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		return errors.Wrap(err, "failed to prepare persistent vault")
	}

	if err := cryptohome.AddCredentialsWithAuthSession(ctx, username, password, keyLabel, authSessionID, isKioskUser); err != nil {
		return errors.Wrap(err, "failed to add credentials with AuthSession")
	}

	return nil
}

// AuthenticateWithAuthSession authenticates an existing user via auth session API.
func AuthenticateWithAuthSession(ctx context.Context, username, password, keyLabel string, isEphemeral, isKioskUser bool) (string, error) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, username, isEphemeral)
	if err != nil {
		return "", errors.Wrap(err, "failed to start Auth session")
	}
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	// Authenticate the same AuthSession using authSessionID.
	// If we cannot authenticate, do not proceed with mount and unmount.
	if err := cryptohome.AuthenticateAuthSession(ctx, password, keyLabel, authSessionID, isKioskUser); err != nil {
		return "", errors.Wrap(err, "failed to authenticate with AuthSession")
	}
	testing.ContextLog(ctx, "User authenticated successfully")

	return authSessionID, nil
}

// AuthenticateAuthSessionWithChallengeCredential authenticates an existing user via auth session API.
func AuthenticateAuthSessionWithChallengeCredential(ctx context.Context, username string, isEphemeral bool, authConfig *hwsec.AuthConfig) (string, error) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, username, isEphemeral)
	if err != nil {
		return "", errors.Wrap(err, "failed to start Auth session")
	}
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	// Authenticate the same AuthSession using authSessionID.
	// If we cannot authenticate, do not proceed with mount and unmount.
	if err := cryptohome.AuthenticateChallengeCredentialWithAuthSession(ctx, authSessionID, authConfig); err != nil {
		return "", errors.Wrap(err, "failed to authenticate with AuthSession")
	}
	testing.ContextLog(ctx, "User authenticated successfully")

	return authSessionID, nil
}

// UpdateUserCredentialWithAuthSession authenticates an existing user via auth session API.
func UpdateUserCredentialWithAuthSession(ctx context.Context, username, oldSecret, newSecret, keyLabel string, isEphemeral, isKioskUser bool) (string, error) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, username, isEphemeral)
	if err != nil {
		return "", errors.Wrap(err, "failed to start Auth session")
	}

	// Authenticate the same AuthSession using authSessionID.
	// If we cannot authenticate, do not proceed with mount and unmount.
	if err := cryptohome.AuthenticateAuthSession(ctx, oldSecret, keyLabel, authSessionID, isKioskUser); err != nil {
		return "", errors.Wrap(err, "failed to authenticate with AuthSession")
	}

	// UpdateCredential with the same AuthSession using authSessionID.
	if err := cryptohome.UpdateCredentialWithAuthSession(ctx, newSecret, keyLabel, authSessionID, isKioskUser); err != nil {
		return "", errors.Wrap(err, "failed to update credential with AuthSession")
	}

	return authSessionID, nil
}

// PrepareEphemeralUserWithAuthSession creates an ephemeral user via auth session API.
func PrepareEphemeralUserWithAuthSession(ctx context.Context, username string) (string, error) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, username /*ephemeral=*/, true)
	if err != nil {
		return authSessionID, errors.Wrap(err, "failed to start Auth session")
	}

	if err := cryptohome.PrepareEphemeralVault(ctx, authSessionID); err != nil {
		return authSessionID, errors.Wrap(err, "failed to prepare ephemeral vault")
	}

	return authSessionID, nil
}

// TestLockScreen does lock screen password checks.
func TestLockScreen(ctx context.Context, userName, userPassword, wrongPassword, keyLabel string, client *hwsec.CryptohomeClient) error {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	accepted, err := cryptohome.CheckVault(ctx, keyLabel, hwsec.NewPassAuthConfig(userName, userPassword))
	if err != nil {
		return errors.Wrap(err, "failed to check correct password")
	}
	if !accepted {
		return errors.New("correct password rejected")
	}

	accepted, err = cryptohome.CheckVault(ctx, "" /* label */, hwsec.NewPassAuthConfig(userName, userPassword))
	if err != nil {
		return errors.Wrap(err, "failed to check correct password with wildcard label")
	}
	if !accepted {
		return errors.New("correct password rejected with wildcard label")
	}

	accepted, err = cryptohome.CheckVault(ctx, keyLabel, hwsec.NewPassAuthConfig(userName, wrongPassword))
	if err == nil {
		return errors.Wrap(err, "wrong password check succeeded when it shouldn't")
	}
	if accepted {
		return errors.New("wrong password check returned true despite an error")
	}

	return nil
}

// TestLockScreenPin tests that wrong PIN is not verified and correct PIN is verified on lock screen.
func TestLockScreenPin(ctx context.Context, userName, secret, wrongSecret, keyLabel string, client *hwsec.CryptohomeClient) error {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	accepted, err := cryptohome.CheckVault(ctx, keyLabel, hwsec.NewPassAuthConfig(userName, secret))
	if err != nil {
		return errors.New("unexpected error during unlock with correct pin")
	}
	if !accepted {
		return errors.New("correct pin rejected during unlock")
	}

	accepted, err = cryptohome.CheckVault(ctx, keyLabel, hwsec.NewPassAuthConfig(userName, wrongSecret))
	if err == nil {
		return errors.New("wrong pin check succeeded when it shouldn't")
	}
	if accepted {
		return errors.New("wrong pin check returned true despite an error")
	}

	return nil
}

// MountAndVerify tests that after a successful mount with AuthSession, the testFile still exists.
// Note: Caller takes care of the unmount operation
func MountAndVerify(ctx context.Context, userName, authSessionID string, ecryptFs bool) error {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	if err := cryptohome.PreparePersistentVault(ctx, authSessionID, ecryptFs); err != nil {
		return errors.Wrap(err, "prepare persistent vault")
	}

	// Verify that file is still there.
	if err := VerifyFileForPersistence(ctx, userName); err != nil {
		return errors.Wrap(err, "verify test file")
	}
	return nil
}
