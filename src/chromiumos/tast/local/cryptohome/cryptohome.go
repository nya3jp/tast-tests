// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cryptohome operates on encrypted home directories.
package cryptohome

// TODO(b/182152667): We should deprecate the usage of this file

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	// mountPollInterval contains the delay between WaitForUserMount's parses of mtab.
	mountPollInterval = 10 * time.Millisecond

	// GuestUser is the name representing a guest user account.
	// Defined in libbrillo/brillo/cryptohome.cc.
	GuestUser = "$guest"

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

// UserHash returns user's cryptohome hash.
func UserHash(ctx context.Context, user string) (string, error) {
	p, err := UserPath(ctx, user)
	if err != nil {
		return "", err
	}
	m := hashRegexp.FindStringSubmatch(p)
	if m == nil {
		return "", errors.Errorf("didn't find hash in path %q", p)
	}
	return m[1], nil
}

// UserPath returns the path to user's encrypted home directory.
func UserPath(ctx context.Context, user string) (string, error) {
	b, err := testexec.CommandContext(ctx, "cryptohome-path", "user", user).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// SystemPath returns the path to user's encrypted system directory.
func SystemPath(user string) (string, error) {
	b, err := exec.Command("cryptohome-path", "system", user).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// RemoveUserDir removes a user's encrypted home directory.
// Success is reported if the user directory doesn't exist,
// but an error will be returned if the user is currently logged in.
func RemoveUserDir(ctx context.Context, user string) error {
	testing.ContextLog(ctx, "Removing cryptohome for ", user)
	cmd := testexec.CommandContext(ctx, "cryptohome", "--action=remove", "--force", "--user="+user)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to remove cryptohome")
	}
	return nil
}

// findMounterPID finds the pid of the given mounter process.
func findMounterPID(mounter string) (int32, error) {
	procs, err := process.Processes()
	if err != nil {
		return -1, errors.Wrap(err, "could not list running processes")
	}

	for _, proc := range procs {
		// With 'cros deploy', the underlying cryptohomed binary may be overwritten
		// which results in a dangling symlink ('/usr/sbin/cryptohomed (deleted)').
		if exe, err := proc.Exe(); err == nil && exe != "" && strings.Contains(exe, mounter) {
			return proc.Pid, nil
		}
	}

	// If the mounter process is not found, don't return an error.
	return -1, nil
}

// findMountsForPID returns the list of mounts in pid's mount namespace.
func findMountsForPID(pid int32) ([]disk.PartitionStat, error) {
	path := fmt.Sprintf("/proc/%d/mounts", pid)
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get list of mounts for pid %d", pid)
	}
	output := strings.Trim(string(bs), "\n")
	mounts := strings.Split(output, "\n")

	res := make([]disk.PartitionStat, 0, len(mounts))
	for _, mount := range mounts {
		var d disk.PartitionStat
		fields := strings.Fields(mount)
		d = disk.PartitionStat{
			Device:     fields[0],
			Mountpoint: fields[1],
			Fstype:     fields[2],
			Opts:       fields[3],
		}
		res = append(res, d)
	}
	return res, nil
}

// findMounts returns the list of mounts in the given mounter's mount namespace.
func findMounts(mounter string) ([]disk.PartitionStat, error) {
	mounterPID, err := findMounterPID(mounter)
	if err != nil {
		return nil, errors.Wrap(err, "could not list running processes")
	} else if mounterPID == -1 {
		// If the mounter process is not running, the list of mounts is
		// empty.
		return nil, nil
	}
	return findMountsForPID(mounterPID)
}

// findPartition returns a pointer to the entry in ps corresponding to path,
// or nil if no matching entry is present.
func findPartition(ps []disk.PartitionStat, path string) *disk.PartitionStat {
	for i := range ps {
		if ps[i].Mountpoint == path {
			return &ps[i]
		}
	}
	return nil
}

// validatePermanentPartition checks if the given partition is valid for a
// (non-guest) user mount. Returns nil on success, or reports an error.
func validatePermanentPartition(p *disk.PartitionStat) error {
	switch p.Fstype {
	case "ext4":
		if !devRegexp.MatchString(p.Device) || devLoopRegexp.MatchString(p.Device) {
			return errors.Errorf("ext4 device %q should match %q excluding %q", p.Device, devRegexp, devLoopRegexp)
		}
	case "ecryptfs":
		if !shadowRegexp.MatchString(p.Device) {
			return errors.Errorf("ecryptfs device %q should match %q", p.Device, shadowRegexp)
		}
	default:
		return errors.Errorf("unexpected file system: %q", p.Fstype)
	}
	return nil
}

