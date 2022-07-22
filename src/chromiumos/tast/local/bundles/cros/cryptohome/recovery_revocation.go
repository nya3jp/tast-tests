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
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RecoveryRevocation,
		Desc: `Test revocation of recovery auth factor. After recovery auth factor
					 was removed, it's not possible to authenticate with it even when auth
					 factor secret is restored on disk`,
		Contacts: []string{
			"anastasiian@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"pinweaver"},
	})
}

func RecoveryRevocation(ctx context.Context, s *testing.State) {
	const (
		userName                              = "foo@bar.baz"
		userPassword                          = "secret"
		passwordLabel                         = "online-password"
		recoveryLabel                         = "test-recovery"
		testFile                              = "file"
		testFileContent                       = "content"
		cryptohomeErrorAuthorizationKeyFailed = 3
		shadow                                = "/home/.shadow"
		ussFile                               = "user_secret_stash/uss.0"
		recoveryFactorFile                    = "auth_factors/cryptohome_recovery." + recoveryLabel
		tmpUssFile                            = "tmp_uss_file"
		tmpRecoveryFactorFile                 = "tmp_recovery_factor_file"
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

	// Add a recovery auth factor to the user.
	if err := client.AddRecoveryAuthFactor(ctx, authSessionID, recoveryLabel, mediatorPubKey); err != nil {
		s.Fatal("Failed to add a recovery auth factor: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Check that USS file is created.
	hash, err := cryptohome.UserHash(ctx, userName)
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	ussFilePath := filepath.Join(shadow, hash, ussFile)
	_, err = os.Stat(ussFilePath)
	if err != nil {
		s.Fatal("USS file not created: ", err)
	}

	// Check that recovery auth factor file is created.
	recoveryFilePath := filepath.Join(shadow, hash, recoveryFactorFile)
	_, err = os.Stat(recoveryFilePath)
	if err != nil {
		s.Fatal("Recovery auth factor file not created: ", err)
	}

	// Create a temp directory for backup.
	ussBackup, err := ioutil.TempDir("", "uss_backup*")
	if err != nil {
		s.Fatal("Could not create a temp directory: ", err)
	}
	defer os.RemoveAll(ussBackup)

	// Backup USS file and AF file.
	err = fsutil.CopyFile(ussFilePath, filepath.Join(ussBackup, tmpUssFile))
	if err != nil {
		s.Fatal("Couldn't backup USS file: ", err)
	}
	err = fsutil.CopyFile(recoveryFilePath, filepath.Join(ussBackup, tmpRecoveryFactorFile))
	if err != nil {
		s.Fatal("Couldn't backup recovery auth factor file: ", err)
	}

	// Start auth session again. Password and Recovery factors are available.
	authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}

	// Authenticate with password.
	if err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}

	// Remove the recovery auth factor.
	if err := client.RemoveAuthFactor(ctx, authSessionID, recoveryLabel); err != nil {
		s.Fatal("Failed to remove recovery auth factor: ", err)
	}

	// Check that recovery auth factor file not present.
	_, err = os.Stat(recoveryFilePath)
	if err == nil {
		s.Fatal("Recovery auth factor file still present: ", err)
	}

	// Restore USS file and recovery auth factor file.
	if err := os.RemoveAll(ussFilePath); err != nil {
		s.Fatalf("Failed to remove the ussFilePath directory %v: %v", ussFilePath, err)
	}
	err = fsutil.CopyFile(filepath.Join(ussBackup, tmpUssFile), ussFilePath)
	if err != nil {
		s.Fatal("Couldn't restore USS file: ", err)
	}
	err = fsutil.CopyFile(filepath.Join(ussBackup, tmpRecoveryFactorFile), recoveryFilePath)
	if err != nil {
		s.Fatal("Couldn't restore recovery auth factor file: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Start auth session again. Password and Recovery (restored) factors are available.
	authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}

	// Start recovery.
	epoch, err := testTool.FetchFakeEpochResponseHex(ctx)
	if err != nil {
		s.Fatal("Failed to get fake epoch response: ", err)
	}

	requestHex, err := client.FetchRecoveryRequest(ctx, authSessionID, recoveryLabel, epoch)
	if err != nil {
		s.Fatal("Failed to get recovery request: ", err)
	}

	response, err := testTool.FakeMediateWithRequest(ctx, requestHex)
	if err != nil {
		s.Fatal("Failed to mediate: ", err)
	}

	// Authentication should fail now.
	err = client.AuthenticateRecoveryAuthFactor(ctx, authSessionID, recoveryLabel, epoch, response)
	var exitErr *hwsec.CmdExitError
	if !errors.As(err, &exitErr) {
		s.Fatalf("Unexpected error: got %q; want *hwsec.CmdExitError", err)
	}
	if exitErr.ExitCode != cryptohomeErrorAuthorizationKeyFailed {
		s.Fatalf("Unexpected exit code: got %d; want %d", exitErr.ExitCode, cryptohomeErrorAuthorizationKeyFailed)
	}
}
