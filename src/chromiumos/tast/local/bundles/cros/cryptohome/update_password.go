// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"bytes"
	"context"
	"io/ioutil"
	"path/filepath"
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
		Func: UpdatePassword,
		Desc: "Update password auth factor and authenticate with the new password",
		Contacts: []string{
			"anastasiian@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func UpdatePassword(ctx context.Context, s *testing.State) {
	const (
		userName                         = "foo@bar.baz"
		oldUserPassword                  = "old secret"
		newUserPassword                  = "new secret"
		passwordLabel                    = "online-password"
		wrongLabel                       = "wrong label"
		testFile                         = "file"
		testFileContent                  = "content"
		cryptohomeAuthorizationKeyFailed = 3
		cryptohomeErrorKeyNotFound       = 15
	)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	// Wait for cryptohomed becomes available if needed.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Clean up obsolete state, in case there's any.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Enable the UserSecretStash experiment for the duration of the test by
	// creating a flag file that's checked by cryptohomed.
	// TODO(b/223213284): Implement the test for VaultKeyset users.
	cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
	if err != nil {
		s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
	}
	defer cleanupUSSExperiment(ctx)

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
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Write a test file to verify persistence.
	userPath, err := cryptohome.UserPath(ctx, userName)
	if err != nil {
		s.Fatal("Failed to get user vault path: ", err)
	}
	filePath := filepath.Join(userPath, testFile)
	if err := ioutil.WriteFile(filePath, []byte(testFileContent), 0644); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	authenticateWithPassword := func(password string) error {
		// Authenticate a new auth session via the auth factor and mount the user.
		authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session for re-mounting")
		}
		authReply, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, password)
		if err != nil {
			return errors.Wrap(err, "failed to authenticate with auth session")
		}
		if !authReply.Authenticated {
			return errors.New("AuthSession not authenticated despite successful reply")
		}
		if err := cryptohomecommon.ExpectAuthIntents(authReply.AuthorizedFor, []uda.AuthIntent{
			uda.AuthIntent_AUTH_INTENT_DECRYPT,
			uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY,
		}); err != nil {
			return errors.Wrap(err, "unexpected AuthSession authorized intents")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ephemeral*/); err != nil {
			return errors.Wrap(err, "failed to prepare persistent vault")
		}

		// Verify that the test file is still there.
		if content, err := ioutil.ReadFile(filePath); err != nil {
			return errors.Wrap(err, "failed to read back test file")
		} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
			return errors.Errorf("incorrect tests file content. got: %q, want: %q", content, testFileContent)
		}
		return nil
	}

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, oldUserPassword); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Successfully authenticate with password.
	if err := authenticateWithPassword(oldUserPassword); err != nil {
		s.Fatal("Failed to authenticate with password: ", err)
	}

	// Try to update password auth factor with wrong label.
	err = client.UpdateAuthFactor(ctx, authSessionID, wrongLabel /*label*/, wrongLabel /*newKeyLabel*/, newUserPassword)
	var exitErr *hwsec.CmdExitError
	if !errors.As(err, &exitErr) {
		s.Fatalf("Unexpected error for auth factor update: got %q; want *hwsec.CmdExitError", err)
	}
	if exitErr.ExitCode != cryptohomeErrorKeyNotFound {
		s.Fatalf("Unexpected exit code for auth factor update: got %d; want %d", exitErr.ExitCode, cryptohomeErrorKeyNotFound)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Successfully authenticate with old password.
	if err := authenticateWithPassword(oldUserPassword); err != nil {
		s.Fatal("Failed to authenticate with password after update attempt: ", err)
	}

	// Update password auth factor.
	if err := client.UpdateAuthFactor(ctx, authSessionID, passwordLabel /*label*/, passwordLabel /*newKeyLabel*/, newUserPassword); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Authentication with old password fails.
	err = authenticateWithPassword(oldUserPassword)
	var authExitErr *hwsec.CmdExitError
	if !errors.As(err, &authExitErr) {
		s.Fatalf("Unexpected error for authentication with old password: got %q; want *hwsec.CmdExitError", err)
	}
	if authExitErr.ExitCode != cryptohomeAuthorizationKeyFailed {
		s.Fatalf("Unexpected exit code for authentication with old password: got %d; want %d",
			authExitErr.ExitCode, cryptohomeAuthorizationKeyFailed)
	}

	// Successfully authenticate with new password.
	if err := authenticateWithPassword(newUserPassword); err != nil {
		s.Fatal("Failed to authenticate with password: ", err)
	}
}
