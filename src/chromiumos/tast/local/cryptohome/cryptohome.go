// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cryptohome operates on encrypted home directories.
package cryptohome

// TODO(b/182152667): We should deprecate the usage of this file

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// WaitForUserTimeout is the maximum time until a user mount is available.
	WaitForUserTimeout = 80 * time.Second

	// mountPollInterval contains the delay between WaitForUserMount's parses of mtab.
	mountPollInterval = 10 * time.Millisecond

	// userCleanupWaitTime is the time we wait to cleanup a user post user creation.
	userCleanupWaitTime = 5 * time.Second

	// GuestUser is the name representing a guest user account.
	// Defined in libbrillo/brillo/cryptohome.cc.
	GuestUser = "$guest"

	// KioskUser is the name representing a kiosk user account.
	KioskUser = "kiosk"

	// mounterExe is the full executable for the out-of-process cryptohome
	// mounter.
	// Defined in cryptohome/BUILD.gn.
	mounterExe = "/usr/sbin/cryptohome-namespace-mounter"

	// cryptohomedExe is the full path of the daemon cryptohomed.
	//
	// TODO(crbug.com/1074735): Remove this and use mounterExe above for normal user mounts as well
	// once the out-of-process cryptohome mounter is ready for that.
	cryptohomedExe = "/usr/sbin/cryptohomed"
)

// hashRegexp extracts the hash from a cryptohome dir's path.
var hashRegexp *regexp.Regexp

var shadowRegexp *regexp.Regexp        // matches a path to vault under /home/shadow.
var devRegexp *regexp.Regexp           // matches a path to /dev/*.
var devLoopRegexp *regexp.Regexp       // matches a path to /dev/loop\d+.
var authSessionIDRegexp *regexp.Regexp // matches auth_session_id:*

const shadowRoot = "/home/.shadow"               // is a root directory of vault.
const mountNsPath = "/run/namespaces/mnt_chrome" // is the user session mount namespace path.
// nsfsMagic is an int64 value returned by statfs syscall in the |Type| field of the
// returned buffer. This value is returned if the filesystem type of the path is nsfs.
const nsfsMagic = 0x6e736673

func init() {
	hashRegexp = regexp.MustCompile("^/home/user/([[:xdigit:]]+)$")

	shadowRegexp = regexp.MustCompile(`^/home/\.shadow/[^/]*/vault$`)
	devRegexp = regexp.MustCompile(`^/dev(/[^/]*){1,2}$`)
	devLoopRegexp = regexp.MustCompile(`^/dev/loop[0-9]+$`)
	authSessionIDRegexp = regexp.MustCompile(`(auth_session_id:)(.+)(\n)`)
}

// MountType is a type of the user mount.
type MountType int

const (
	// Ephemeral is used to specify that the expected user mount type is ephemeral.
	Ephemeral MountType = iota
	// Permanent is used to specify that the expected user mount type is permanent.
	Permanent
)

// RemoveVault removes the vault for the user.
func RemoveVault(ctx context.Context, cryptohome *hwsec.CryptohomeClient, user string) error {
	hash, err := cryptohome.GetUserHash(ctx, user)
	if err != nil {
		return err
	}

	testing.ContextLogf(ctx, "Removing vault for user %q", user)
	cmd := testexec.CommandContext(
		ctx, "cryptohome", "--action=remove", "--force", "--user="+user)
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to remove vault for %q", user)
	}

	// Ensure that the vault does not exist.
	if _, err := os.Stat(filepath.Join(shadowRoot, hash)); !os.IsNotExist(err) {
		return errors.Wrapf(err, "cryptohome could not remove vault for user %q", user)
	}
	return nil
}

// UnmountVault unmounts the vault for the user.
func UnmountVault(ctx context.Context, mountInfo *hwsec.CryptohomeMountInfo, user string) error {
	testing.ContextLogf(ctx, "Unmounting vault for user %q", user)
	cmd := testexec.CommandContext(ctx, "cryptohome", "--action=unmount")
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to unmount vault for user %q", user)
	}

	if mounted, err := mountInfo.IsMounted(ctx, user); err == nil && mounted {
		return errors.Errorf("cryptohome did not unmount user %q", user)
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
func AddCredentialsWithAuthSession(ctx context.Context, cryptohome *hwsec.CryptohomeClient, user, password, authSessionID string, publicMount bool) error {
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
	hash, err := cryptohome.GetUserHash(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to get UserHash")
	}

	if _, err := os.Stat(filepath.Join(shadowRoot, hash)); err != nil {
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

// AuthSessionMountFlow mounts a user with AuthSession.
func AuthSessionMountFlow(ctx context.Context, cryptohome *hwsec.CryptohomeClient, mountInfo *hwsec.CryptohomeMountInfo, isKioskUser bool, username, password string, createUser bool) error {
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
		if err := AddCredentialsWithAuthSession(ctx, cryptohome, username, password, authSessionID, isKioskUser); err != nil {
			return errors.Wrap(err, "failed to add credentials with AuthSession")
		}
	}

	defer func(ctx context.Context, testUser string) error {
		// Removing the user now despite if we could authenticate or not.
		if err := RemoveVault(ctx, cryptohome, testUser); err != nil {
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

	// Unmounting user vault.
	if err := UnmountVault(ctx, mountInfo, username); err != nil {
		return errors.Wrap(err, "failed to unmount vault user -")
	}
	return nil
}
