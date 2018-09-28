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
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"

	"github.com/shirou/gopsutil/disk"
)

const (
	// mountPollInterval contains the delay between WaitForUserMount's parses of mtab.
	mountPollInterval = 10 * time.Millisecond
)

// hashRegexp extracts the hash from a cryptohome dir's path.
var hashRegexp *regexp.Regexp

// shadowRegexp matches a path to vault under /home/shadow.
var shadowRegexp *regexp.Regexp

// devRegexp matches a path to /dev/*.
var devRegexp *regexp.Regexp

// devLoopRegexp matches a path to /dev/loop\d+.
var devLoopRegexp *regexp.Regexp

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
		return "", fmt.Errorf("didn't find hash in path %q", p)
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
	out, err := testexec.CommandContext(ctx, "cryptohome", "--action=remove", "--force", "--user="+user).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v (%v)", err, strings.TrimSpace(string(out)))
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

// findPartition returns partition stat for the given |path|.
func findPartition(ps []disk.PartitionStat, path string) *disk.PartitionStat {
	for _, p := range ps {
		if p.Mountpoint == path {
			return &p
		}
	}
	return nil
}

// validatePartition checks if the given partition is valid for the user mount.
func validatePartition(p *disk.PartitionStat) bool {
	switch p.Fstype {
	case "ext4":
		return devRegexp.MatchString(p.Device) && !devLoopRegexp.MatchString(p.Device)
	case "ecryptfs":
		return shadowRegexp.MatchString(p.Device)
	}
	return false
}

// WaitForUserMount waits for user's encrypted home directory to be mounted.
func WaitForUserMount(ctx context.Context, user string) error {
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

	testing.ContextLog(ctx, "Waiting for cryptohome ", user)
	err = testing.Poll(ctx, func(ctx context.Context) error {
		partitions, err := disk.Partitions(true /* all */)
		if err != nil {
			return err
		}
		up := findPartition(partitions, userpath)
		if up == nil || !validatePartition(up) {
			return fmt.Errorf("Not found: %v", userpath)
		}
		sp := findPartition(partitions, systempath)
		if sp == nil || !validatePartition(sp) {
			return fmt.Errorf("Not found: %v", systempath)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: mountPollInterval})

	if err != nil {
		logStatus(ctx)
		return fmt.Errorf("Not mounted for %s: %v", user, err)
	}
	return nil
}
