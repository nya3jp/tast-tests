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
		Func: KioskWithUSS,
		Desc: "Ensures that cryptohome correctly mounts kiosk sessions through various Auth APIs",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

// KioskWithUSS tests the following case:
// User Created with AuthFactor API and USS:
// Ensure that the user can login with AuthFactor APIs with USS Enabled
func KioskWithUSS(ctx context.Context, s *testing.State) {
	const (
		cleanupTime = 20 * time.Second
	)

	ctxForCleanUp := ctx
	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	// Unmount all user vaults before we start.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}

	// Ensure the vault had been removed.
	if err := cryptohome.RemoveVault(ctx, cryptohome.KioskUser); err != nil {
		s.Log("Failed to remove vault before test starts: ", err)
	}

	cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
	if err != nil {
		s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
	}
	defer cleanupUSSExperiment()

	// Authenticate a new auth session, create the user, mount the vault
	// and add kiosk credential.
	authSessionID, err := client.StartAuthSession(ctx, cryptohome.KioskUser /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	defer client.InvalidateAuthSession(ctx, authSessionID)

	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, cryptohome.KioskUser)

	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to prepare new persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	if err := client.AddKioskAddKioskAuthFactor(ctx, authSessionID); err != nil {
		s.Fatal("Failed to add kiosk credentials: ", err)
	}

	if err := cryptohome.WriteFileForPersistence(ctx, cryptohome.KioskUser); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	if err := client.InvalidateAuthSession(ctx, authSessionID); err != nil {
		s.Fatal("Failed to invalidate session: ", err)
	}

	// Unmount all user vaults before we start.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}

	// Authenticate a new auth session, create the user, mount the vault
	// and add kiosk credential.
	authSessionID, err = client.StartAuthSession(ctx, cryptohome.KioskUser, false /*ephemeral*/)
	if err != nil {
		s.Fatal("Failed to start auth session for re-mounting: ", err)
	}
	defer client.InvalidateAuthSession(ctx, authSessionID)

	if err = client.AuthenticateKioskAuthFactor(ctx, "", "public_mount", authSessionID, true /*=isKioskUser*/); err != nil {
		s.Fatal("Failed to authenticate with auth session: ", err)
	}

	if err := cryptohome.MountAndVerify(ctx, cryptohome.KioskUser, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to mount and verify persistence: ", err)
	}
}
