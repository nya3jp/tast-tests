// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"bytes"
	"context"
	"io/ioutil"
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
		Func: UpdateRecovery,
		Desc: "Update recovery auth factor and authenticate again",
		Contacts: []string{
			"anastasiian@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func UpdateRecovery(ctx context.Context, s *testing.State) {
	const (
		userName        = "foo@bar.baz"
		userPassword    = "secret"
		passwordLabel   = "online-password"
		recoveryLabel   = "test-recovery"
		testFile        = "file"
		testFileContent = "content"
		shadow          = "/home/.shadow"
		recoveryFile    = "auth_factors/cryptohome_recovery." + recoveryLabel
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

	testTool, err := cryptohome.NewRecoveryTestToolWithFakeMediator()
	if err != nil {
		s.Fatal("Failed to initialize RecoveryTestTool: ", err)
	}
	defer func(s *testing.State, testTool *cryptohome.RecoveryTestTool) {
		if err := testTool.RemoveDir(); err != nil {
			s.Error("Failed to remove dir: ", err)
		}
	}(s, testTool)

	authenticateWithRecovery := func() error {
		// Authenticate a new auth session via the new added recovery auth factor and mount the user.
		authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session for re-mounting")
		}

		epoch, err := testTool.FetchFakeEpochResponseHex(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get fake epoch response")
		}

		requestHex, err := client.FetchRecoveryRequest(ctx, authSessionID, recoveryLabel, epoch)
		if err != nil {
			return errors.Wrap(err, "failed to get recovery request")
		}

		response, err := testTool.FakeMediateWithRequest(ctx, requestHex)
		if err != nil {
			return errors.Wrap(err, "failed to mediate")
		}

		if err := client.AuthenticateRecoveryAuthFactor(ctx, authSessionID, recoveryLabel, epoch, response); err != nil {
			return errors.Wrap(err, "failed to authenticate recovery auth factor")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
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
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}

	mediatorPubKey, err := testTool.FetchFakeMediatorPubKeyHex(ctx)
	if err != nil {
		s.Fatal("Failed to get mediator pub key: ", err)
	}

	// Add a recovery auth factor to the user.
	if err := client.AddRecoveryAuthFactor(ctx, authSessionID, recoveryLabel, mediatorPubKey); err != nil {
		s.Fatal("Failed to add a recovery auth factor: ", err)
	}

	hash, err := cryptohome.UserHash(ctx, userName)
	if err != nil {
		s.Fatal("Failed to get user hash: ", err)
	}
	recoveryFileContent, err := ioutil.ReadFile(filepath.Join(shadow, hash, recoveryFile))
	if err != nil {
		s.Fatalf("Could not read the recovery file (%s): %v", filepath.Join(shadow, hash, recoveryFile), err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Successfully authenticate with recovery.
	if err := authenticateWithRecovery(); err != nil {
		s.Fatal("Failed to authenticate with recovery factor: ", err)
	}

	// Update recovery auth factor.
	if err := client.UpdateRecoveryAuthFactor(ctx, authSessionID, recoveryLabel /*label*/, recoveryLabel /*newKeyLabel*/, mediatorPubKey); err != nil {
		s.Fatal("Failed to update recovery factor: ", err)
	}

	updatedRecoveryFileContent, err := ioutil.ReadFile(filepath.Join(shadow, hash, recoveryFile))
	if err != nil {
		s.Fatalf("Could not read the updated recovery file (%s): %v", filepath.Join(shadow, hash, recoveryFile), err)
	}

	// Make sure that secrets were updated.
	if bytes.Equal(recoveryFileContent, updatedRecoveryFileContent) {
		s.Fatal("The secrets in recovery file were not updated")
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Successfully authenticate with recovery factor.
	if err := authenticateWithRecovery(); err != nil {
		s.Fatal("Failed to authenticate with recovery factor after update: ", err)
	}
}
