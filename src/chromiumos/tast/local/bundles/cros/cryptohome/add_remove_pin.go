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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AddRemovePIN,
		Desc: "Adds, removes and re-adds PIN with specified backing store",
		Contacts: []string{
			"anastasiian@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"pinweaver", "reboot"},
		Params: []testing.Param{{
			Name:    "with_uss",
			Fixture: "ussAuthSessionFixture",
		}, {
			Name: "with_vk",
		},
		},
	})
}

func AddRemovePIN(ctx context.Context, s *testing.State) {
	const (
		userName                   = "foo@bar.baz"
		userPassword               = "secret"
		userPIN                    = "123456"
		passwordLabel              = "online-password"
		pinLabel                   = "test-pin"
		cryptohomeErrorKeyNotFound = 15
	)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)

	// Create and mount the persistent user.
	authSessionID, err := client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
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

	addAndAuthenticatePINAuthFactor := func() error {
		// Add a PIN auth factor to the user.
		if err := client.AddPinAuthFactor(ctx, authSessionID, pinLabel, userPIN); err != nil {
			return errors.Wrap(err, "failed to create persistent user")
		}

		// Unmount the user.
		if err := client.UnmountAll(ctx); err != nil {
			return errors.Wrap(err, "failed to unmount vaults for re-mounting")
		}

		if err := client.InvalidateAuthSession(ctx, authSessionID); err != nil {
			return errors.Wrap(err, "failed to invalidate AuthSession")
		}
		// Authenticate a new auth session via the new added PIN auth factor and mount the user.
		authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session for re-mounting")
		}
		if err := client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, userPIN); err != nil {
			return errors.Wrap(err, "failed to authenticate with auth session")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			return errors.Wrap(err, "failed to prepare persistent vault")
		}

		// Verify that file is still there.
		if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
			return errors.Wrap(err, "failed to verify test file persistence")
		}
		if err := client.InvalidateAuthSession(ctx, authSessionID); err != nil {
			return errors.Wrap(err, "failed to invalidate AuthSession")
		}
		return nil
	}

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to add a password authfactor: ", err)
	}

	// Can add and successfully authenticate via PIN.
	if err := addAndAuthenticatePINAuthFactor(); err != nil {
		s.Fatal("Failed to add and authenticate with PIN authfactor: ", err)
	}

	authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to authenticate with auth session with user password: ", err)
	}
	if err := client.RemoveAuthFactor(ctx, authSessionID, pinLabel); err != nil {
		s.Fatal("Failed to remove PIN AuthFactor: ", err)
	}

	// Authentication in the same session should fail as well.
	err = client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, userPIN)
	var exitErr *hwsec.CmdExitError
	if !errors.As(err, &exitErr) {
		s.Fatalf("Unexpected error: got %q; want *hwsec.CmdExitError", err)
	}
	if exitErr.ExitCode != cryptohomeErrorKeyNotFound {
		s.Fatalf("Unexpected exit code: got %d; want %d", exitErr.ExitCode, cryptohomeErrorKeyNotFound)
	}

	// Can add and successfully authenticate via PIN.
	if err := addAndAuthenticatePINAuthFactor(); err != nil {
		s.Fatal("Failed to add and authenticate with PIN authfactor: ", err)
	}
}
