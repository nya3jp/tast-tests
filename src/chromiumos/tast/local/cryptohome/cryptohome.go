// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cryptohome operates on encrypted home directories.
package cryptohome

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/godbus/dbus"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	// mountPollInterval contains the delay between WaitForUserMount's parses of mtab.
	mountPollInterval = 10 * time.Millisecond

	// GuestUser is the name representing a guest user account.
	// Defined in libbrillo/brillo/cryptohome.cc.
	GuestUser = "$guest"
)

// hashRegexp extracts the hash from a cryptohome dir's path.
var hashRegexp *regexp.Regexp

var shadowRegexp *regexp.Regexp  // matches a path to vault under /home/shadow.
var devRegexp *regexp.Regexp     // matches a path to /dev/*.
var devLoopRegexp *regexp.Regexp // matches a path to /dev/loop\d+.

const shadowRoot = "/home/.shadow" // is a root directory of vault.

// InstallAttributesPath is the path to install_attributes file.
const InstallAttributesPath = shadowRoot + "/install_attributes.pb"

func init() {
	hashRegexp = regexp.MustCompile("^/home/user/([[:xdigit:]]+)$")

	shadowRegexp = regexp.MustCompile(`^/home/\.shadow/[^/]*/vault$`)
	devRegexp = regexp.MustCompile(`^/dev/[^/]*$`)
	devLoopRegexp = regexp.MustCompile(`^/dev/loop[0-9]+$`)
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
	validatePartition := validatePermanentPartition
	if user == GuestUser {
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

	const waitTimeout = 30 * time.Second
	testing.ContextLogf(ctx, "Waiting for cryptohome for user %q with timeout %v", user, waitTimeout)
	err = testing.Poll(ctx, func(ctx context.Context) error {
		partitions, err := disk.Partitions(true /* all */)
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

	err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(
			ctx, "cryptohome", "--action=mount_ex",
			"--user="+user, "--password="+password,
			"--async", "--create", "--key_label=bar")
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			return err
		}

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
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 1 * time.Second})

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

// IsMounted checks if the vault for the user is mounted.
func IsMounted(ctx context.Context, user string) (bool, error) {
	validatePartition := validatePermanentPartition
	if user == GuestUser {
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
	partitions, err := disk.Partitions(true /* all */)
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

// CreateInstallAttributes creates the install_attributes file on device if not
// present already. Sets the data as if the device is consumer owned.
func CreateInstallAttributes(ctx context.Context) error {
	if _, err := os.Stat(InstallAttributesPath); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to check file presence")
		}

		// Set the data as if the device is consumer owned.
		data := []byte{0x01, 0x08}
		if err := ioutil.WriteFile(InstallAttributesPath, []byte(fmt.Sprintf("%x", data)), 0604); err != nil {
			return errors.Wrapf(err, "failed to write to %s", InstallAttributesPath)
		}
	}
	return nil
}

// CheckService performs high-level verification of cryptohomed.
// If an error is returned, CheckDeps can be called to return additional
// information pointing to the cause of the problem.
func CheckService(ctx context.Context) error {
	if err := checkJob(ctx, "cryptohomed"); err != nil {
		return err
	}

	bus, err := dbus.SystemBus()
	if err != nil {
		return errors.Wrap(err, "failed to connect to D-Bus system bus")
	}
	const (
		svcName    = "org.chromium.Cryptohome"
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
				if err := checkJob(ctx, job); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	// chapsd should be running unconditionally.
	if err := checkJob(ctx, "chapsd"); err != nil {
		errs = append(errs, err)
	}

	return errs
}

// checkJob checks the named upstart job and returns an error if it isn't running or
// has a process in the zombie state.
func checkJob(ctx context.Context, job string) error {
	if goal, state, pid, err := upstart.JobStatus(ctx, job); err != nil {
		return errors.Wrapf(err, "failed to get %v status", job)
	} else if goal != upstart.StartGoal || state != upstart.RunningState {
		return errors.Errorf("%v not running (%v/%v)", job, goal, state)
	} else if proc, err := process.NewProcess(int32(pid)); err != nil {
		return errors.Wrapf(err, "failed to check %v process %d", job, pid)
	} else if status, err := proc.Status(); err != nil {
		return errors.Wrapf(err, "failed to get %v process %d status", job, pid)
	} else if status == "Z" {
		return errors.Errorf("%v process %d is a zombie", job, pid)
	}
	return nil
}
