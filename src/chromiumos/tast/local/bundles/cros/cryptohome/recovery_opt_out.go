// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
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
		Func: RecoveryOptOut,
		Desc: `Test the opt-out of recovery auth factor. After recovery auth factor
					 was removed, it's not possible to authenticate with it even when the
					 secrets are restored on disk`,
		Contacts: []string{
			"anastasiian@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"pinweaver"},
	})
}

func RecoveryOptOut(ctx context.Context, s *testing.State) {
	const (
		userName                              = "foo@bar.baz"
		userPassword                          = "secret"
		passwordLabel                         = "online-password"
		recoveryLabel                         = "test-recovery"
		cryptohomeErrorAuthorizationKeyFailed = 3
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
	cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
	if err != nil {
		s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
	}
	defer cleanupUSSExperiment()

	// Create and mount the persistent user.
	authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)
	if err := client.PreparePersistentVault(ctx, authSessionID /*ecryptfs*/, false); err != nil {
		s.Fatal("Failed to prepare new persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to add a password authfactor: ", err)
	}

	testTool, err := cryptohome.NewRecoveryTestToolWithFakeMediator()
	if err != nil {
		s.Fatal("Failed to initialize RecoveryTestTool: ", err)
	}
	defer func(s *testing.State, testTool *cryptohome.RecoveryTestTool) {
		if err := testTool.RemoveDir(); err != nil {
			s.Error("Failed to remove dir: ", err)
		}
	}(s, testTool)

	mediatorPubKey, err := testTool.FetchFakeMediatorPubKeyHex(ctx)
	if err != nil {
		s.Fatal("Failed to get mediator pub key: ", err)
	}

	authenticateWithRecoveryFactor := func(ctx context.Context, authSessionID, label string) error {
		// Start recovery.
		epoch, err := testTool.FetchFakeEpochResponseHex(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get fake epoch response")
		}
		requestHex, err := client.FetchRecoveryRequest(ctx, authSessionID, label, epoch)
		if err != nil {
			return errors.Wrap(err, "failed to get recovery request")
		}
		response, err := testTool.FakeMediateWithRequest(ctx, requestHex)
		if err != nil {
			return errors.Wrap(err, "failed to mediate")
		}
		return client.AuthenticateRecoveryAuthFactor(ctx, authSessionID, label, epoch, response)
	}

	// Add a recovery auth factor to the user.
	if err := client.AddRecoveryAuthFactor(ctx, authSessionID, recoveryLabel, mediatorPubKey); err != nil {
		s.Fatal("Failed to add a recovery auth factor: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Create a temp directory for backup.
	loginDataBackup, err := ioutil.TempDir("", "login_data_backup*")
	if err != nil {
		s.Fatal("Could not create a temp directory: ", err)
	}
	defer os.RemoveAll(loginDataBackup)

	// Backup the login data.
	dataPath := filepath.Join(loginDataBackup, "data.tar.gz")
	s.Log("Preparing login data of current version")
	if err := hwseclocal.SaveLoginData(ctx, daemonController, dataPath, false /*includeTpm*/); err != nil {
		s.Fatal("Failed to backup login data: ", err)
	}

	// Start auth session again. Password and Recovery factors are available.
	authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}

	// Authenticate with recovery factor.
	if err := authenticateWithRecoveryFactor(ctx, authSessionID, recoveryLabel); err != nil {
		s.Fatal("Failed to authenticate with recovery auth factor: ", err)
	}

	// Remove the recovery auth factor.
	if err := client.RemoveAuthFactor(ctx, authSessionID, recoveryLabel); err != nil {
		s.Fatal("Failed to remove recovery auth factor: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Restore the login data.
	if err := hwseclocal.LoadLoginData(ctx, daemonController, dataPath, false /*includeTpm*/); err != nil {
		s.Fatal("Failed to restore login data: ", err)
	}

	// Start auth session again. Password and Recovery (restored) factors are available.
	authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}

	err = authenticateWithRecoveryFactor(ctx, authSessionID, recoveryLabel)
	var exitErr *hwsec.CmdExitError
	if !errors.As(err, &exitErr) {
		s.Fatalf("Unexpected error in authentication after factor removal: got %q; want *hwsec.CmdExitError", err)
	}
	if exitErr.ExitCode != cryptohomeErrorAuthorizationKeyFailed {
		s.Fatalf("Unexpected exit code in authentication after factor removal: got %d; want %d",
			exitErr.ExitCode, cryptohomeErrorAuthorizationKeyFailed)
	}
}
