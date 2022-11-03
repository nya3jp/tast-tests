// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"os"
	"path/filepath"
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
		Func: RemoveVaultKeysetAuthFactor,
		Desc: "Test RemoveAuthFactor API on the vault keyset based auth factors",
		Contacts: []string{
			"lziest@google.com",
			"cryptohome-core@google.com",
		},
		Attr:    []string{"group:mainline", "informational"},
		Fixture: "ussAuthSessionFixture",
	})
}

// RemoveVaultKeysetAuthFactor tests that RemoveAuthFactor works as expected
// when the auth factors are all vault keyset based.
func RemoveVaultKeysetAuthFactor(ctx context.Context, s *testing.State) {
	const (
		userName      = "foo@bar.baz"
		userPassword  = "secret"
		passwordLabel = "gaia"
		userPin       = "123456"
		pinLabel      = "test-pin"
		keysetFile    = "master.0" // nocheck
		ussFile       = "user_secret_stash/uss"
	)

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)

	// Mount user cryptohome for test user with the legacy interface.
	// This represents the existing user with existing VaultKeysets
	if err := cryptohome.CreateVault(ctx, userName, userPassword); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}
	defer client.UnmountAndRemoveVault(ctx, userName)

	// Make sure one vault keyset exists before authenticating.
	// and there is no USS file.
	sr, err := client.GetUserShadowRoot(ctx, userName)
	if err != nil {
		s.Fatal("Failed to get user shadow root: ", err)
	}
	if _, err := os.Stat(filepath.Join(sr, keysetFile)); err != nil {
		s.Fatal("Keyset file not found: ", err)
	}

	// Authenticate a new auth session via the password auth factor.
	authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral=*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}

	// Removing the only password factor should fail
	if err := client.RemoveAuthFactor(ctx, authSessionID, passwordLabel); err == nil {
		s.Fatal("Should fail RemoveAuthFactor() when the factor is the last one left")
	}

	// Add a pin auth factor to the user and then remove the pin factor.
	if err := client.AddAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
		s.Fatal("AddAuthFactor() failed when adding a pin auth factor")
	}
	if err := client.RemoveAuthFactor(ctx, authSessionID, pinLabel); err != nil {
		s.Fatal("RemoveAuthFactor() failed when removing a pin auth factor")
	}
}
