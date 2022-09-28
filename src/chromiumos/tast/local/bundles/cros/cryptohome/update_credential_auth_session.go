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
		Func: UpdateCredentialAuthSession,
		Desc: "Test if credentials are updated with AuthSession and that the user can mount post update",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"pinweaver"},
	})
}

const (
	cryptohomeErrorAuthorizationKeyFailed = 3
)

func UpdateCredentialAuthSession(ctx context.Context, s *testing.State) {
	const (
		userName        = "foo@bar.baz"
		userPassword    = "secret"
		updatedPassword = "updatedsecret"
		wrongPassword   = "wrongPassword"
		keyLabel        = "fake_label"
		userPin         = "123456"
		updatedPin      = "098765"
		pinLabel        = "pin"
		wrongPin        = "000000"
	)

	// Step 0: Setup.
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

	// Ensure cryptohomed is started and wait for it to be available.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Step 1: Create a user with password. At this point the user only has one
	// credential configured.
	if err := cryptohome.CreateUserWithAuthSession(ctx, userName, userPassword, keyLabel, false /*=isKioskUser*/); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)

	// Step 2: Mount the user vault for the first time. Authenticate with AuthSession
	// first, and then use the same AuthSession for preparing the vault.
	authSessionID, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, keyLabel, false /*=isEphemeral*/, false /*=isKioskUser*/)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Step 3&4: Get user path and create a file to check for persistence later.
	if err := cryptohome.WriteFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	// Step 5: Add Pin credentials for the user. After this the user has a password
	// and a pin configured to mount.
	if err = client.AddPinCredentialsWithAuthSession(ctx, pinLabel, userPin, authSessionID); err != nil {
		s.Fatal("Failed to add pin as a credential: ", err)
	}
	client.InvalidateAuthSession(ctx, authSessionID)

	// Step 6: Check we can use lock screen with password. This also checks if wrong
	// password attempt fails.
	if err := cryptohome.TestLockScreen(ctx, userName, userPassword, wrongPassword, keyLabel, client); err != nil {
		s.Fatal("Failed to check lock screen with initial password: ", err)
	}

	// Step 7: Check we can use lock screen with pin. This also checks if wrong pin
	// attempt fails.
	if err := cryptohome.TestLockScreenPin(ctx, userName, userPin, wrongPin, pinLabel, client); err != nil {
		s.Fatal("Failed to check lock screen with initial pin: ", err)
	}

	// Step 8: Update password credential for the user.
	authSessionID, err = cryptohome.UpdateUserCredentialWithAuthSession(ctx, userName, userPassword, updatedPassword, keyLabel, false /*=isEphemeral*/, false /*=isKioskUser*/)
	if err != nil {
		s.Fatal("Failed to update credential: ", err)
	}
	client.InvalidateAuthSession(ctx, authSessionID)

	// Step 9: Unmount everything. We will remount with the updated password credentials.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Step 10: Authenticate with old and then new credential -- both for password keysets.
	// One will expectedly fail and the other will not.
	// Note: the correct password now is the updated password.
	authSessionID, err = loginWithCorrectAndIncorrectCredentials(ctx, client, userName, userPassword, updatedPassword, keyLabel, false /*=is_pin*/)
	if err != nil {
		s.Fatal("Could not successfully login with appropriate credentials: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Step 11: Following successful auth, ensure that the file we created earlier still exists.
	if err := mountAndVerifyFilePersistence(ctx, client, userName, authSessionID); err != nil {
		s.Fatal("Failed to verify file persistence when logging in with updated password: ", err)
	}
	defer client.UnmountAll(ctx)
	client.InvalidateAuthSession(ctx, authSessionID)

	// Step 12: Ensure we still pass unlock screen with passwords.
	if err := cryptohome.TestLockScreen(ctx, userName, updatedPassword, userPassword, keyLabel, client); err != nil {
		s.Fatal("Failed to check lock screen with updated password: ", err)
	}

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults: ", err)
	}

	// Step 13: Authenticate with a correct and an incorrect pin credential.
	// One will expectedly fail and the other will not.
	authSessionID, err = loginWithCorrectAndIncorrectCredentials(ctx, client, userName, wrongPin, userPin, pinLabel, true /*=is_pin*/)
	if err != nil {
		s.Fatal("Could not successfully login with appropriate credentials: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Step 14: Following successful auth, ensure that the file we created
	// earlier still exists.
	if err := mountAndVerifyFilePersistence(ctx, client, userName, authSessionID); err != nil {
		s.Fatal("Failed to verify file persistence when logging in with pin after updating password: ", err)
	}
	defer client.UnmountAll(ctx)
	client.InvalidateAuthSession(ctx, authSessionID)

	// Step 15: Ensure we still pass unlock screen with pin.
	if err := cryptohome.TestLockScreenPin(ctx, userName, userPin, wrongPin, pinLabel, client); err != nil {
		s.Fatal("Failed to check lock screen after re-login: ", err)
	}

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults: ", err)
	}

	// Step 16: Update pin credential for the user.
	authSessionID, err = cryptohome.UpdateUserCredentialWithAuthSession(ctx, userName, userPin, updatedPin, pinLabel, false /*=isEphemeral*/, false /*=isKioskUser*/)
	if err != nil {
		s.Fatal("Failed to update credential: ", err)
	}
	client.InvalidateAuthSession(ctx, authSessionID)

	// Step 17: Authenticate with old and then new credential -- both for
	// password keysets. One will expectedly fail and the other will not.
	// Note: the correct password is the updatedPassword.
	authSessionID, err = loginWithCorrectAndIncorrectCredentials(ctx, client, userName, userPassword, updatedPassword, keyLabel, false /*=is_pin*/)
	if err != nil {
		s.Fatal("Could not successfully login with appropriate credentials: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Step 18: Following successful auth, ensure that the file we created
	// earlier still exists.
	if err := mountAndVerifyFilePersistence(ctx, client, userName, authSessionID); err != nil {
		s.Fatal("Failed to verify file persistenc when logging in with updatedPassword, post update pin: ", err)
	}
	defer client.UnmountAll(ctx)
	client.InvalidateAuthSession(ctx, authSessionID)

	// Step 19: Ensure we still pass unlock screen with passwords.
	if err := cryptohome.TestLockScreen(ctx, userName, updatedPassword, userPassword, keyLabel, client); err != nil {
		s.Fatal("Failed to check lock screen after re-login: ", err)
	}

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults: ", err)
	}

	// Step 20: Authenticate with a correct and an incorrect pin credential.
	// One will expectedly fail and the other will not.
	// Note: the correct pin now is the updatedPin.
	authSessionID, err = loginWithCorrectAndIncorrectCredentials(ctx, client, userName, userPin, updatedPin, pinLabel, true /*=is_pin*/)
	if err != nil {
		s.Fatal("Could not successfully login with appropriate credentials: ", err)
	}

	// Step 21: Following successful auth, ensure that the file we created earlier still exists.
	if err := mountAndVerifyFilePersistence(ctx, client, userName, authSessionID); err != nil {
		s.Fatal("Failed to verify file persistence after after logging in with new pin: ", err)
	}
	defer client.UnmountAll(ctx)
	client.InvalidateAuthSession(ctx, authSessionID)

	// Step 22: Ensure we still pass unlock screen with pin.
	if err := cryptohome.TestLockScreenPin(ctx, userName, updatedPin, userPin, pinLabel, client); err != nil {
		s.Fatal("Failed to check lock screen after re-login: ", err)
	}
}

// loginWithCorrectAndIncorrectCredentials first attempts to authenticate with
// wrongSecret and then the correct secret. First should fail and second should pass.
func loginWithCorrectAndIncorrectCredentials(ctx context.Context, client *hwsec.CryptohomeClient, userName, wrongSecret, secret, keyLabel string, isPin bool) (string, error) {
	// Start an Auth session and get an authSessionID.
	_, authSessionID, err := client.StartAuthSession(ctx, userName /*isEphemeral*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		return authSessionID, errors.Wrap(err, "failed to start Auth session")
	}

	// Attempt the incorrect credential first
	if isPin {
		err = client.AuthenticatePinWithAuthSession(ctx, wrongSecret, keyLabel, authSessionID)
	} else {
		err = client.AuthenticateAuthSession(ctx, wrongSecret, keyLabel, authSessionID, false /*=isKioskUser*/)
	}

	if err == nil {
		return authSessionID, errors.Wrap(err, "authenticate with incorrect credentials succeeded when it should not have")
	}

	var exitErr *hwsec.CmdExitError
	if !errors.As(err, &exitErr) {
		return authSessionID, errors.Wrap(err, "unexpected error, want *hwsec.CmdExitError")
	}

	if exitErr.ExitCode != cryptohomeErrorAuthorizationKeyFailed {
		return authSessionID, errors.Wrap(err, "authenticate with incorrect credentials failed but with unexpected error")
	}

	// Attempt the correct credential.
	if isPin {
		err = client.AuthenticatePinWithAuthSession(ctx, secret, keyLabel, authSessionID)
	} else {
		err = client.AuthenticateAuthSession(ctx, secret, keyLabel, authSessionID, false /*=isKioskUser*/)
	}

	return authSessionID, err
}

// mountAndVerifyFilePersistence mounts using the given AuthSessionID and ensures
// that the file we wrote earlier still exists.
func mountAndVerifyFilePersistence(ctx context.Context, client *hwsec.CryptohomeClient, username, authSessionID string) error {
	// Write a test file to verify persistence.
	if err := client.PreparePersistentVault(ctx, authSessionID, false /*=ecryptfs*/); err != nil {
		return errors.Wrap(err, "failed to prepare persistent vault with given credential")
	}

	// Verify that file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, username); err != nil {
		return errors.Wrap(err, "failed to verify file persistence")
	}
	return nil
}
