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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AuthSessionUnlock,
		Desc: "Check session unlock via AuthSession",
		Contacts: []string{
			"emaxx@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:    "with_vk",
			Fixture: "vkAuthSessionFixture",
		}, {
			Name:    "with_uss",
			Fixture: "ussAuthSessionFixture",
		}},
	})
}

func AuthSessionUnlock(ctx context.Context, s *testing.State) {
	const (
		ownerName       = "owner@bar.baz"
		userName        = "foo@bar.baz"
		userPassword    = "secret"
		newUserPassword = "i-forgot-secret"
		secondUserName  = "doo@bar.baz"
		secondPassword  = "different-secret"
		passwordLabel   = "online-password"
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

	// Wait for cryptohomed to become available if needed.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Clean up old state or mounts for the test user, if any exists.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}
	if err := cryptohome.RemoveVault(ctx, secondUserName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Create and mount the user with a password auth factor.
	if err := client.WithAuthSession(ctx, userName, false /*isEphemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT, func(authSessionID string) error {
		if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
			return errors.Wrap(err, "failed to create persistent user")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			return errors.Wrap(err, "failed to prepare new persistent vault")
		}
		if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
			return errors.Wrap(err, "failed to add initial user password")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to create and set up the user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)

	// Create and mount the user with a second password auth factor.
	if err := client.WithAuthSession(ctx, secondUserName, false /*isEphemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT, func(authSessionID string) error {
		if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
			return errors.Wrap(err, "failed to create persistent user")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			return errors.Wrap(err, "failed to prepare new persistent vault")
		}
		if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, secondPassword); err != nil {
			return errors.Wrap(err, "failed to add initial user password")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to create and set up the second user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, secondUserName)

	// Verify that the user passwords can be used to authenticate.
	if err := client.WithAuthSession(ctx, userName, false /*isEphemeral*/, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY, func(authSessionID string) error {
		var authReply *uda.AuthenticateAuthFactorReply
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, newUserPassword); err == nil {
			return errors.New("authenticated user with the wrong password")
		}
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, secondPassword); err == nil {
			return errors.New("authenticated user with the other user's password")
		}
		if authReply, err = client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
			return errors.Wrap(err, "failed to authenticate user")
		}
		if err := cryptohomecommon.ExpectContainsAuthIntent(
			authReply.AuthorizedFor, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY,
		); err != nil {
			return errors.Wrap(err, "unexpected AuthSession authorized intents")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to authenticate first user with initial password: ", err)
	}
	if err := client.WithAuthSession(ctx, secondUserName, false /*isEphemeral*/, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY, func(authSessionID string) error {
		var authReply *uda.AuthenticateAuthFactorReply
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, newUserPassword); err == nil {
			return errors.New("authenticated user with the wrong password")
		}
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err == nil {
			return errors.New("authenticated user with the other user's password")
		}
		if authReply, err = client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, secondPassword); err != nil {
			return errors.Wrap(err, "failed to authenticate user")
		}
		if err := cryptohomecommon.ExpectContainsAuthIntent(
			authReply.AuthorizedFor, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY,
		); err != nil {
			return errors.Wrap(err, "unexpected AuthSession authorized intents")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to authenticate second user: ", err)
	}

	// Change the user's password.
	if err := client.WithAuthSession(ctx, userName, false /*isEphemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT, func(authSessionID string) error {
		var authReply *uda.AuthenticateAuthFactorReply
		if authReply, err = client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
			return errors.Wrap(err, "failed to authenticate user")
		}
		if err := cryptohomecommon.ExpectContainsAuthIntent(
			authReply.AuthorizedFor, uda.AuthIntent_AUTH_INTENT_DECRYPT,
		); err != nil {
			return errors.Wrap(err, "unexpected AuthSession authorized intents")
		}
		if err := client.UpdatePasswordAuthFactor(ctx, authSessionID, passwordLabel, passwordLabel, newUserPassword); err != nil {
			return errors.Wrap(err, "failed to update password")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to change the user password: ", err)
	}

	// Verify the new password can be used to authenticate.
	if err := client.WithAuthSession(ctx, userName, false /*isEphemeral*/, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY, func(authSessionID string) error {
		var authReply *uda.AuthenticateAuthFactorReply
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err == nil {
			return errors.New("authenticated user with the old password")
		}
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, secondPassword); err == nil {
			return errors.New("authenticated user with the other user's password")
		}
		if authReply, err = client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, newUserPassword); err != nil {
			return errors.Wrap(err, "failed to authenticate user")
		}
		if err := cryptohomecommon.ExpectContainsAuthIntent(
			authReply.AuthorizedFor, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY,
		); err != nil {
			return errors.Wrap(err, "unexpected AuthSession authorized intents")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to authenticate with changed password: ", err)
	}
}
