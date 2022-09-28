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
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RemoveCredentialAuthSession,
		Desc: "Test when credentials are removed with AuthSession and that the user cannot mount with old credentials",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"pinweaver"},
	})
}

func RemoveCredentialAuthSession(ctx context.Context, s *testing.State) {
	const (
		userName                              = "foo@bar.baz"
		userPassword                          = "secret"
		updatedPassword                       = "updatedsecret"
		wrongPassword                         = "wrongPassword"
		keyLabel                              = "fake_label"
		userPin                               = "123456"
		updatedPin                            = "098765"
		pinLabel                              = "pin"
		wrongPin                              = "000000"
		cryptohomeErrorAuthorizationKeyFailed = 3
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
	if err := cryptohome.CreateUserWithAuthSession(ctx, userName, userPassword, keyLabel, false); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)

	// Step 2: Mount the user vault for the first time. Authenticate with AuthSession
	// first, and then use the same AuthSession for preparing the vault.
	authSessionID, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, keyLabel, false, false)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Step 3 & 4: Write a test file to verify persistence later.
	if err := cryptohome.WriteFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	// Step 5: Add Pin credentials for the user. After this the user has a password and
	// a pin configured to mount.
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

	// Step 8: Unmount everything. We will remount with the updated password
	// credentials.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Step 9: Check we can use login with pin. This also checks if wrong pin
	// attempt fails.
	_, authSessionID, err = client.StartAuthSession(ctx, userName /*isEphemeral*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start Auth session: ", err)
	}

	if err = client.AuthenticatePinWithAuthSession(ctx, userPin, pinLabel, authSessionID); err != nil {
		s.Fatal("Failed to authenticate with pin: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault with given credential: ", err)
	}

	// Step 10: Remove pin key, and test everything is alright after removing the key.
	if err := client.RemoveVaultKey(ctx, userName, userPassword, pinLabel); err != nil {
		s.Fatal("Failed to remove pin for the user: ", err)
	}
	client.InvalidateAuthSession(ctx, authSessionID)

	// Step 11: Check key should fail after remove, even when AuthSession was started
	//  with pin keyset.
	accepted, err := client.CheckVault(ctx, pinLabel, hwsec.NewPassAuthConfig(userName, userPin))
	if accepted || err == nil {
		s.Fatal("Wrong pin check succeeded when it shouldn't: ", err)
	}

	// Step 12: Unmount everything.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	_, authSessionID, err = client.StartAuthSession(ctx, userName /*isEphemeral*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start Auth session: ", err)
	}

	// Step 13: Attempt to login with removed pin, this attempt should fail.
	err = client.AuthenticatePinWithAuthSession(ctx, userPin, pinLabel, authSessionID)
	if err == nil {
		s.Fatal("Successfully authenticated with pin when it should have failed: ", err)
	}

	// Step 14: Check we can still login with password. This should succeed.
	if err = client.AuthenticateAuthSession(ctx, userPassword, keyLabel, authSessionID /*kiosk_mount=*/, false); err != nil {
		s.Fatal("Failed to authenticate with pin: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault with given credential: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Step 15: Verify that file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to verify file persistence: ", err)
	}
}
