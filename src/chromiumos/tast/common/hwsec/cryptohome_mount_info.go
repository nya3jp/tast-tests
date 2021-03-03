// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/disk"

	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
)

// CryptohomeMountInfo is a helper to get cryptohome mount information.
type CryptohomeMountInfo struct {
	runner     CmdRunner
	cryptohome *CryptohomeClient

	shadowRegexp  *regexp.Regexp // matches a path to vault under /home/shadow.
	devRegexp     *regexp.Regexp // matches a path to /dev/*.
	devLoopRegexp *regexp.Regexp // matches a path to /dev/loop\d+.
}

const (
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

// NewCryptohomeMountInfo creates a new CryptohomeMountInfo, with r responsible for CmdRunner
// and c responsible for CryptohomeClient.
func NewCryptohomeMountInfo(r CmdRunner, c *CryptohomeClient) *CryptohomeMountInfo {
	shadowRegexp := regexp.MustCompile(`^/home/\.shadow/[^/]*/vault$`)
	devRegexp := regexp.MustCompile(`^/dev(/[^/]*){1,2}$`)
	devLoopRegexp := regexp.MustCompile(`^/dev/loop[0-9]+$`)
	return &CryptohomeMountInfo{r, c, shadowRegexp, devRegexp, devLoopRegexp}
}

// findMounterPID finds the pid of the given mounter process.
func (c *CryptohomeMountInfo) findMounterPID(ctx context.Context, mounter string) (int32, error) {
	bs, err := c.runner.Run(ctx, "pgrep", "-x", "-o", "-f", shutil.Escape(mounterExe))
	if err != nil {
		return -1, errors.Wrap(err, "could not get the mounter pid")
	}

	// Convert the result to pid
	msg := string(bs)
	pid, err := strconv.Atoi(strings.TrimSpace(msg))
	if err != nil {
		return -1, errors.Wrapf(err, "failed to convert the pid %q", msg)
	}
	return int32(pid), nil
}

// findMountsForPID returns the list of mounts in pid's mount namespace.
func (c *CryptohomeMountInfo) findMountsForPID(ctx context.Context, pid int32) ([]disk.PartitionStat, error) {
	path := fmt.Sprintf("/proc/%d/mounts", pid)
	bs, err := c.runner.Run(ctx, "cat", "--", shutil.Escape(path))
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
func (c *CryptohomeMountInfo) findPartition(ps []disk.PartitionStat, path string) *disk.PartitionStat {
	for i := range ps {
		if ps[i].Mountpoint == path {
			return &ps[i]
		}
	}
	return nil
}

// validatePermanentPartition checks if the given partition is valid for a
// (non-guest) user mount. Returns nil on success, or reports an error.
func (c *CryptohomeMountInfo) validatePermanentPartition(p *disk.PartitionStat) error {
	switch p.Fstype {
	case "ext4":
		if !c.devRegexp.MatchString(p.Device) || c.devLoopRegexp.MatchString(p.Device) {
			return errors.Errorf("ext4 device %q should match %q excluding %q", p.Device, c.devRegexp, c.devLoopRegexp)
		}
	case "ecryptfs":
		if !c.shadowRegexp.MatchString(p.Device) {
			return errors.Errorf("ecryptfs device %q should match %q", p.Device, c.shadowRegexp)
		}
	default:
		return errors.Errorf("unexpected file system: %q", p.Fstype)
	}
	return nil
}

// validateGuestPartition checks if the given partition is valid for a guest
// user mount. Returns nil on success, or reports an error.
func (c *CryptohomeMountInfo) validateGuestPartition(p *disk.PartitionStat) error {
	switch p.Fstype {
	case "ext4":
		if !c.devLoopRegexp.MatchString(p.Device) {
			return errors.Errorf("ext4 device %q should match %q", p.Device, c.devLoopRegexp)
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

// IsMounted checks if the vault for the user is mounted.
func (c *CryptohomeMountInfo) IsMounted(ctx context.Context, user string) (bool, error) {
	mounter := cryptohomedExe
	validatePartition := c.validatePermanentPartition

	if user == GuestUser {
		mounter = mounterExe
		validatePartition = c.validateGuestPartition
	}

	userpath, err := c.cryptohome.GetHomeUserPath(ctx, user)
	if err != nil {
		return false, err
	}
	systempath, err := c.cryptohome.GetRootUserPath(ctx, user)
	if err != nil {
		return false, err
	}
	partitions, err := c.findMounts(ctx, mounter)
	if err != nil {
		return false, err
	}

	up := c.findPartition(partitions, userpath)
	if up == nil {
		return false, nil
	}
	if err = validatePartition(up); err != nil {
		return false, nil
	}
	sp := c.findPartition(partitions, systempath)
	if sp == nil {
		return false, nil
	}
	if err = validatePartition(sp); err != nil {
		return false, nil
	}
	return true, nil
}
