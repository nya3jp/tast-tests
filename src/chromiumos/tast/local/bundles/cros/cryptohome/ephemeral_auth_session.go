// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: EphemeralAuthSession,
		Desc: "Test ephemeral sessions with auth session API",
		Contacts: []string{
			"dlunev@chromium.org",
			"hardikgoyal@chromium.org",
			"cryptohome-core@chromium.org",
		},
		Attr: []string{"group:mainline", "informational"},
		Data: []string{"testcert.p12"},
	})
}

func EphemeralAuthSession(ctx context.Context, s *testing.State) {
	const (
		ownerUser       = "owner@owner.owner"
		userName        = "foo@bar.baz"
		userPassword    = "secret"
		testFile        = "file"
		testFileContent = "content"
	)

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

	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	// Set up the first user as the owner. It is required to mount ephemeral.
	if err := hwseclocal.SetUpVaultAndUserAsOwner(ctx, s.DataPath("testcert.p12"), ownerUser, "whatever", "whatever", helper.CryptohomeClient()); err != nil {
		cryptohome.UnmountAll(ctx)
		cryptohome.RemoveVault(ctx, ownerUser)
		s.Fatal("Failed to setup vault and user as owner: ", err)
	}
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	defer cryptohome.RemoveVault(ctx, ownerUser)

	// Set up an ephemeral session.
	authSessionID, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, true, false)
	if err != nil {
		s.Fatal("Failed to authenticate ephemeral user: ", err)
	}
	defer cryptohome.InvalidateAuthSession(ctx, authSessionID)

	if err := cryptohome.PrepareEphemeralVault(ctx, authSessionID); err != nil {
		s.Fatal("Failed to prepare ephemeral vault: ", err)
	}
	defer cryptohome.UnmountAll(ctx)

	if err := cryptohome.PrepareEphemeralVault(ctx, authSessionID); err == nil {
		s.Fatal("Secondary prepare attempt for the same user should fail, but succeeded")
	}

	// Write a test file to verify non-persistence.
	userPath, err := cryptohome.UserPath(ctx, userName)
	if err != nil {
		s.Fatal("Failed to get user vault path: ", err)
	}

	filePath := filepath.Join(userPath, testFile)
	if err := ioutil.WriteFile(filePath, []byte(testFileContent), 0644); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	// Unmount and mount again.
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	authSessionID, err = cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, true, false)
	if err != nil {
		s.Fatal("Failed to authenticate ephemeral user: ", err)
	}
	defer cryptohome.InvalidateAuthSession(ctx, authSessionID)

	if err := cryptohome.PrepareEphemeralVault(ctx, authSessionID); err != nil {
		s.Fatal("Failed to prepare ephemeral vault: ", err)
	}

	// Verify non-persistentce.
	if _, err := ioutil.ReadFile(filePath); err == nil {
		s.Fatal("File is persisted across ephemeral session boundary")
	}
}
