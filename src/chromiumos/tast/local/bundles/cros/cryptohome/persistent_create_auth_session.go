// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PersistentCreateAuthSession,
		Desc: "Test AuthSession with a new flow where we create before authenticate",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func PersistentCreateAuthSession(ctx context.Context, s *testing.State) {
	const (
		userName                              = "foo@bar.baz"
		userPassword                          = "secret"
		keyLabel                              = "foo"
		wrongPassword                         = "wrongPassword"
		cryptohomeErrorAuthorizationKeyFailed = 3
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

	if err := cryptohome.CreateAndMountUserWithAuthSession(ctx, userName, userPassword, keyLabel, false); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)

	if err := cryptohome.TestLockScreen(ctx, userName, userPassword, wrongPassword, keyLabel, client); err != nil {
		s.Fatal("Failed to check lock screen: ", err)
	}

	if err := cryptohome.WriteFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	// Unmount and mount again.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Authenticate with the wrong password.
	authSessionID, err := cryptohome.AuthenticateWithAuthSession(ctx, userName, wrongPassword, keyLabel, false, false)
	var exitErr *hwsec.CmdExitError
	if !errors.As(err, &exitErr) {
		s.Fatalf("Unexpected error: got %q; want *hwsec.CmdExitError", err)
	}
	if exitErr.ExitCode != cryptohomeErrorAuthorizationKeyFailed {
		s.Fatalf("Unexpected exit code: got %d; want %d", exitErr.ExitCode, cryptohomeErrorAuthorizationKeyFailed)
	}

	authSessionID, err = cryptohome.AuthenticateWithAuthSession(ctx, userName, userPassword, keyLabel, false, false)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}

	// Verify that file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to verify test file: ", err)
	}
}
