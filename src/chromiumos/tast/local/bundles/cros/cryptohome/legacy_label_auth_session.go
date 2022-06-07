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
		Attr: []string{"informational", "group:mainline"},
	})
}

func LegacyLabelAuthSession(ctx context.Context, s *testing.State) {
	const (
		userName       = "foo@bar.baz"
		userPassword   = "secret"
		legacyKeyLabel = "legacy-0"
		wrongKeyLabel  = "gaia"
	)

	ctxForCleanUp := ctx
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
	if err := client.MountVault(ctx, "label not needed" /* keyLabel */, authConfig, true /*create*/, vaultConfig); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)

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
	if _, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, "" /* keyLabel */, false, false); err == nil {
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
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Verify mounting succeeds.
	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}
}
