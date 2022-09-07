// Copyright 2022 The ChromiumOS Authors.
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
		Func: AddRemovePin,
		Desc: "Adds, removes and re-adds pin in user secret stash",
		Contacts: []string{
			"anastasiian@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"pinweaver", "reboot"},
	})
}

func AddRemovePin(ctx context.Context, s *testing.State) {
	const (
		userName                   = "foo@bar.baz"
		userPassword               = "secret"
		userPin                    = "123456"
		passwordLabel              = "online-password"
		pinLabel                   = "test-pin"
		cryptohomeErrorKeyNotFound = 15
	)

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

	// Wait for cryptohomed becomes available if needed.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed is running: ", err)
	}

	// Clean up obsolete state, in case there's any.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Enable the UserSecretStash experiment for the duration of the test by
	// creating a flag file that's checked by cryptohomed.
	cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
	if err != nil {
		s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
	}
	defer cleanupUSSExperiment(ctx)

	// Create and mount the persistent user.
	authSessionID, err := client.StartAuthSession(ctx, userName, false /*ephemeral*/)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)
	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to prepare new persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	addAndAuthenticatePinAuthFactor := func() error {
		// Add a pin auth factor to the user.
		if err := client.AddPinAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
			return errors.Wrap(err, "failed to create persistent user")
		}

		// Unmount the user.
		if err := client.UnmountAll(ctx); err != nil {
			return errors.Wrap(err, "failed to unmount vaults for re-mounting")
		}

		// Authenticate a new auth session via the new added pin auth factor and mount the user.
		authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session for re-mounting")
		}
		if err := client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, userPin); err != nil {
			return errors.Wrap(err, "failed to authenticate with auth session")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			return errors.Wrap(err, "failed to prepare persistent vault")
		}

		// Verify that file is still there.
		if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
			s.Fatal("Failed to verify test file: ", err)
		}
		return nil
	}

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to add a password authfactor: ", err)
	}

	// Can add and successfully authenticate via pin.
	if err := addAndAuthenticatePinAuthFactor(); err != nil {
		s.Fatal("Failed to add and authenticate with pin authfactor: ", err)
	}

	// Remove the pin auth factor.
	if err := client.RemoveAuthFactor(ctx, authSessionID, pinLabel); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}

	err = client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, userPin)
	var exitErr *hwsec.CmdExitError
	if !errors.As(err, &exitErr) {
		s.Fatalf("Unexpected error: got %q; want *hwsec.CmdExitError", err)
	}
	if exitErr.ExitCode != cryptohomeErrorKeyNotFound {
		s.Fatalf("Unexpected exit code: got %d; want %d", exitErr.ExitCode, cryptohomeErrorKeyNotFound)
	}

	// Can add and successfully authenticate via pin.
	if err := addAndAuthenticatePinAuthFactor(); err != nil {
		s.Fatal("Failed to add and authenticate with pin authfactor: ", err)
	}
}
