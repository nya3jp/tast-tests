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

type authSessionUnlockConfig struct {
	useUserSecretStash bool
}

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
			Name: "with_vk",
			Val: authSessionUnlockConfig{
				useUserSecretStash: false,
			},
		}, {
			Name: "with_uss",
			Val: authSessionUnlockConfig{
				useUserSecretStash: true,
			},
		}},
	})
}

func AuthSessionUnlock(ctx context.Context, s *testing.State) {
	const (
		userName      = "foo@bar.baz"
		password      = "secret"
		passwordLabel = "online-password"
	)

	config := s.Param().(authSessionUnlockConfig)

	ctxForCleanup := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()

	if config.useUserSecretStash {
		cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
		if err != nil {
			s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
		}
		defer cleanupUSSExperiment(ctxForCleanup)
	}

	// Prepare by waiting for the daemon availability and cleaning obsolete state.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}
	if err := cryptohome.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}
	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Log("Failed to remove vault before test starts: ", err)
	}

	// Create the user and verify unlock succeeds.
	cleanupUser, err := createUserWithPasswordFactor(ctx, client, userName, password, passwordLabel)
	if err != nil {
		s.Fatal("Failed to create user: ", err)
	}
	defer cleanupUser(ctxForCleanup)
	if err := testAuthSessionUnlock(ctx, client, userName, password, passwordLabel); err != nil {
		s.Fatal("Unlock failed after user creation: ", err)
	}

	// Log out, log in back and verify unlock succeeds again.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount user for remount: ", err)
	}
	if err := mountUserWithPasswordFactor(ctx, client, userName, password, passwordLabel); err != nil {
		s.Fatal("Failed to remount user: ", err)
	}
	if err := testAuthSessionUnlock(ctx, client, userName, password, passwordLabel); err != nil {
		s.Fatal("Unlock failed after user creation: ", err)
	}
}

func createUserWithPasswordFactor(ctx context.Context, client *hwsec.CryptohomeClient, userName, password, passwordLabel string) (func(context.Context), error) {
	_, authSessionID, err := client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		return nil, errors.Wrap(err, "start auth session")
	}
	defer client.InvalidateAuthSession(ctx, authSessionID)
	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		return nil, errors.Wrap(err, "create user")
	}
	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
		return nil, errors.Wrap(err, "prepare user vault")
	}
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, password); err != nil {
		return nil, errors.Wrap(err, "add password factor")
	}
	cleanup := func(ctxForCleanup context.Context) {
		client.UnmountAll(ctxForCleanup)
		cryptohome.RemoveVault(ctxForCleanup, userName)
	}
	return cleanup, nil
}

func mountUserWithPasswordFactor(ctx context.Context, client *hwsec.CryptohomeClient, userName, password, passwordLabel string) error {
	_, authSessionID, err := client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		return errors.Wrap(err, "start auth session")
	}
	defer client.InvalidateAuthSession(ctx, authSessionID)
	if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, password); err != nil {
		return errors.Wrap(err, "authenticate using correct password")
	}
	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ephemeral*/); err != nil {
		return errors.Wrap(err, "prepare persistent vault")
	}
	return nil
}

func testAuthSessionUnlock(ctx context.Context, client *hwsec.CryptohomeClient, userName, password, passwordLabel string) error {
	// Check VERIFY_ONLY authentication using the correct password.
	_, authSessionID, err := client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY)
	if err != nil {
		return errors.Wrap(err, "start AuthSession")
	}
	authReply, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, password)
	if err != nil {
		return errors.Wrap(err, "authenticate with correct password")
	}
	defer client.InvalidateAuthSession(ctx, authSessionID)

	if err := cryptohomecommon.ExpectAuthIntents(authReply.AuthorizedFor, []uda.AuthIntent{
		uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY,
	}); err != nil {
		return errors.Wrap(err, "unexpected AuthSession authorized intents")
	}

	// Check VERIFY_ONLY authentication fails when using a wrong password.
	_, authSessionID, err = client.StartAuthSession(ctx, userName /*ephemeral=*/, false, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY)
	if err != nil {
		return errors.Wrap(err, "start second AuthSession")
	}
	if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, "wrong-secret"); err == nil {
		return errors.Wrap(err, "unexpectedly authenticated using a wrong password")
	}
	defer client.InvalidateAuthSession(ctx, authSessionID)

	return nil
}
