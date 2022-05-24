// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/disk"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	// GuestUser is the name representing a guest user account.
	// Defined in libbrillo/brillo/cryptohome.cc.
	GuestUser = "$guest"

	// KioskUser is the name representing a kiosk user account.
	KioskUser = "kiosk"

	// WaitForUserTimeout is the maximum time until a user mount is available.
	WaitForUserTimeout = 80 * time.Second

	// mounterExe is the full executable for the out-of-process cryptohome
	// mounter.
	// Defined in cryptohome/BUILD.gn.
	mounterExe = "/usr/sbin/cryptohome-namespace-mounter"

	// cryptohomedExe is the full path of the daemon cryptohomed.
	//
	// TODO(crbug.com/1074735): Remove this and use mounterExe above for
	// normal user mounts as well once the out-of-process cryptohome
	// mounter is ready for that.
	cryptohomedExe = "/usr/sbin/cryptohomed"

	// mountPollInterval contains the delay between WaitForUserMount's parses of mtab.
	mountPollInterval = 10 * time.Millisecond

	// The user session mount namespace path.
	mountNsPath = "/run/namespaces/mnt_chrome"

	// The root directory of vault.
	shadowRoot = "/home/.shadow"
)

var (
	// matches a path to vault under /home/shadow.
	// Example: "/home/.shadow/118c4648065f5cd3660e17a53533ec7bc924d01f/vault"
	shadowRegexp = regexp.MustCompile(`^/home/\.shadow/[^/]*/vault$`)

	// matches a path to /dev/*.
	// Example: "/dev/sda1"
	devRegexp = regexp.MustCompile(`^/dev(/[^/]*){1,2}$`)

	// matches a path to /dev/loop\d+.
	// Example: "/dev/loop0"
	devLoopRegexp = regexp.MustCompile(`^/dev/loop[0-9]+$`)
)

// MountType is a type of the user mount.
type MountType int

const (
	// Ephemeral is used to specify that the expected user mount type is ephemeral.
	Ephemeral MountType = iota
	// Permanent is used to specify that the expected user mount type is permanent.
	Permanent
)

// CryptohomeMountInfo is a helper to get cryptohome mount information.
type CryptohomeMountInfo struct {
	runner     CmdRunner
	cryptohome *CryptohomeClient
}

// NewCryptohomeMountInfo creates a new CryptohomeMountInfo
func NewCryptohomeMountInfo(r CmdRunner, c *CryptohomeClient) *CryptohomeMountInfo {
	return &CryptohomeMountInfo{r, c}
}

// IsMounted checks if the vault for the user is mounted.
func (c *CryptohomeMountInfo) IsMounted(ctx context.Context, user string) (bool, error) {
	mounter := cryptohomedExe
	validatePartition := validatePermanentPartition

	if user == GuestUser {
		mounter = mounterExe
		validatePartition = validateGuestPartition
	}

	userpath, err := c.cryptohome.GetHomeUserPath(ctx, user)
	if err != nil {
		return false, errors.Wrap(err, "failed to get user home path")
	}
	systempath, err := c.cryptohome.GetRootUserPath(ctx, user)
	if err != nil {
		return false, errors.Wrap(err, "failed to get user root path")
	}
	partitions, err := c.findMounts(ctx, mounter)
	if err != nil {
		return false, errors.Wrap(err, "failed to find mount points")
	}

	up := findPartition(partitions, userpath)
	if up == nil {
		return false, nil
	}
	if !validatePartition(up) {
		return false, nil
	}
	sp := findPartition(partitions, systempath)
	if sp == nil {
		return false, nil
	}
	if !validatePartition(sp) {
		return false, nil
	}
	return true, nil
}

// CleanUpMount cleans up the mount point for the user, and check it's unmounted.
func (c *CryptohomeMountInfo) CleanUpMount(ctx context.Context, user string) error {
	if _, err := c.cryptohome.Unmount(ctx, user); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	if _, err := c.cryptohome.RemoveVault(ctx, user); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	mounted, err := c.IsMounted(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to get mount info")
	}
	if mounted {
		return errors.Errorf("mount point of %q still exists", user)
	}
	return nil
}

// WaitForUserMountAndValidateType waits for user's encrypted home directory to
// be mounted and validates that it is of correct type.
func (c *CryptohomeMountInfo) WaitForUserMountAndValidateType(ctx context.Context, user string, mountType MountType) error {
	ctx, st := timing.Start(ctx, "wait_for_user_mount")
	defer st.End()

	mounter := cryptohomedExe
	validatePartition := validatePermanentPartition

	if mountType == Ephemeral {
		mounter = mounterExe
		validatePartition = validateGuestPartition
	}

	userpath, err := c.cryptohome.GetHomeUserPath(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to get user home path")
	}
	systempath, err := c.cryptohome.GetRootUserPath(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to get user root path")
	}

	testing.ContextLogf(ctx, "Waiting for cryptohome for user %q with timeout %v", user, WaitForUserTimeout)
	err = testing.Poll(ctx, func(ctx context.Context) error {
		partitions, err := c.findMounts(ctx, mounter)
		if err != nil {
			return err
		}
		up := findPartition(partitions, userpath)
		if up == nil {
			return errors.Errorf("%v not found", userpath)
		}
		if !validatePartition(up) {
			return errors.Errorf("%v not a valid partition", up)
		}
		sp := findPartition(partitions, systempath)
		if sp == nil {
			return errors.Errorf("%v not found", systempath)
		}
		if !validatePartition(sp) {
			return errors.Errorf("%v not a valid partition", sp)
		}
		return nil
	}, &testing.PollOptions{Timeout: WaitForUserTimeout, Interval: mountPollInterval})

	if err != nil {
		return errors.Wrapf(err, "not mounted for %s", user)
	}
	return nil
}

