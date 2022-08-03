// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

// smartCardWithAuthAPIParam contains the test parameters which are different
// between the types of backing store.
type smartCardWithAuthAPIParam struct {
	// Specifies whether to use user secret stash.
	useUserSecretStash bool
	// Specifies whether to use AuthFactor.
	// This, for now, also assumes that AuthSession would be used with AuthFactors.
	useAuthFactor bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SmartCardWithAuthAPI,
		Desc: "Checks that Smart Cards work with AuthSession, AuthFactor and USS",
		Contacts: []string{
			"thomascedeno@google.org", // Test author
			"cryptohome-core@google.com",
		},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"pinweaver", "reboot"},
		Params: []testing.Param{{
			Name: "smart_card_with_auth_factor_with_no_uss",
			Val: smartCardWithAuthAPIParam{
				useUserSecretStash: false,
				useAuthFactor:      true,
			},
		}, {
			Name: "smart_card_with_auth_session",
			Val: smartCardWithAuthAPIParam{
				useUserSecretStash: false,
				useAuthFactor:      false,
			},
		}, {
			Name: "smart_card_with_auth_factor_with_uss",
			Val: smartCardWithAuthAPIParam{
				useUserSecretStash: true,
				useAuthFactor:      true,
			},
		}},
	})
}

// Some constants used across the test.
const (
	authFactorLabelPIN       = "lecred"
	correctPINSecret         = "123456"
	incorrectPINSecret       = "000000"
	passwordAuthFactorLabel  = "fake_label"
	passwordAuthFactorSecret = "password"
	testUser1                = "testUser1@example.com"
	testUser2                = "testUser2@example.com"
	testFile                 = "file"
	testFileContent          = "content"
)

func SmartCardWithAuthAPI(ctx context.Context, s *testing.State) {
	userParam := s.Param().(smartCardWithAuthAPIParam)
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwsecremote.NewHelper(cmdRunner, s.DUT())

	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	daemonController := helper.DaemonController()

	// Wait for cryptohomed becomes available if needed.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Clean up obsolete state, in case there's any.
	cmdRunner.Run(ctx, "rm -rf /home/.shadow/low_entropy_creds")
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if _, err := client.RemoveVault(ctx, testUser1); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}
	if _, err := client.RemoveVault(ctx, testUser2); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	if userParam.useAuthFactor {
		// Enable the UserSecretStash experiment for the duration of the test by
		// creating a flag file that's checked by cryptohomed.
		cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
		if err != nil {
			s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
		}
		defer cleanupUSSExperiment()
	}

	/**Initial User Setup. Test both user 1 and user 2 can login successfully.**/
	// Setup a user 1 for testing.
	if err = setupUserWithSmartCard(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run setupUserWithSmartCard with error: ", err)
	}
	defer removeSmartCardCredential(ctx, ctxForCleanUp, testUser1, cmdRunner, helper)

	// Ensure we can authenticate with correct Smart Card.
	if err = authenticateWithCorrectSmartCard(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectSmartCard with error: ", err)
	}

	// Ensure we can authenticate with correct password.
	if err = authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPassword with error: ", err)
	}

	// Setup a user 2 for testing. This user will be removed.
	if err = setupUserWithSmartCard(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run setupUserWithSmartCard with error: ", err)
	}
	defer removeSmartCardCredential(ctx, ctxForCleanUp, testUser2, cmdRunner, helper)

	// Ensure we can authenticate with correct password for testUser2.
	if err = authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPassword with error: ", err)
	}

	// Ensure we can authenticate with correct Smart Card for testUser2.
	if err = authenticateWithCorrectSmartCard(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectSmartCard with error: ", err)
	}

	// Ensure that testUser1 still works wth Smart Card.
	if err = authenticateWithCorrectSmartCard(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectSmartCard with error: ", err)
	}

	// Ensure that testUser1 still works wth password.
	if err = authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPassword with error: ", err)
	}

	// Attempt wrong Smart Card authentication four times.
	if err = attemptWrongSmartCard(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, 4 /*attempts*/); err != nil {
		s.Fatal("Failed to run attemptWrongSmartCard with error: ", err)
	}

	// Ensure that testUser1 still works wth Smart Card.
	if err = authenticateWithCorrectSmartCard(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectSmartCard with error: ", err)
	}

	/** Ensure that testUser2 can still use Smart Card **/
	if err = authenticateWithCorrectSmartCard(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectSmartCard with error: ", err)
	}

	// Remove the added Smart Card.
	if err = removeSmartCardCredential(ctx, ctxForCleanUp, testUser2, cmdRunner, helper); err != nil {
		s.Fatal("Failed to run removeSmartCardCredential with error: ", err)
	}

	/** Ensure test user 1 can still login with Smart Card**/
	if err = authenticateWithCorrectSmartCard(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectSmartCard with error: ", err)
	}
}

