// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"io/ioutil"
	"os"
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
	"chromiumos/tast/testing/hwdep"
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
		// TODO(b/195385797): Run on gooey when the bug is fixed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("gooey")),
		Fixture:      "ussAuthSessionFixture",
	})
}

func RecoveryOptOut(ctx context.Context, s *testing.State) {
	const (
		userName      = "foo@bar.baz"
		userPassword  = "secret"
		passwordLabel = "online-password"
		recoveryLabel = "test-recovery"
		userGaiaID    = "123456789"
		deviceUserID  = "123-456-AA-BB"
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

	// Create and mount the persistent user.
	_, authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
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
	if err := client.AddRecoveryAuthFactor(ctx, authSessionID, recoveryLabel, mediatorPubKey, userGaiaID, deviceUserID); err != nil {
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
	_, authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
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
	_, authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}

	err = authenticateWithRecoveryFactor(ctx, authSessionID, recoveryLabel)
	if err := cryptohomecommon.ExpectCryptohomeErrorCode(err, uda.CryptohomeErrorCode_CRYPTOHOME_ERROR_KEY_NOT_FOUND); err != nil {
		s.Fatal("Failed to get the correct error code for auth factor removal: ", err)
	}
}
