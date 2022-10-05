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
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UserSecretStashAddPin,
		Desc: "Test user secret stash basic add pin flow with password",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"pinweaver", "reboot"},
		Fixture:      "ussAuthSessionFixture",
	})
}

func UserSecretStashAddPin(ctx context.Context, s *testing.State) {
	const (
		userName      = "foo@bar.baz"
		userPassword  = "secret"
		userPin       = "123456"
		passwordLabel = "online-password"
		pinLabel      = "test-pin"
	)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)

	// Create and mount the persistent user.
	authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral=*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
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
		s.Fatal("Failed to add a password authfactor: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Authenticate a new auth session via the auth factor, mount the user and add a pin.
	authSessionID, err = client.StartAuthSession(ctx, userName /*ephemeral=*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	authReply, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword)
	if err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}
	if !authReply.Authenticated {
		s.Fatal("AuthSession not authenticated despite successful reply")
	}
	if err := cryptohomecommon.ExpectAuthIntents(authReply.AuthorizedFor, []uda.AuthIntent{
		uda.AuthIntent_AUTH_INTENT_DECRYPT,
		uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY,
	}); err != nil {
		s.Fatal("Unexpected AuthSession authorized intents: ", err)
	}
	if err := client.PreparePersistentVault(ctx, authSessionID /*ecryptfs=*/, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}

	// Verify that the test file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to verify file persistence: ", err)
	}

	// Add a pin auth factor to the user.
	if err := client.AddPinAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Authenticate a new auth session via the new added pin auth factor and mount the user.
	authSessionID, err = client.StartAuthSession(ctx, userName /*ephemeral=*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	if _, err := client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}
	if err := client.PreparePersistentVault(ctx, authSessionID /*ecryptfs=*/, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}

	// Verify that the test file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to verify file persistence: ", err)
	}
}
