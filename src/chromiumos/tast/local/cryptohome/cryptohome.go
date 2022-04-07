// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cryptohome operates on encrypted home directories.
package cryptohome

// TODO(b/182152667): We should deprecate the usage of this file.
// Please considering use hwsec.CryptohomeClient directly for new consumer.

import (
	"context"
	"os"
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
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	path, err := cryptohome.GetHomeUserPath(ctx, user)
	if err != nil {
		return "", errors.Wrap(err, "failed to get user home path")
	}
	return path, nil
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

	if err := cryptohome.MountVault(ctx, "bar", hwsec.NewPassAuthConfig(user, password), true, hwsec.NewVaultConfig()); err != nil {
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

// AuthSessionMountFlow mounts a user with AuthSession.
func AuthSessionMountFlow(ctx context.Context, isKioskUser bool, username, password string, createUser bool) error {
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
		if err := cryptohome.AddCredentialsWithAuthSession(ctx, username, password, authSessionID, isKioskUser); err != nil {
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
	if err := cryptohome.AuthenticateAuthSession(ctx, password, authSessionID, isKioskUser); err != nil {
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
func CreateUserWithAuthSession(ctx context.Context, username, password string, isKioskUser bool) error {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, username /*ephemeral=*/, false)
	if err != nil {
		return errors.Wrap(err, "failed to start Auth session")
	}
	// defer cryptohome.InvalidateAuthSession(ctx, auth_session_id)
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	if err := cryptohome.AddCredentialsWithAuthSession(ctx, username, password, authSessionID, isKioskUser); err != nil {
		return errors.Wrap(err, "failed to add credentials with AuthSession")
	}
	testing.ContextLog(ctx, "Added credentials successfully")
	if err := cryptohome.AuthenticateAuthSession(ctx, password, authSessionID, isKioskUser); err != nil {
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

// CreateAndMountUserWithAuthSession creates a persistent user via auth session API.
func CreateAndMountUserWithAuthSession(ctx context.Context, username, password string, isKioskUser bool) error {
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

	if err := cryptohome.AddCredentialsWithAuthSession(ctx, username, password, authSessionID, isKioskUser); err != nil {
		return errors.Wrap(err, "failed to add credentials with AuthSession")
	}

	return nil
}

// AuthenticateWithAuthSession authenticates an existing user via auth session API.
func AuthenticateWithAuthSession(ctx context.Context, username, password string, isEphemeral, isKioskUser bool) (string, error) {
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
	if err := cryptohome.AuthenticateAuthSession(ctx, password, authSessionID, isKioskUser); err != nil {
		return "", errors.Wrap(err, "failed to authenticate with AuthSession")
	}
	testing.ContextLog(ctx, "User authenticated successfully")

	return authSessionID, nil
}

// UpdateUserCredentialWithAuthSession authenticates an existing user via auth session API.
func UpdateUserCredentialWithAuthSession(ctx context.Context, username, oldPassword, newPassword string, isEphemeral, isKioskUser bool) (string, error) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, username, isEphemeral)
	if err != nil {
		return "", errors.Wrap(err, "failed to start Auth session")
	}

	// Authenticate the same AuthSession using authSessionID.
	// If we cannot authenticate, do not proceed with mount and unmount.
	if err := cryptohome.AuthenticateAuthSession(ctx, oldPassword, authSessionID, isKioskUser); err != nil {
		return "", errors.Wrap(err, "failed to authenticate with AuthSession")
	}

	// UpdateCredential with the same AuthSession using authSessionID.
	if err := cryptohome.UpdateCredentialWithAuthSession(ctx, newPassword, authSessionID, isKioskUser); err != nil {
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