// validateGuestPartition checks if the given partition is valid for a guest
// user mount. Returns nil on success, or reports an error.
func validateGuestPartition(p *disk.PartitionStat) error {
	switch p.Fstype {
	case "ext4":
		if !devLoopRegexp.MatchString(p.Device) {
			return errors.Errorf("ext4 device %q should match %q", p.Device, devLoopRegexp)
		}
	case "tmpfs":
		if p.Device != "guestfs" {
			return errors.Errorf("tmpfs device %q should be guestfs", p.Device)
		}
	default:
		return errors.Errorf("unexpected file system: %q", p.Fstype)
	}
	return nil
}

// WaitForUserMount waits for user's encrypted home directory to be mounted.
func WaitForUserMount(ctx context.Context, user string) error {
	ctx, st := timing.Start(ctx, "wait_for_user_mount")
	defer st.End()

	mounter := cryptohomedExe
	validatePartition := validatePermanentPartition

	if user == GuestUser {
		mounter = mounterExe
		validatePartition = validateGuestPartition
	}

	userpath, err := UserPath(ctx, user)
	if err != nil {
		return err
	}
	systempath, err := SystemPath(user)
	if err != nil {
		return err
	}

	const waitTimeout = 80 * time.Second
	testing.ContextLogf(ctx, "Waiting for cryptohome for user %q with timeout %v", user, waitTimeout)
	err = testing.Poll(ctx, func(ctx context.Context) error {
		partitions, err := findMounts(mounter)
		if err != nil {
			return err
		}
		up := findPartition(partitions, userpath)
		if up == nil {
			return errors.Errorf("%v not found", userpath)
		}
		if err = validatePartition(up); err != nil {
			return err
		}
		sp := findPartition(partitions, systempath)
		if sp == nil {
			return errors.Errorf("%v not found", systempath)
		}
		if err = validatePartition(sp); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: waitTimeout, Interval: mountPollInterval})

	if err != nil {
		return errors.Wrapf(err, "not mounted for %s", user)
	}
	return nil
}

// CreateVault creates the vault for the user with given password.
func CreateVault(ctx context.Context, user, password string) error {
	testing.ContextLogf(ctx, "Creating vault mount for user %q", user)

	cmd := testexec.CommandContext(
		ctx, "cryptohome", "--action=mount_ex",
		"--user="+user, "--password="+password,
		"--async", "--create", "--key_label=bar")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrapf(err, "failed to create vault for %s", user)
	}

	err := testing.Poll(ctx, func(ctx context.Context) error {
		// TODO(crbug.com/690994): Remove this additional call to
		// UserHash when crbug.com/690994 is fixed.
		hash, err := UserHash(ctx, user)
		if err != nil {
			return err
		}

		if _, err := os.Stat(filepath.Join(shadowRoot, hash)); err != nil {
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
	hash, err := UserHash(ctx, user)
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

// UnmountAll unmounts all user vaults.
func UnmountAll(ctx context.Context) error {
	testing.ContextLog(ctx, "Unmounting all user vaults")
	cmd := testexec.CommandContext(ctx, "cryptohome", "--action=unmount")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to unmount vaults")
	}
	return nil
}

// UnmountVault unmounts the vault for the user.
func UnmountVault(ctx context.Context, user string) error {
	testing.ContextLogf(ctx, "Unmounting vault for user %q", user)
	cmd := testexec.CommandContext(ctx, "cryptohome", "--action=unmount")
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to unmount vault for user %q", user)
	}

	if mounted, err := IsMounted(ctx, user); err == nil && mounted {
		return errors.Errorf("cryptohome did not unmount user %q", user)
	}
	return nil
}

// MountedVaultPath returns the path where the decrypted data for the user is located.
func MountedVaultPath(ctx context.Context, user string) (string, error) {
	hash, err := UserHash(ctx, user)
	if err != nil {
		return "", err
	}
	return filepath.Join(shadowRoot, hash, "mount"), nil
}

// IsMounted checks if the vault for the user is mounted.
func IsMounted(ctx context.Context, user string) (bool, error) {
	mounter := cryptohomedExe
	validatePartition := validatePermanentPartition

	if user == GuestUser {
		mounter = mounterExe
		validatePartition = validateGuestPartition
	}

	userpath, err := UserPath(ctx, user)
	if err != nil {
		return false, err
	}
	systempath, err := SystemPath(user)
	if err != nil {
		return false, err
	}
	partitions, err := findMounts(mounter)
	if err != nil {
		return false, err
	}

	up := findPartition(partitions, userpath)
	if up == nil {
		return false, nil
	}
	if err = validatePartition(up); err != nil {
		return false, nil
	}
	sp := findPartition(partitions, systempath)
	if sp == nil {
		return false, nil
	}
	if err = validatePartition(sp); err != nil {
		return false, nil
	}
	return true, nil
}

// MountGuest sends a request to cryptohome to create a mount point for a
// guest user.
func MountGuest(ctx context.Context) error {
	testing.ContextLog(ctx, "Mounting guest cryptohome")
	cmd := testexec.CommandContext(ctx, "cryptohome", "--action=mount_guest_ex")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to request mounting guest vault")
	}

	if err := WaitForUserMount(ctx, GuestUser); err != nil {
		return errors.Wrap(err, "failed to mount guest vault")
	}
	return nil
}

