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
	cryptohomecommon "chromiumos/tast/common/cryptohome"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PasswordAuthFactorExistingVaultKeyset,
		Desc: "Test AuthFactor API basic password flow when USS experiment is enabled but there is an existing VaultKeysets",
		Contacts: []string{
			"betuls@google.com",
			"cryptohome-core@google.com",
		},
		Attr:    []string{"group:mainline", "informational"},
		Fixture: "ussAuthSessionFixture",
	})
}

// PasswordAuthFactorExistingVaultKeyset tests that AuthenticateAuthFactor uses
// existing VaultKeyset to authenticate an existing user even if USS experiment
// is enabled.
func PasswordAuthFactorExistingVaultKeyset(ctx context.Context, s *testing.State) {
	const (
		userName      = "foo@bar.baz"
		userPassword  = "secret"
		passwordLabel = "gaia"
		ussFlagFile   = "/var/lib/cryptohome/uss_enabled"
		shadow        = "/home/.shadow"
		keysetFile    = "master.0" // nocheck
		ussFile       = "user_secret_stash/uss"
	)

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Mount user cryptohome for test user with the legacy interface.
	// This represents the existing user with existing VaultKeysets
	if err := cryptohome.CreateVault(ctx, userName, userPassword); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}
	defer cryptohome.RemoveVault(ctx, userName)

	// Unmount user vault directory and daemon-store directories.
	if err := cryptohome.UnmountVault(ctx, userName); err != nil {
		s.Error("Failed to unmount user vault: ", err)
	}

	// Authenticate a new auth session via the auth factor and mount the user.
	_, authSessionID, err := client.StartAuthSession(ctx, userName /*ephemeral=*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}

	// Make sure one vault keyset exists before authenticating.
	sr, err := client.GetUserShadowRoot(ctx, userName)
	if err != nil {
		s.Fatal("Failed to get user shadow root: ", err)
	}
	if _, err := os.Stat(filepath.Join(sr, keysetFile)); err != nil {
		s.Fatal("Keyset file not found: ", err)
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
	defer cryptohome.UnmountVault(ctx, userName)

	// Check that USS file is not created
	if _, err = os.Stat(filepath.Join(sr, ussFile)); err == nil {
		s.Fatal("USS file created when there is vault keyset: ", err)
	}
}