// setupUserWithSmartCard sets up a user with a password and a Smart Card auth factor.
func setupUserWithSmartCard(ctx, ctxForCleanUp context.Context, userName string, cmdRunner *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, userParam smartCardWithAuthAPIParam) error {
	cryptohomeHelper := helper.CryptohomeClient()

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, userName, false /*ephemeral*/)
	if err != nil {
		return errors.Wrap(err, "failed to start auth session for Smart Card authentication")
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctx, authSessionID)

	if err = cryptohomeHelper.CreatePersistentUser(ctx, authSessionID); err != nil {
		return errors.Wrap(err, "failed to create persistent user with auth session")
	}

	if err = cryptohomeHelper.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		return errors.Wrap(err, "failed to prepare persistent user with auth session")
	}
	defer cryptohomeHelper.Unmount(ctx, userName)

	if userParam.useAuthFactor {
		err = cryptohomeHelper.AddAuthFactor(ctx, authSessionID, passwordAuthFactorLabel, passwordAuthFactorSecret)
	} else {
		err = cryptohomeHelper.AddCredentialsWithAuthSession(ctx, userName, passwordAuthFactorSecret, passwordAuthFactorLabel, authSessionID, false /*kiosk*/)
	}

	if err != nil {
		return errors.Wrap(err, "failed to add password auth factor")
	}

	// Add a Smart Card auth factor to the user.
	if userParam.useAuthFactor {
		err = cryptohomeHelper.AddSmartCardAuthFactor(ctx, authSessionID, authFactorLabelPIN, correctPINSecret)
	} else {
		err = cryptohomeHelper.AddChallengeCredentialsWithAuthSession(ctx, authFactorLabelPIN, correctPINSecret, authSessionID)
	}
	if err != nil {
		return errors.Wrap(err, "failed to add smart card credential")
	}
	return nil
}

// attemptWrongSmartCard attempts to try wrong Smart Card for authentication for given number of attempts.
func attemptWrongSmartCard(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, userParam smartCardWithAuthAPIParam, numberOfWrongAttempts int) error {
	cryptohomeHelper := helper.CryptohomeClient()

	// Authenticate a new auth session via the new added Smart Card auth factor.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser, false /*ephemeral*/)
	if err != nil {
		return errors.Wrap(err, "failed to start auth session for Smart Card authentication")
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Supply invalid credentials five times to trigger firmware lockout of the credential.
	for i := 0; i < numberOfWrongAttempts; i++ {
		if userParam.useAuthFactor {
			err = cryptohomeHelper.AuthenticateSmartCardAuthFactor(ctx, authSessionID, authFactorLabelPIN, incorrectPINSecret)
		} else {
			err = cryptohomeHelper.AuthenticateChallengeCredentialWithAuthSession(ctx, incorrectPINSecret, authFactorLabelPIN, authSessionID)
		}
		if err == nil {
			return errors.Wrap(err, "authentication with wrong Smart Card succeeded unexpectedly")
		}
	}
	return nil
}

// authenticateWithCorrectSmartCard authenticates a given user with the correct Smart Card.
func authenticateWithCorrectSmartCard(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, userParam smartCardWithAuthAPIParam, shouldAuthenticate bool) error {
	cryptohomeHelper := helper.CryptohomeClient()

	// Authenticate a new auth session via the new added Smart Card auth factor.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser, false /*ephemeral*/)
	if err != nil {
		return errors.Wrap(err, "failed to start auth session for Smart Card authentication")
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if userParam.useAuthFactor {
		err = cryptohomeHelper.AuthenticateSmartCardAuthFactor(ctx, authSessionID, authFactorLabelPIN, correctPINSecret)
	} else {
		err = cryptohomeHelper.AuthenticateChallengeCredentialWithAuthSession(ctx, correctPINSecret, authFactorLabelPIN, authSessionID)
	}
	if (err == nil) != shouldAuthenticate {
		return errors.Wrapf(err, "failed to authenticated auth factor with correct Smart Card. got %v, want %v", (err == nil), shouldAuthenticate)
	}
	return nil
}

// authenticateWithCorrectPassword authenticates a given user with the correct password.
func authenticateWithCorrectPassword(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, userParam smartCardWithAuthAPIParam) error {
	cryptohomeHelper := helper.CryptohomeClient()

	// Authenticate a new auth session via the new password auth factor and mount the user.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser, false /*ephemeral*/)
	if err != nil {
		return errors.Wrap(err, "failed to start auth session for password authentication")
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Authenticate with correct password.
	if userParam.useAuthFactor {
		err = cryptohomeHelper.AuthenticateAuthFactor(ctx, authSessionID, passwordAuthFactorLabel, passwordAuthFactorSecret)
	} else {
		err = cryptohomeHelper.AuthenticateAuthSession(ctx, passwordAuthFactorSecret, passwordAuthFactorLabel, authSessionID, false /*kiosk_mount*/)
	}
	if err != nil {
		return errors.Wrap(err, "failed to authenticated auth factor with correct password")
	}

	return nil
}

// removeSmartCardCredential removes testUser.
func removeSmartCardCredential(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote) error {
	cryptohomeHelper := helper.CryptohomeClient()

	if _, err := cryptohomeHelper.RemoveVault(ctx, testUser); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}
