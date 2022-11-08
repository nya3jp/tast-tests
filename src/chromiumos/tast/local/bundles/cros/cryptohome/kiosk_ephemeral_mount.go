// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KioskEphemeralMount,
		Desc: "Ensures that cryptohome correctly mounts kiosk sessions with ephemeral vaults",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Data: []string{"testcert.p12"},
		Params: []testing.Param{{
			Name:    "with_vk",
			Fixture: "vkAuthSessionFixture",
		}, {
			Name:    "with_uss",
			Fixture: "ussAuthSessionFixture",
		}},
	})
}

func KioskEphemeralMount(ctx context.Context, s *testing.State) {
	const (
		ownerName   = "owner@bar.baz"
		cleanupTime = 20 * time.Second
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	daemonController := helper.DaemonController()

	// Wait for cryptohomed to become available if needed.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Clean up old state or mounts for the test user, if any exists.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if err := cryptohome.RemoveVault(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Set up an owner. This is needed for ephemeral users. Once this is done
	// unmount everything to put things in a clean state for the test proper.
	if err := hwseclocal.SetUpVaultAndUserAsOwner(ctx, s.DataPath("testcert.p12"), ownerName, "whatever", "whatever", helper.CryptohomeClient()); err != nil {
		client.UnmountAll(ctx)
		client.RemoveVault(ctx, ownerName)
		s.Fatal("Failed to setup vault and user as owner: ", err)
	}
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	defer client.RemoveVault(cleanupCtx, ownerName)

	// Create the user with an ephemeral vault.
	// Verify that we CANNOT add a kiosk credential.
	if err := client.WithAuthSession(ctx, cryptohome.KioskUser, true /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT, func(authSessionID string) error {
		if err := client.PrepareEphemeralVault(ctx, authSessionID); err != nil {
			return errors.Wrap(err, "failed to prepare new ephemeral vault")
		}

		// Here we try to add a kiosk credential. This should fail! This seems
		// a bit confusing, because if this is a kiosk test you would expect us
		// to use kiosk credentials. But 1) kiosk does not support a lock screen
		// and so does not support verify-only credentials, and 2) ephemeral
		// users can only use verify-only credentials.
		//
		// What this means is that an "ephemeral kiosk" is really just an
		// ephemeral user with NO credentials.
		if err := client.AddKioskAuthFactor(ctx, authSessionID); err == nil {
			return errors.New("adding kiosk credentials to an ephemeral user should have failed but did not")
		}

		// Write a file. This file should be gone once we unmount.
		if err := cryptohome.WriteFileForPersistence(ctx, cryptohome.KioskUser); err != nil {
			return errors.Wrap(err, "failed to write test file")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to create and set up the user: ", err)
	}

	// Unmount the user.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount ephemeral user")
	}

	// Verify that we can no longer read the file we wrote.
	if err := cryptohome.VerifyFileUnreadability(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("File verified when it should not have: ", err)
	}

	// Create the user with an ephemeral vault, again.
	// Verify that we cannot read the file we added to the first ephemeral vault.
	if err := client.WithAuthSession(ctx, cryptohome.KioskUser, true /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT, func(authSessionID string) error {
		if err := client.PrepareEphemeralVault(ctx, authSessionID); err != nil {
			return errors.Wrap(err, "failed to prepare new ephemeral vault")
		}
		if err := cryptohome.VerifyFileUnreadability(ctx, cryptohome.KioskUser); err != nil {
			return errors.Wrap(err, "file verified when it should not have")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to create and set up the user a second time: ", err)
	}
}
