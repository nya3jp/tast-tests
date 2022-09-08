// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UserSecretStash,
		Desc: "Test user secret stash basic password flow",
		Contacts: []string{
			"emaxx@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr:    []string{"group:mainline"},
		Fixture: "ussAuthSessionFixture",
	})
}

func UserSecretStash(ctx context.Context, s *testing.State) {
	const (
		userName                          = "foo@bar.baz"
		userPassword                      = "secret"
		passwordLabel                     = "online-password"
		cryptohomeRemoveCredentialsFailed = 54
	)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)

	// Create and mount the persistent user.
	authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)
	if err := client.PreparePersistentVault(ctx, authSessionID /*ecryptfs=*/, false); err != nil {
		s.Fatal("Failed to prepare new persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}

	authenticateWithPassword := func() error {
		// Authenticate a new auth session via the auth factor and mount the user.
		authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session for re-mounting")
		}
		authReply, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword)
		if err != nil {
			return errors.Wrap(err, "failed to authenticate with auth session")
		}
		if !authReply.Authenticated {
			return errors.New("AuthSession not authenticated despite successful reply")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ephemeral*/); err != nil {
			return errors.Wrap(err, "failed to prepare persistent vault")
		}

		// Verify that the test file is still there.
		if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
			return errors.Wrap(err, "failed to verify file persistence")
		}
		return nil
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Successfully authenticate with password.
	if err := authenticateWithPassword(); err != nil {
		s.Fatal("Failed to authenticate with password: ", err)
	}

	// Try to remove the (only) password auth factor, which should fail.
	err = client.RemoveAuthFactor(ctx, authSessionID, passwordLabel)
	var exitErr *hwsec.CmdExitError
	if !errors.As(err, &exitErr) {
		s.Fatalf("Unexpected error for auth factor removal: got %q; want *hwsec.CmdExitError", err)
	}
	if exitErr.ExitCode != cryptohomeRemoveCredentialsFailed {
		s.Fatalf("Unexpected exit code for auth factor removal: got %d; want %d", exitErr.ExitCode, cryptohomeRemoveCredentialsFailed)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Successfully authenticate with password.
	if err := authenticateWithPassword(); err != nil {
		s.Fatal("Failed to authenticate with password after removal attempt: ", err)
	}
}