// WaitForUserMount waits for user's encrypted home directory to be mounted and
// validates that it is of permanent type for all users except guest.
func (c *CryptohomeMountInfo) WaitForUserMount(ctx context.Context, user string) error {
	mountType := Permanent
	if user == GuestUser {
		mountType = Ephemeral
	}
	return c.WaitForUserMountAndValidateType(ctx, user, mountType)
}

// CheckMountNamespace checks whether the user session mount namespace has been created.
func (c *CryptohomeMountInfo) CheckMountNamespace(ctx context.Context) error {
	mounts, err := c.readMountsInfo(ctx, "/proc/mounts")
	if err != nil {
		return errors.Wrap(err, "failed to read mount information")
	}
	mp := findPartition(mounts, mountNsPath)
	if mp == nil {
		return errors.Errorf("%v not found", mountNsPath)
	}
	if mp.Fstype != "nsfs" {
		return errors.Errorf("user session mount namespace has not been created at %s", mountNsPath)
	}
	return nil
}

// UserCryptohomePath returns the path where the cryptohome data for the user is located.
func (c *CryptohomeMountInfo) UserCryptohomePath(ctx context.Context, user string) (string, error) {
	hash, err := c.cryptohome.GetUserHash(ctx, user)
	if err != nil {
		return "", err
	}
	return filepath.Join(shadowRoot, hash), nil
}

// MountedVaultPath returns the path where the decrypted data for the user is located.
func (c *CryptohomeMountInfo) MountedVaultPath(ctx context.Context, user string) (string, error) {
	path, err := c.UserCryptohomePath(ctx, user)
	if err != nil {
		return "", err
	}
	return filepath.Join(path, "mount"), nil
}

// findMounterPID finds the pid of the given mounter process.
func (c *CryptohomeMountInfo) findMounterPID(ctx context.Context, mounter string) (int32, error) {
	bs, err := c.runner.Run(ctx, "pidof", "-s", mounter)
	var ee *CmdExitError
	// If the mounter process is not found, don't return an error.
	if errors.As(err, &ee) && ee.ExitCode == 1 {
		return -1, nil
	}
	if err != nil {
		return -1, errors.Wrap(err, "could not get the mounter pid")
	}

	// Convert the result to pid
	msg := string(bs)
	pid, err := strconv.ParseInt(strings.TrimSpace(msg), 10, 32)
	if err != nil {
		return -1, errors.Wrapf(err, "failed to convert the pid %q", msg)
	}
	return int32(pid), nil
}

// readMountsInfo returns the list of mounts from a mount information file.
func (c *CryptohomeMountInfo) readMountsInfo(ctx context.Context, path string) ([]disk.PartitionStat, error) {
	bs, err := c.runner.Run(ctx, "cat", "--", path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get list of mounts from %v", path)
	}
	output := strings.Trim(string(bs), "\n")
	mounts := strings.Split(output, "\n")

	var res []disk.PartitionStat
	for _, mount := range mounts {
		fields := strings.Fields(mount)
		// The mount information should match this format:
		// https://man7.org/linux/man-pages/man5/fstab.5.html
		if len(fields) != 6 {
			return nil, errors.Errorf("mount information doesn't match 6 fields: %q", mount)
		}
		d := disk.PartitionStat{
			Device:     fields[0],
			Mountpoint: fields[1],
			Fstype:     fields[2],
			Opts:       []string{fields[3]},
		}
		res = append(res, d)
	}
	return res, nil
}

// findMountsForPID returns the list of mounts in pid's mount namespace.
func (c *CryptohomeMountInfo) findMountsForPID(ctx context.Context, pid int32) ([]disk.PartitionStat, error) {
	path := fmt.Sprintf("/proc/%d/mounts", pid)
	res, err := c.readMountsInfo(ctx, path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get list of mounts for pid %d", pid)
	}
	return res, nil
}

// findMounts returns the list of mounts in the given mounter's mount namespace.
func (c *CryptohomeMountInfo) findMounts(ctx context.Context, mounter string) ([]disk.PartitionStat, error) {
	mounterPID, err := c.findMounterPID(ctx, mounter)
	if err != nil {
		return nil, errors.Wrap(err, "could not list running processes")
	} else if mounterPID == -1 {
		// If the mounter process is not running, the list of mounts is
		// empty.
		return nil, nil
	}
	return c.findMountsForPID(ctx, mounterPID)
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
// (non-guest) user mount.
func validatePermanentPartition(p *disk.PartitionStat) bool {
	switch p.Fstype {
	case "ext4":
		if !devRegexp.MatchString(p.Device) || devLoopRegexp.MatchString(p.Device) {
			return false
		}
	case "ecryptfs":
		if !shadowRegexp.MatchString(p.Device) {
			return false
		}
	default:
		return false
	}
	return true
}

// validateGuestPartition checks if the given partition is valid for a guest
// user mount.
func validateGuestPartition(p *disk.PartitionStat) bool {
	switch p.Fstype {
	case "ext4":
		if !devLoopRegexp.MatchString(p.Device) {
			return false
		}
	case "tmpfs":
		if p.Device != "guestfs" {
			return false
		}
	default:
		return false
	}
	return true
}
