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
	_, authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral=*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
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
	if err := client.InvalidateAuthSession(ctx, authSessionID); err != nil {
		s.Fatal("Failed to invalidate auth session: ", err)
	}

	// Check the unlock works while still in-session.
	if err := testPasswordUnlock(ctx, client, userName, passwordLabel, userPassword); err != nil {
		s.Fatal("Password unlock failed: ", err)
	}

	// Unmount the user and remount them back.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}
	authSessionID, err = testPasswordLogin(ctx, client, userName, passwordLabel, userPassword)
	if err != nil {
		s.Fatal("Password remount failed: ", err)
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

	// Unmount the user and verify that authentication, remount and unlock still work.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}
	if _, err := testPasswordLogin(ctx, client, userName, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to authenticate with password after removal attempt: ", err)
	}
	if err := testPasswordUnlock(ctx, client, userName, passwordLabel, userPassword); err != nil {
		s.Fatal("Password unlock failed after removal attempt: ", err)
	}
}

func testPasswordLogin(ctx context.Context, cryptohomeClient *hwsec.CryptohomeClient, userName, passwordLabel, password string) (string, error) {
	// Start AuthSession.
	_, authSessionID, err := cryptohomeClient.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		return "", errors.Wrap(err, "failed to start auth session")
	}

	// Authenticate.
	authReply, err := cryptohomeClient.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, password)
	if err != nil {
		return "", errors.Wrap(err, "failed to authenticate with auth session")
	}
	if !authReply.Authenticated {
		return "", errors.New("AuthSession not authenticated despite successful reply")
	}
	if err := cryptohomecommon.ExpectAuthIntents(authReply.AuthorizedFor, []uda.AuthIntent{
		uda.AuthIntent_AUTH_INTENT_DECRYPT,
		uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY,
	}); err != nil {
		return "", errors.Wrap(err, "unexpected AuthSession authorized intents")
	}

	// Mount the user's vault.
	if err := cryptohomeClient.PreparePersistentVault(ctx, authSessionID, false /*ephemeral*/); err != nil {
		return "", errors.Wrap(err, "failed to prepare persistent vault")
	}

	// Verify that the test file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
		return "", errors.Wrap(err, "failed to verify file persistence")
	}
	return authSessionID, nil
}

func testPasswordUnlock(ctx context.Context, cryptohomeClient *hwsec.CryptohomeClient, userName, passwordLabel, password string) error {
	_, authSessionID, err := cryptohomeClient.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY)
	if err != nil {
		return errors.Wrap(err, "failed to start auth session")
	}
	authReply, err := cryptohomeClient.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, password)
	if err != nil {
		return errors.Wrap(err, "failed to authenticate with auth session")
	}
	if authReply.Authenticated {
		return errors.New("AuthSession authenticated despite verify-only intent")
	}
	if err := cryptohomecommon.ExpectAuthIntents(authReply.AuthorizedFor, []uda.AuthIntent{
		uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY,
	}); err != nil {
		return errors.Wrap(err, "unexpected AuthSession authorized intents")
	}
	return nil
}
