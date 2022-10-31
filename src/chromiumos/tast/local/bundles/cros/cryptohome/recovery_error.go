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
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RecoveryError,
		Desc:     "Checks that the correct error code is returned after cryptohome recovery failure",
		Contacts: []string{"anastasiian@chromium.org", "cros-lurs@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "ussAuthSessionFixture",
		// For "no_tpm_dynamic" - see http://b/251789202.
		SoftwareDeps: []string{"tpm", "no_tpm_dynamic"},
		// TODO(b/195385797): Run on gooey when the bug is fixed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("gooey")),
	})
}

func RecoveryError(ctx context.Context, s *testing.State) {
	const (
		userName             = "foo@bar.baz"
		userPassword         = "secret"
		passwordLabel        = "online-password"
		recoveryLabel        = "test-recovery"
		recoveryUserGaiaID   = "123456789"
		recoveryDeviceUserID = "123-456-AA-BB"
		// TODO(b/250518701): Don't user hardcoded constants, generate the error rpc response by the test tool.
		// CryptoRecoveryRpcResponse with error set to RECOVERY_ERROR_EPOCH.
		responseEpochErrHex = "08011804"
		// CryptoRecoveryRpcResponse with error set to RECOVERY_ERROR_FATAL.
		responseFatalErrHex = "08011801"
	)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)

	// Create and mount the persistent user.
	_, authSessionID, err := client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)
	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
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
	if err := client.AddRecoveryAuthFactor(ctx, authSessionID, recoveryLabel, mediatorPubKey, recoveryUserGaiaID, recoveryDeviceUserID); err != nil {
		s.Fatal("Failed to add a recovery auth factor: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Authenticate a new auth session via the new added recovery auth factor.
	_, authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}

	epoch, err := testTool.FetchFakeEpochResponseHex(ctx)
	if err != nil {
		s.Fatal("Failed to get fake epoch response: ", err)
	}

	_, err = client.FetchRecoveryRequest(ctx, authSessionID, recoveryLabel, epoch)
	if err != nil {
		s.Fatal("Failed to get recovery request: ", err)
	}

	// Authenticate with `responseEpochErrHex` - transient error is expected.
	err = client.AuthenticateRecoveryAuthFactor(ctx, authSessionID, recoveryLabel, epoch, responseEpochErrHex)
	if err := cryptohomecommon.ExpectCryptohomeErrorCode(err, uda.CryptohomeErrorCode_CRYPTOHOME_ERROR_RECOVERY_TRANSIENT); err != nil {
		s.Fatal("Failed to get the correct error code for transient error: ", err)
	}

	// Authenticate with `responseFatalErrHex` - fatal error is expected.
	err = client.AuthenticateRecoveryAuthFactor(ctx, authSessionID, recoveryLabel, epoch, responseFatalErrHex)
	if err := cryptohomecommon.ExpectCryptohomeErrorCode(err, uda.CryptohomeErrorCode_CRYPTOHOME_ERROR_RECOVERY_FATAL); err != nil {
		s.Fatal("Failed to get the correct error code for fatal error: ", err)
	}
}
