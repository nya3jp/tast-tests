// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cryptohome operates on encrypted home directories.
package cryptohome

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/shirou/gopsutil/disk"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
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

func init() {
	hashRegexp = regexp.MustCompile("^/home/user/([[:xdigit:]]+)$")

	shadowRegexp = regexp.MustCompile(`^/home/\.shadow/[^/]*/vault$`)
	devRegexp = regexp.MustCompile(`^/dev/[^/]*$`)
	devLoopRegexp = regexp.MustCompile(`^/dev/loop[0-9]+$`)
}

// UserHash returns user's cryptohome hash.
func UserHash(user string) (string, error) {
	p, err := UserPath(user)
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
func UserPath(user string) (string, error) {
	b, err := exec.Command("cryptohome-path", "user", user).Output()
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
func RemoveUserDir(ctx context.Context, user string) error {
	testing.ContextLog(ctx, "Removing cryptohome for ", user)
	cmd := testexec.CommandContext(ctx, "cryptohome", "--action=remove", "--force", "--user="+user)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to remove cryptohome")
	}
	return nil
}

// logStatus logs information about cryptohome's status.
// TODO(derat): Delete this after https://crbug.com/864282 is resolved.
func logStatus(ctx context.Context) {
	cmd := testexec.CommandContext(ctx, "cryptohome", "--action=status")
	if b, err := cmd.Output(); err != nil {
		testing.ContextLog(ctx, "Failed to get cryptohome status")
		cmd.DumpLog(ctx)
	} else {
		testing.ContextLog(ctx, "cryptohome status:\n", strings.TrimSpace(string(b)))
	}

	for _, p := range []string{"/sys/class/tpm/tpm0/device/owned", "/sys/class/misc/tpm0/device/owned"} {
		if _, err := os.Stat(p); err == nil {
			if b, err := ioutil.ReadFile(p); err == nil {
				testing.ContextLogf(ctx, "%v contains %q", p, strings.TrimSpace(string(b)))
			} else {
				testing.ContextLogf(ctx, "Failed to read %v: %v", p, err)
			}
		}
	}
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

	userpath, err := UserPath(user)
	if err != nil {
		return err
	}
	systempath, err := SystemPath(user)
	if err != nil {
		return err
	}

	// Reserve a bit of time to log the status before ctx's deadline.
	// TODO(derat): Delete this after https://crbug.com/864282 is resolved.
	var timeout time.Duration
	if dl, ok := ctx.Deadline(); ok {
		timeout = dl.Sub(time.Now()) - (3 * time.Second) // testing.Poll ignores negative timeouts
	}

	testing.ContextLogf(ctx, "Waiting for cryptohome for user %q", user)
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
	}, &testing.PollOptions{Timeout: timeout, Interval: mountPollInterval})

	if err != nil {
		logStatus(ctx)
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
		hash, err := UserHash(user)
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(shadowRoot, hash)); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 6 * time.Second, Interval: 1 * time.Second})

	if err != nil {
		return errors.Wrapf(err, "failed to create vault for %s", user)
	}

	return nil
}

// RemoveVault removes the vault for the user.
func RemoveVault(ctx context.Context, user string) error {
	hash, err := UserHash(user)
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

	userpath, err := UserPath(user)
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