// CheckMountNamespace checks whether the user session mount namespace has been created.
func CheckMountNamespace(ctx context.Context) error {
	var buff *syscall.Statfs_t
	if err := syscall.Statfs(mountNsPath, buff); err != nil {
		return errors.Wrapf(err, "failed to stat mount namespace file at %s", mountNsPath)
	}
	if buff.Type != nsfsMagic {
		return errors.Errorf("user session mount namespace has not been created at %s", mountNsPath)
	}
	return nil
}

// CheckService performs high-level verification of cryptohomed.
// If an error is returned, CheckDeps can be called to return additional
// information pointing to the cause of the problem.
func CheckService(ctx context.Context) error {
	if err := upstart.CheckJob(ctx, "cryptohomed"); err != nil {
		return err
	}

	bus, err := dbusutil.SystemBus()
	if err != nil {
		return errors.Wrap(err, "failed to connect to D-Bus system bus")
	}
	const (
		svcName = "org.chromium.UserDataAuth"

		svcTimeout = 10 * time.Second
	)
	svcCtx, svcCancel := context.WithTimeout(ctx, svcTimeout)
	defer svcCancel()
	if err := dbusutil.WaitForService(svcCtx, bus, svcName); err != nil {
		return errors.Wrapf(err, "%v D-Bus service unavailable", svcName)
	}

	return nil
}

// CheckDeps checks services that cryptohomed depends on and returns a list of potential problems.
// It can be used to collect more detail after CheckService reports an error.
func CheckDeps(ctx context.Context) (errs []error) {
	if out, err := testexec.CommandContext(ctx, "tpmc", "tpmver").Output(); err != nil {
		errs = append(errs, errors.Wrap(err, "unknown TPM version"))
	} else {
		version := strings.TrimSpace(string(out))
		switch version {
		case "1.2":
			// TPM 1.2 systems use the trousers library rather than a daemon.
		case "2.0":
			for _, job := range []string{"attestationd", "tpm_managerd", "trunksd"} {
				if err := upstart.CheckJob(ctx, job); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	// chapsd should be running unconditionally.
	if err := upstart.CheckJob(ctx, "chapsd"); err != nil {
		errs = append(errs, err)
	}

	return errs
}

// StartAuthSession starts an AuthSession for a given user.
func StartAuthSession(ctx context.Context, user string, publicMount bool) (string, error) {
	testing.ContextLogf(ctx, "Creating AuthSession for user %q", user)
	cmd := testexec.CommandContext(
		ctx, "cryptohome", "--action=start_auth_session",
		"--user="+user)
	if publicMount {
		cmd.Args = append(cmd.Args, "--public_mount")
	}
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
	hash, err := UserHash(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to get UserHash")
	}

	if _, err := os.Stat(filepath.Join(shadowRoot, hash)); err != nil {
		return errors.Wrap(err, "failed to get user cryptohome directory")
	}
	return nil
}

// MountWithAuthSession mounts user with AuthSessionID.
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
