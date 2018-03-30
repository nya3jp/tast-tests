// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"chromiumos/tast/testing"
)

var cryptohomeRegexp *regexp.Regexp

func init() {
	cryptohomeRegexp = regexp.MustCompile("^/home/user/([[:xdigit:]]+)$")
}

// userCryptohomePath gets the path to the logged in user's cryptohome.
func userCryptohomePath(user string) (string, error) {
	b, err := exec.Command("cryptohome-path", "user", user).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get cryptohome for %s: %v", user, err)
	}

	p := strings.TrimSpace(string(b))
	return p, nil
}

// CryptohomeHash returns the logged-in user's cryptohome hash.
func CryptohomeHash(c *Chrome) (string, error) {
	p, err := userCryptohomePath(c.user)
	if err != nil {
		return "", err
	}
	m := cryptohomeRegexp.FindStringSubmatch(p)
	if m == nil {
		return "", fmt.Errorf("failed to read cryptohome hash for %s: %v", c.user, err)
	}

	return m[1], nil
}

// clearCryptohome clears user's encrypted home directory.
func clearCryptohome(ctx context.Context, user string) error {
	testing.ContextLog(ctx, "Clearing cryptohome for ", user)
	out, err := exec.Command("cryptohome", "--action=remove", "--force", "--user="+user).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to clear cryptohome for %s: %v: %v", user, err, string(out))
	}
	return nil
}

// waitForCryptohome waits for user's encrypted home directory to be mounted.
func waitForCryptohome(ctx context.Context, user string) error {
	p, err := userCryptohomePath(user)
	if err != nil {
		return err
	}
	testing.ContextLog(ctx, "Waiting for cryptohome ", p)
	err = poll(ctx, func() bool {
		f, err := os.Open("/etc/mtab")
		if err != nil {
			return false
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 && fields[1] == p {
				return true
			}
		}
		return false
	})
	if err != nil {
		return fmt.Errorf("%s not mounted: %v", p, err)
	}
	return nil
}
