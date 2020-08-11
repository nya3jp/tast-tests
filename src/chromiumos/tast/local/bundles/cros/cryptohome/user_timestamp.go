// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UserTimestamp,
		Desc: "Test removing oldest user",
		Contacts: []string{
			"asavery@chromium.org",
			"gwendal@chromium.org",
			"chromeos-storage@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func UserTimestamp(ctx context.Context, s *testing.State) {
	const (
		shadow        = "/home/.shadow"
		timestampFile = "master.0.timestamp"
		keysetFile    = "master.0"

		user1    = "user1"
		user2    = "user2"
		user3    = "user3"
		password = "1234"

		timestampNew = "0"
		// This comes from kSetCurrentUserOldOffsetInDays, which is the
		// number of days that the set_current_user_old action uses when
		// updating the home directory timestamp.
		timestampOld = "92"
	)

	createUser := func(ctx context.Context, user, pass string) error {
		if err := cryptohome.CreateVault(ctx, user, pass); err != nil {
			return errors.Wrap(err, "failed to create user vault")
		}
		success := false
		defer func() {
			if !success {
				cryptohome.RemoveVault(ctx, user)
			}
		}()

		hash, err := cryptohome.UserHash(ctx, user)
		if err != nil {
			return errors.Wrap(err, "failed to get user hash")
		}

		// The update user activity timestamp action is not mandatory, so we perform it after
		// CryptohomeMount() returns, in the background. We need to add a poll for the
		// timestamp file to give it more time to complete.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := os.Stat(filepath.Join(shadow, hash, timestampFile)); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "timestamp file not found")
		}

		success = true
		return nil
	}

	checkDumpKeyset := func(ctx context.Context, user, age string) error {
		testing.ContextLogf(ctx, "Checking user keyset %q", user)
		cmd := testexec.CommandContext(
			ctx, "cryptohome", "--action=dump_keyset", "--user="+user)
		output, err := cmd.Output()
		if err != nil {
			return errors.Wrap(err, "failed to set user old")
		}
		re := regexp.MustCompile(`(?m)(.*)Last activity \(days ago\):$\s*(?P<age>\d+)`)
		match := re.FindStringSubmatch(string(output))
		if match[2] != age {
			return errors.Wrap(err, "last activity is not expected value")
		}
		return nil
	}

	checkLastActivity := func(ctx context.Context, user, age string) error {
		testing.ContextLogf(ctx, "Checking last activity %q", user)
		cmd := testexec.CommandContext(
			ctx, "cryptohome", "--action=dump_last_activity")
		output, err := cmd.Output()
		if err != nil {
			return errors.Wrap(err, "failed to set user old")
		}

		hash, err := cryptohome.UserHash(ctx, user)
		if err != nil {
			return errors.Wrap(err, "failed to get user hash")
		}

		pattern := "(?m)(.*)" + hash + `\s*(?P<age>\d+)$`
		re := regexp.MustCompile(pattern)
		match := re.FindStringSubmatch(string(output))
		if match[2] != age {
			return errors.Wrap(err, "last activity is not expected value")
		}
		return nil
	}

	// Start cryptohomed and wait for it to be available
	if err := upstart.EnsureJobRunning(ctx, "cryptohomed"); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Failed to start cryptohomed: ", err)
	}

	if err := createUser(ctx, user1, password); err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, user1)

	if err := createUser(ctx, user2, password); err != nil {
		s.Fatal("Failed to create user with content: ", err)
	}
	defer cryptohome.RemoveVault(ctx, user2)

	if err := checkDumpKeyset(ctx, user1, timestampNew); err != nil {
		s.Fatal("Unexpected value in keyset: ", err)
	}

	if err := checkLastActivity(ctx, user1, timestampNew); err != nil {
		s.Fatal("Unexpected value for last activity: ", err)
	}

	if err := checkDumpKeyset(ctx, user2, timestampNew); err != nil {
		s.Fatal("Unexpected value in keyset: ", err)
	}

	if err := checkLastActivity(ctx, user2, timestampNew); err != nil {
		s.Fatal("Unexpected value for last activity: ", err)
	}

	if err := cryptohome.UnmountVault(ctx, user1); err != nil {
		s.Fatal("Failed to unmount user vault: ", err)
	}

	if err := cryptohome.CreateVault(ctx, user2, password); err != nil {
		s.Fatal("Failed to remount user vault: ", err)
	}

	if err := cryptohome.WaitForUserMount(ctx, user2); err != nil {
		s.Fatal("Failed to remount user vault: ", err)
	}
	defer cryptohome.UnmountVault(ctx, user2)

	hash, err := cryptohome.UserHash(ctx, user2)
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}

	user2StatBefore, err := os.Stat(filepath.Join(shadow, hash, keysetFile))
	if err != nil {
		s.Fatal("Keyset file not found: ", err)
	}

	testing.ContextLogf(ctx, "Setting user old %q", user2)
	cmd := testexec.CommandContext(
		ctx, "cryptohome", "--action=set_current_user_old")
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to set user old: ", err)
	}

	user2StatAfter, err := os.Stat(filepath.Join(shadow, hash, keysetFile))
	if err != nil {
		s.Fatal("Keyset file not found: ", err)
	}
	if user2StatBefore.ModTime() != user2StatAfter.ModTime() {
		s.Fatal("The keyset file has been modified after changing timestamp")
	}

	if err := checkDumpKeyset(ctx, user1, timestampNew); err != nil {
		s.Fatal("Unexpected value in keyset: ", err)
	}

	if err := checkLastActivity(ctx, user1, timestampNew); err != nil {
		s.Fatal("Unexpected value for last activity: ", err)
	}

	if err := checkDumpKeyset(ctx, user2, timestampOld); err != nil {
		s.Fatal("Unexpected value in keyset: ", err)
	}

	if err := checkLastActivity(ctx, user2, timestampOld); err != nil {
		s.Fatal("Unexpected value for last activity: ", err)
	}
}
