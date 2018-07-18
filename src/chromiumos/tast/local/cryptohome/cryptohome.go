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

	"chromiumos/tast/local/testexec"
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
	out, err := testexec.CommandContext(ctx, "cryptohome", "--action=remove", "--force", "--user="+user).CombinedOutput()
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
}

// WaitForUserMount waits for user's encrypted home directory to be mounted.
func WaitForUserMount(ctx context.Context, user string) error {
	p, err := UserPath(user)
	if err != nil {
		return err
	}

	// Reserve a bit of time (if it's available) to log the status before ctx's deadline.
	// TODO(derat): Delete this after https://crbug.com/864282 is resolved.
	var wctx context.Context
	var wcancel func()
	logTime := 3 * time.Second
	if dl, ok := ctx.Deadline(); ok && dl.Add(-logTime).After(time.Now()) {
		wctx, wcancel = context.WithDeadline(ctx, dl.Add(-logTime))
	} else {
		wctx, wcancel = context.WithCancel(ctx)
	}
	defer wcancel()

	testing.ContextLog(ctx, "Waiting for cryptohome ", p)
	for {
		if mounted, err := isMounted(p); err != nil {
			return err
		} else if mounted {
			return nil
		}
		if wctx.Err() != nil {
			logStatus(ctx)
			return fmt.Errorf("%s not mounted: %v", p, wctx.Err())
		}
		time.Sleep(mountPollInterval)
	}
}
