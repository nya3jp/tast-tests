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
	"chromiumos/tast/testing/hwdep"
)

// Parameters that control test behavior.
type updatePinParams struct {
	// Specifies whether to use user secret stash.
	useUserSecretStash bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: UpdatePin,
		Desc: "Update pin auth factor and authenticate with the new pin",
		Contacts: []string{
			"anastasiian@chromium.org",
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		// TODO(b/195385797): Run on gooey when the bug is fixed.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("gooey")),
		SoftwareDeps: []string{"pinweaver"},
		Params: []testing.Param{{
			Name: "with_vk",
			Val: updatePinParams{
				useUserSecretStash: false,
			},
		}, {
			Name: "with_uss",
			Val: updatePinParams{
				useUserSecretStash: true,
			},
		}},
	})
}

func UpdatePin(ctx context.Context, s *testing.State) {
	const (
		userName                         = "foo@bar.baz"
		userPassword                     = "secret"
		oldUserPin                       = "123456"
		newUserPin                       = "098765"
		passwordLabel                    = "online-password"
		pinLabel                         = "test-pin"
		cryptohomeAuthorizationKeyFailed = 3
	)

	userParam := s.Param().(updatePinParams)
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
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Clean up obsolete state, in case there's any.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if err := cryptohome.RemoveVault(ctx, userName); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	if userParam.useUserSecretStash {
		// Enable the UserSecretStash experiment for the duration of the test by
		// creating a flag file that's checked by cryptohomed.
		cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
		if err != nil {
			s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
		}
		defer cleanupUSSExperiment(ctxForCleanUp)
	}

	// Create and mount the persistent user.
	_, authSessionID, err := client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		s.Fatal("Failed to start auth session: ", err)
	}
	if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, userName)
	if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, userName); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	authenticatePasswordAuthFactor := func(password string) (string, error) {
		// Authenticate a new auth session via the new added pin auth factor and mount the user.
		_, authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return "", errors.Wrap(err, "failed to start auth session for re-mounting")
		}
		authReply, err := client.AuthenticateAuthFactor(ctx, authSessionID, passwordLabel, password)
		if err != nil {
			return authSessionID, errors.Wrap(err, "failed to authenticate with auth session")
		}
		if !authReply.Authenticated {
			return authSessionID, errors.New("AuthSession not authenticated despite successful reply")
		}
		if err := cryptohomecommon.ExpectAuthIntents(authReply.AuthorizedFor, []uda.AuthIntent{
			uda.AuthIntent_AUTH_INTENT_DECRYPT,
			uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY,
		}); err != nil {
			return authSessionID, errors.Wrap(err, "unexpected AuthSession authorized intents")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			return authSessionID, errors.Wrap(err, "failed to prepare persistent vault")
		}

		// Verify that file is still there.
		if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
			return authSessionID, errors.Wrap(err, "failed to verify test file")
		}
		return authSessionID, nil
	}

	authenticatePinAuthFactor := func(pin string) (string, error) {
		// Authenticate a new auth session via the new added pin auth factor and mount the user.
		_, authSessionID, err = client.StartAuthSession(ctx, userName, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return "", errors.Wrap(err, "failed to start auth session for re-mounting")
		}
		if err := client.AuthenticatePinAuthFactor(ctx, authSessionID, pinLabel, pin); err != nil {
			return authSessionID, errors.Wrap(err, "failed to authenticate with auth session")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			return authSessionID, errors.Wrap(err, "failed to prepare persistent vault")
		}

		// Verify that file is still there.
		if err := cryptohome.VerifyFileForPersistence(ctx, userName); err != nil {
			return authSessionID, errors.Wrap(err, "failed to verify test file")
		}
		return authSessionID, nil
	}

	// Add a password auth factor to the user.
	if err := client.AddAuthFactor(ctx, authSessionID, passwordLabel, userPassword); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}

	// Add a pin auth factor to the user.
	if err := client.AddPinAuthFactor(ctx, authSessionID, pinLabel, oldUserPin); err != nil {
		s.Fatal("Failed to add pin auth factor: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Authenticate via password.
	authSessionID, err = authenticatePasswordAuthFactor(userPassword)
	if err != nil {
		s.Fatal("Failed to authenticate with pin authfactor: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Update pin auth factor.
	if err := client.UpdatePinAuthFactor(ctx, authSessionID, pinLabel /*label*/, newUserPin); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Unmount the user.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	// Authentication with old pin fails.
	authSessionID, err = authenticatePinAuthFactor(oldUserPin)
	var authExitErr *hwsec.CmdExitError
	if !errors.As(err, &authExitErr) {
		s.Fatalf("Unexpected error for authentication with old pin: got %q; want *hwsec.CmdExitError", err)
	}
	if authExitErr.ExitCode != cryptohomeAuthorizationKeyFailed {
		s.Fatalf("Unexpected exit code for authentication with old pin: got %d; want %d",
			authExitErr.ExitCode, cryptohomeAuthorizationKeyFailed)
	}

	// Successfully authenticate with new pin.
	if _, err := authenticatePinAuthFactor(newUserPin); err != nil {
		s.Fatal("Failed to authenticate with new pin after update: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)
}
