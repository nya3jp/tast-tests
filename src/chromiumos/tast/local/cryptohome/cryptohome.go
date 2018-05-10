// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cryptohome operates on encrypted home directories.
package cryptohome

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/testing"
)

const (
	// mountPollInterval contains the delay between WaitForUserMount's parses of mtab.
	mountPollInterval = 10 * time.Millisecond
)

// hashRegexp extracts the hash from a cryptohome dir's path.
var hashRegexp *regexp.Regexp

func init() {
	hashRegexp = regexp.MustCompile("^/home/user/([[:xdigit:]]+)$")
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

// RemoveUserDir removes a user's encrypted home directory.
func RemoveUserDir(ctx context.Context, user string) error {
	testing.ContextLog(ctx, "Removing cryptohome for ", user)
	out, err := exec.Command("cryptohome", "--action=remove", "--force", "--user="+user).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v (%v)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// isMounted returns true if dir is mounted.
func isMounted(dir string) (bool, error) {
	f, err := os.Open("/etc/mtab")
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == dir {
			return true, nil
		}
	}
	return false, nil
}

// WaitForUserMount waits for user's encrypted home directory to be mounted.
func WaitForUserMount(ctx context.Context, user string) error {
	p, err := UserPath(user)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Waiting for cryptohome ", p)
	for {
		if mounted, err := isMounted(p); err != nil {
			return err
		} else if mounted {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("%s not mounted: %v", p, ctx.Err())
		}
		time.Sleep(mountPollInterval)
	}
}
