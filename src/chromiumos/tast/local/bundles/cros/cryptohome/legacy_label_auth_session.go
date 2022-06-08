// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LegacyLabelAuthSession,
		Desc: "Test AuthSession with a cryptohome created via legacy APIs without key labels",
		Contacts: []string{
			"emaxx@chromium.org", // Test authors
			"cryptohome-core@google.com",
		},
		Attr:    []string{"informational", "group:mainline"},
		Timeout: 60 * time.Second,
	})
}

func LegacyLabelAuthSession(ctx context.Context, s *testing.State) {
	const (
		userName       = "foo@bar.baz"
		userPassword   = "secret"
		wrongPassword  = "wrong-password"
		legacyKeyLabel = "legacy-0"
		wrongKeyLabel  = "gaia"
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	// Ensure cryptohomed is started and wait for it to be available.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	client := hwsec.NewCryptohomeClient(cmdRunner)
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Create persistent user with a vault keyset that has an empty label.
	authConfig := hwsec.NewPassAuthConfig(userName, userPassword)
	vaultConfig := hwsec.NewVaultConfig()
	vaultConfig.CreateEmptyLabel = true
	// Pass a random label as Cryptohome would refuse a request if none was passed.
	// CreateEmptyLabel takes precedence over this label when performing the actual mount.
	if err := client.MountVault(ctx /* keyLabel= */, "label not needed", authConfig /*create=*/, true, vaultConfig); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cryptohome.RemoveVault(cleanupCtx, userName)

	keys, err := client.ListVaultKeys(ctx, userName)
	if err != nil {
		s.Fatal("Failed to list keys: ", err)
	}
	if len(keys) != 1 || keys[0] != legacyKeyLabel {
		s.Fatal("Unexpected keys: ", keys)
	}

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Verify authenticate fails if no or incorrect label is passed.
	if _, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword /* keyLabel= */, "", false, false); err == nil {
		s.Fatal("Authentication with empty label succeeded unexpectedly")
	}
	if _, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, wrongKeyLabel, false, false); err == nil {
		s.Fatal("Authentication with incorrect label succeeded unexpectedly")
	}

	// Verify authentication succeeds if the legacy label is explicitly passed.
	authSessionID, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, legacyKeyLabel, false, false)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(cleanupCtx, authSessionID)

	// Verify mounting succeeds.
	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer client.UnmountAll(cleanupCtx)

	// Verify lock-screen check passes with an explicit or wildcard label.
	accepted, err := client.CheckVault(ctx, legacyKeyLabel, hwsec.NewPassAuthConfig(userName, userPassword))
	if err != nil {
		s.Fatal("Failed to check key with explicit label: ", err)
	}
	if !accepted {
		s.Fatal("Key check with explicit label failed despite no error")
	}
	accepted, err = client.CheckVault(ctx /*keyLabel=*/, "", hwsec.NewPassAuthConfig(userName, userPassword))
	if err != nil {
		s.Fatal("Failed to check key with wildcard label: ", err)
	}
	if !accepted {
		s.Fatal("Key check with wildcard label failed despite no error")
	}

	// Verify lock-screen check fails with wrong password or wrong label.
	accepted, err = client.CheckVault(ctx, wrongKeyLabel, hwsec.NewPassAuthConfig(userName, userPassword))
	if err == nil {
		s.Fatal("Key check with wrong label succeeded when it shouldn't")
	}
	if accepted {
		s.Fatal("Key check with wrong label succeeded despite an error")
	}
	accepted, err = client.CheckVault(ctx, legacyKeyLabel, hwsec.NewPassAuthConfig(userName, wrongPassword))
	if err == nil {
		s.Fatal("Key check with wrong password succeeded when it shouldn't")
	}
	if accepted {
		s.Fatal("Key check with wrong password succeeded despite an error")
	}
}
