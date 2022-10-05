// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	uda "chromiumos/system_api/user_data_auth_proto"
	cryptohomecommon "chromiumos/tast/common/cryptohome"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RemovePassword,
		Desc: "Adds and removes password in user secret stash",
		Contacts: []string{
			"anastasiian@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"reboot"},
		Fixture:      "ussAuthSessionFixture",
	})
}

func RemovePassword(ctx context.Context, s *testing.State) {
	const (
		userName                   = "foo@bar.baz"
		userPassword               = "secret"
		userPin                    = "123456"
		passwordLabel              = "online-password"
		pinLabel                   = "test-pin"
		cryptohomeErrorKeyNotFound = 15
	)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)

	// Create and mount the persistent user.
	authSessionID, err := client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)

	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to prepare new persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	authenticateWithPinAuthFactor := func() (string, error) {
		// Unmount the user.
		if err := client.UnmountAll(ctx); err != nil {
			return "", errors.Wrap(err, "failed to unmount vaults for re-mounting")
		}

		// Authenticate a new auth session via the new added pin auth factor and mount the user.
		authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return "", errors.Wrap(err, "failed to start auth session for re-mounting")
		}
		if err := client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
			return authSessionID, errors.Wrap(err, "failed to authenticate with pin")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			return authSessionID, errors.Wrap(err, "failed to prepare persistent vault")
		}

		// Verify that file is still there.
		if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
			return authSessionID, errors.Wrap(err, "failed to verify test file")
		}
		return authSessionID, nil
	}

	unlockWithPassword := func() (string, error) {
		// Unmount the user.
		if err := client.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmount vaults for re-mounting: ", err)
		}

		// Authenticate a new auth session via the password auth factor and mount the user.
		authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY)
		if err != nil {
			return "", errors.Wrap(err, "failed to start auth session for re-mounting")
		}
		authReply, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword)
		if err != nil {
			return authSessionID, errors.Wrap(err, "failed to authenticate (unlock) with password")
		}
		if err := cryptohomecommon.ExpectAuthIntents(authReply.AuthorizedFor, []uda.AuthIntent{
			uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY,
		}); err != nil {
			return authSessionID, errors.Wrap(err, "unexpected AuthSession authorized intents")
		}
		return authSessionID, nil
	}

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to add a password authfactor: ", err)
	}

	// Add a pin auth factor to the user.
	if err := client.AddPinAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
		s.Fatal("Failed to add pin auth factor: ", err)
	}

	// Authenticate.
	authSessionID, err = authenticateWithPinAuthFactor()
	if err != nil {
		s.Fatal("Failed to authenticate with pin authfactor: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Can unlock with password.
	authSessionID, err = unlockWithPassword()
	if err != nil {
		s.Fatal("Failed to unlock with password authfactor: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Remove the password.
	if err := client.RemoveAuthFactor(ctx, authSessionID, passwordLabel); err != nil {
		s.Fatal("Failed to remove password authfactor: ", err)
	}

	// Unlock with password fails.
	authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	var exitErr *hwsec.CmdExitError
	_, err = client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword)
	if !errors.As(err, &exitErr) {
		s.Fatalf("Unexpected error during unlock with password: got %q; want *hwsec.CmdExitError", err)
	}
	if exitErr.ExitCode != cryptohomeErrorKeyNotFound {
		s.Fatalf("Unexpected exit code during unlock with password: got %d; want %d",
			exitErr.ExitCode, cryptohomeErrorKeyNotFound)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Authentication with password fails.
	authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	_, err = client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword)
	if !errors.As(err, &exitErr) {
		s.Fatalf("Unexpected error during authentication with password: got %q; want *hwsec.CmdExitError", err)
	}
	if exitErr.ExitCode != cryptohomeErrorKeyNotFound {
		s.Fatalf("Unexpected exit code during authentication with password: got %d; want %d",
			exitErr.ExitCode, cryptohomeErrorKeyNotFound)
	}
}
