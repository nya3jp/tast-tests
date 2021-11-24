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
	"regexp"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
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
	KioskUser = hwsec.GuestUser

	// userCleanupWaitTime is the time we wait to cleanup a user post user creation.
	userCleanupWaitTime = 5 * time.Second
)

var (
	// authSessionIDRegexp matches the auth session ID.
	// It would matche "auth_session_id:*"
	authSessionIDRegexp = regexp.MustCompile(`(auth_session_id:)(.+)(\n)`)
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
	path, err := cryptohome.GetHomeUserPath(ctx, user)
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

// StartAuthSession starts an AuthSession for a given user.
func StartAuthSession(ctx context.Context, user string) (string, error) {
	testing.ContextLogf(ctx, "Creating AuthSession for user %q", user)
	cmd := testexec.CommandContext(
		ctx, "cryptohome", "--action=start_auth_session",
		"--user="+user)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create AuthSession for %q", user)
	}
	authSessionID := authSessionIDRegexp.FindSubmatch(out)[2]
	return string(authSessionID), nil
}

// AuthenticateAuthSession authenticates an AuthSession with a given authSessionID.
// password is ignored if publicMount is set to true.
func AuthenticateAuthSession(ctx context.Context, password, authSessionID string, publicMount bool) error {
	testing.ContextLog(ctx, "Authenticating AuthSession")
	cmd := testexec.CommandContext(
		ctx, "cryptohome", "--action=authenticate_auth_session",
		"--auth_session_id="+authSessionID)
	if publicMount {
		cmd.Args = append(cmd.Args, "--public_mount")
	} else {
		cmd.Args = append(cmd.Args, "--password="+password)
	}
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to authenticate AuthSession")
	}
	return nil
}

// AddCredentialsWithAuthSession creates the credentials for the user with given password.
// password is ignored if publicMount is set to true.
func AddCredentialsWithAuthSession(ctx context.Context, user, password, authSessionID string, publicMount bool) error {
	testing.ContextLogf(ctx, "Creating new credentials for with AuthSession id: %q", authSessionID)

	cmd := testexec.CommandContext(
		ctx, "cryptohome", "--action=add_credentials",
		"--auth_session_id="+authSessionID)
	if publicMount {
		cmd.Args = append(cmd.Args, "--public_mount", "--key_label=public_mount")
	} else {
		cmd.Args = append(cmd.Args, "--password="+password, "--key_label=fake_label")
	}
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to create new credentials for %s", user)
	}

	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	path, err := mountInfo.UserCryptohomePath(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to locate user crypthome path")
	}

	if _, err := os.Stat(path); err != nil {
		return errors.Wrap(err, "failed to get user cryptohome directory")
	}
	return nil
}

// MountWithAuthSession mounts a user with AuthSessionID.
func MountWithAuthSession(ctx context.Context, authSessionID string, publicMount bool) error {
	testing.ContextLogf(ctx, "Trying to mount user vault with AuthSession id: %q", authSessionID)
	cmd := testexec.CommandContext(
		ctx, "cryptohome", "--action=mount_ex",
		"--auth_session_id="+authSessionID)
	if publicMount {
		cmd.Args = append(cmd.Args, "--public_mount")
	}
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to mount vault")
	}
	return nil
}

// InvalidateAuthSession invalidates a user with AuthSessionID.
func InvalidateAuthSession(ctx context.Context, authSessionID string) error {
	testing.ContextLogf(ctx, "Trying to invalidate AuthSession with id: %q", authSessionID)
	cmd := testexec.CommandContext(
		ctx, "cryptohome", "--action=invalidate_auth_session",
		"--auth_session_id="+authSessionID)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to invalidate AuthSession")
	}
	return nil
}

// AuthSessionMountFlow mounts a user with AuthSession.
func AuthSessionMountFlow(ctx context.Context, isKioskUser bool, username, password string, createUser bool) error {
	// Start an Auth session and get an authSessionID.
	authSessionID, err := StartAuthSession(ctx, username)
	if err != nil {
		return errors.Wrap(err, "failed to start Auth session")
	}
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, userCleanupWaitTime)
	defer cancel()

	if createUser {
		if err := AddCredentialsWithAuthSession(ctx, username, password, authSessionID, isKioskUser); err != nil {
			return errors.Wrap(err, "failed to add credentials with AuthSession")
		}
	}

	defer func(ctx context.Context, testUser string) error {
		// Removing the user now despite if we could authenticate or not.
		if err := RemoveVault(ctx, testUser); err != nil {
			return errors.Wrap(err, "failed to remove user -")
		}
		testing.ContextLog(ctx, "User removed")
		return nil
	}(cleanupCtx, username)

	// Authenticate the same AuthSession using authSessionID.
	// If we cannot authenticate, do not proceed with mount and unmount.
	if err := AuthenticateAuthSession(ctx, password, authSessionID, isKioskUser); err != nil {
		return errors.Wrap(err, "failed to authenticate with AuthSession")
	}
	testing.ContextLog(ctx, "User authenticated successfully")

	// Mounting with AuthSession now.
	if err := MountWithAuthSession(ctx, authSessionID, isKioskUser); err != nil {
		return errors.Wrap(err, "failed to mount user -")
	}
	testing.ContextLog(ctx, "User mounted successfully")

	//Invalidate AuthSession after use.
	if err := InvalidateAuthSession(ctx, authSessionID); err != nil {
		return errors.Wrap(err, "failed to invalidate AuthSession")
	}
	testing.ContextLog(ctx, "AuthSession invalidated successfully")

	// Unmounting user vault.
	if err := UnmountVault(ctx, username); err != nil {
		return errors.Wrap(err, "failed to unmount vault user -")
	}
	return nil
}
