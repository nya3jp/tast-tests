// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

// pinWeaverWithAuthAPIParam contains the test parameters which are different
// between the types of backing store.
type pinWeaverWithAuthAPIParam struct {
	// Specifies whether to useUserSecretStash.
	// This, for now, also assumes that AuthSession would be used with AuthFactors.
	useUserSecretStash bool
	// For M104 AuthSession launch, pin is currently set with Legacy API.
	// Note: both these parameters cannot be true at the same time as that is not a supported case.
	useLegacyAddAPIForPin bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PINWeaverWithAuthAPI,
		Desc: "Checks that LE credentials work with AuthSession, AuthFactor and USS",
		Contacts: []string{
			"hardikgoyal@chromium.org", // Test author
			"cryptohome-core@google.com",
		},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"pinweaver", "reboot"},
		Params: []testing.Param{{
			Name: "pin_weaver_with_auth_factor",
			Val: pinWeaverWithAuthAPIParam{
				useUserSecretStash:    true,
				useLegacyAddAPIForPin: false,
			},
		}, {
			Name: "pin_weaver_with_auth_session",
			Val: pinWeaverWithAuthAPIParam{
				useUserSecretStash:    false,
				useLegacyAddAPIForPin: false,
			},
		}, {
			Name: "pin_weaver_with_auth_session_legacy_pin_add",
			Val: pinWeaverWithAuthAPIParam{
				useUserSecretStash:    false,
				useLegacyAddAPIForPin: true,
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

func PINWeaverWithAuthAPI(ctx context.Context, s *testing.State) {
	userParam := s.Param().(pinWeaverWithAuthAPIParam)
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

	supportsLE, err := client.SupportsLECredentials(ctx)
	if err != nil {
		s.Fatal("Failed to get supported policies: ", err)
	} else if !supportsLE {
		s.Fatal("Device does not support PinWeaver")
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

	if userParam.useUserSecretStash {
		// Enable the UserSecretStash experiment for the duration of the test by
		// creating a flag file that's checked by cryptohomed.
		cleanupUSSExperiment, err := helper.EnableUserSecretStash(ctx)
		if err != nil {
			s.Fatal("Failed to enable the UserSecretStash experiment: ", err)
		}
		defer cleanupUSSExperiment()
	}

	/**Initial User Setup. Test both user 1 and user 2 can login successfully.**/
	// Setup a user 1 for testing. This user will be locked out and re-authed to ensure the PIN is unlocked.
	if err = setupUserWithPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run setupUserWithPIN with error: ", err)
	}
	defer removeLeCredential(ctx, ctxForCleanUp, testUser1, cmdRunner, helper)

	if err = authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPIN with error: ", err)
	}

	// After the PIN lock out we should not be able to authenticate with correct PIN.
	if err = authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPassword with error: ", err)
	}

	// // Setup a user 2 for testing. This user will be removed and the le_credential file will be checked.
	if err = setupUserWithPIN(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run setupUserWithPIN with error: ", err)
	}
	defer removeLeCredential(ctx, ctxForCleanUp, testUser2, cmdRunner, helper)

	// // After the PIN lock out we should not be able to authenticate with correct PIN.
	if err = authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPassword with error: ", err)
	}

	if err = authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPIN with error: ", err)
	}

	// Ensure that user 1 still works.
	if err = authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPIN with error: ", err)
	}

	if err = authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPassword with error: ", err)
	}

	/** Running test where we try to almost lock out PIN with 4 attempts twice, but the user is able to log back in **/
	// Attempt four wrong PIN.
	if err = attemptWrongPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, 4 /*attempts*/); err != nil {
		s.Fatal("Failed to run attemptWrongPIN with error: ", err)
	}

	// After the PIN lock out we should not be able to authenticate with correct PIN.
	if err = authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPIN with error: ", err)
	}

	// Attempt four wrong PIN.
	if err = attemptWrongPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, 4 /*attempts*/); err != nil {
		s.Fatal("Failed to run attemptWrongPIN with error: ", err)
	}

	// After the PIN lock out we should not be able to authenticate with correct PIN.
	if err = authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPIN with error: ", err)
	}

	/** Test whether the attempt counter persists after reboot **/
	// Attempt four wrong PIN.
	if err = attemptWrongPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, 4 /*attempts*/); err != nil {
		s.Fatal("Failed to run attemptWrongPIN with error: ", err)
	}

	// Because Cr50 stores state in the firmware, that persists across reboots, this test
	// needs to run before and after a reboot.
	if err = helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to run helper with error: ", err)
	}

	// Lockout the PIN this time.
	if err = attemptWrongPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, 2 /*attempts*/); err != nil {
		s.Fatal("Failed to run attemptWrongPIN with error: ", err)
	}

	// After the PIN lock out we should not be able to authenticate with correct PIN.
	if err = authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, false /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run // with error: ", err)
	}

	if err = ensurePINLockedOut(ctx, testUser1, client); err != nil {
		s.Fatal("Failed to run ensurePINLockedOut with error: ", err)
	}

	/** Ensure that testUser2 can still use PIN **/
	// After the PIN lock out we should not be able to authenticate with correct PIN.
	if err = authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPIN with error: ", err)
	}

	/** Unlock PIN **/
	// After the PIN lock out we should not be able to authenticate with correct PIN.
	if err = authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPassword with error: ", err)
	}

	// After the PIN lock out we should not be able to authenticate with correct PIN.
	if err = authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPIN with error: ", err)
	}

	// Remove the added PIN and check to see if le_credential file was updated.
	if err = removeLeCredential(ctx, ctxForCleanUp, testUser2, cmdRunner, helper); err != nil {
		s.Fatal("Failed to run removeLeCredential with error: ", err)
	}

	/** Ensure test user 1 can still login with PIN**/
	// After the PIN lock out we should not be able to authenticate with correct PIN.
	if err = authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, true /*shouldAuthenticate*/); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectPIN with error: ", err)
	}
}

// getLeCredsFromDisk gets the LE Credential file from disk.
func getLeCredsFromDisk(ctx context.Context, r *hwsecremote.CmdRunnerRemote) ([]string, error) {
	output, err := r.Run(ctx, "/bin/ls", "/home/.shadow/low_entropy_creds")
	if err != nil {
		return nil, err
	}

	labels := strings.Split(string(output), "\n")
	sort.Strings(labels)
	return labels, nil
}

// setupUserWithPIN sets up a user with a password and a PIN auth factor.
func setupUserWithPIN(ctx, ctxForCleanUp context.Context, userName string, cmdRunner *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, userParam pinWeaverWithAuthAPIParam) error {
	cryptohomeHelper := helper.CryptohomeClient()

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, userName, false /*ephemeral*/)
	if err != nil {
		return errors.Wrap(err, "failed to start auth session for PIN authentication")
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctx, authSessionID)

	if err = cryptohomeHelper.CreatePersistentUser(ctx, authSessionID); err != nil {
		return errors.Wrap(err, "failed to create persistent user with auth session")
	}

	if err = cryptohomeHelper.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		return errors.Wrap(err, "failed to prepare persistent user with auth session")
	}
	defer cryptohomeHelper.Unmount(ctx, userName)

	if userParam.useUserSecretStash {
		err = cryptohomeHelper.AddAuthFactor(ctx, authSessionID, passwordAuthFactorLabel, passwordAuthFactorSecret)
	} else {
		err = cryptohomeHelper.AddCredentialsWithAuthSession(ctx, passwordAuthFactorLabel, passwordAuthFactorSecret, authSessionID, false /*kiosk*/)
	}

	if err != nil {
		return errors.Wrap(err, "failed to add password auth factor")
	}

	leCredsBeforeAdd, err := getLeCredsFromDisk(ctx, cmdRunner)
	if err != nil {
		return errors.Wrap(err, "failed to get le creds from disk before add")
	}

	// Add a PIN auth factor to the user.
	if userParam.useLegacyAddAPIForPin {
		err = cryptohomeHelper.AddVaultKey(ctx, userName, passwordAuthFactorSecret, passwordAuthFactorLabel, correctPINSecret, authFactorLabelPIN, true)
	} else {
		if userParam.useUserSecretStash {
			err = cryptohomeHelper.AddPinAuthFactor(ctx, authSessionID, authFactorLabelPIN, correctPINSecret)
		} else {
			err = cryptohomeHelper.AddPinCredentialsWithAuthSession(ctx, authFactorLabelPIN, correctPINSecret, authSessionID)
		}
	}
	if err != nil {
		return errors.Wrap(err, "failed to add le credential")
	}
	leCredsAfterAdd, err := getLeCredsFromDisk(ctx, cmdRunner)
	if err != nil {
		return errors.Wrap(err, "failed to get le creds from disk after add")
	}

	if diff := cmp.Diff(leCredsAfterAdd, leCredsBeforeAdd); diff == "" {
		return errors.Wrap(err, "failed to get le creds from disk after add")
	}
	return nil
}

// attemptWrongPIN attempts to try wrong PIN for authentication for given number of attempts.
func attemptWrongPIN(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, userParam pinWeaverWithAuthAPIParam, numberOfWrongAttempts int) error {
	cryptohomeHelper := helper.CryptohomeClient()

	// Authenticate a new auth session via the new added PIN auth factor and mount the user.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser, false /*ephemeral*/)
	if err != nil {
		return errors.Wrap(err, "failed to start auth session for PIN authentication")
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Supply invalid credentials five times to trigger firmware lockout of the credential.
	for i := 0; i < numberOfWrongAttempts; i++ {
		if userParam.useUserSecretStash {
			err = cryptohomeHelper.AuthenticatePinAuthFactor(ctx, authSessionID, authFactorLabelPIN, incorrectPINSecret)
		} else {
			err = cryptohomeHelper.AuthenticatePinWithAuthSession(ctx, incorrectPINSecret, authFactorLabelPIN, authSessionID)
		}
		if err == nil {
			return errors.Wrap(err, "failed to not authenticate credentials with wrong PIN")
		}
	}
	return nil
}

// authenticateWithCorrectPIN authenticates a given user with the correct PIN.
func authenticateWithCorrectPIN(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, userParam pinWeaverWithAuthAPIParam, shouldAuthenticate bool) error {
	cryptohomeHelper := helper.CryptohomeClient()

	// Authenticate a new auth session via the new added PIN auth factor and mount the user.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser, false /*ephemeral*/)
	if err != nil {
		return errors.Wrap(err, "failed to start auth session for PIN authentication")
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Here is the logic for the following if statement:
	// shouldAuthenticate | err!=nil (aka did not authenticate) | result
	// 		0						0								1 (error)
	// 		1						0								0 (expected)
	// 		0						1								0 (expected)
	// 		1						1								1 (error)
	if userParam.useUserSecretStash {
		err = cryptohomeHelper.AuthenticatePinAuthFactor(ctx, authSessionID, authFactorLabelPIN, correctPINSecret)
	} else {
		err = cryptohomeHelper.AuthenticatePinWithAuthSession(ctx, correctPINSecret, authFactorLabelPIN, authSessionID)
	}
	if (err != nil) == shouldAuthenticate {
		return errors.Wrap(err, "failed to authenticated auth factor with correct PIN")
	}
	return nil
}

// authenticateWithCorrectPassword authenticates a given user with the correct PIN.
func authenticateWithCorrectPassword(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, userParam pinWeaverWithAuthAPIParam) error {
	cryptohomeHelper := helper.CryptohomeClient()

	// Authenticate a new auth session via the new added PIN auth factor and mount the user.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser, false /*ephemeral*/)
	if err != nil {
		return errors.Wrap(err, "failed to start auth session for PIN authentication")
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Authenticate with correct password.
	if userParam.useUserSecretStash {
		err = cryptohomeHelper.AuthenticateAuthFactor(ctx, authSessionID, passwordAuthFactorLabel, passwordAuthFactorSecret)
	} else {
		err = cryptohomeHelper.AuthenticateAuthSession(ctx, passwordAuthFactorSecret, passwordAuthFactorLabel, authSessionID, false /*kiosk_mount*/)
	}
	if err != nil {
		return errors.Wrap(err, "failed to authenticated auth factor with correct password")
	}

	if err = cryptohomeHelper.InvalidateAuthSession(ctx, authSessionID); err != nil {
		return errors.Wrap(err, "failed to invalidate AuthSession")
	}
	return nil
}

// removeLeCredential removes testUser and checks to see if the leCreds on disk was updated.
func removeLeCredential(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote) error {
	cryptohomeHelper := helper.CryptohomeClient()

	leCredsBeforeRemove, err := getLeCredsFromDisk(ctx, r)
	if err != nil {
		return errors.Wrap(err, "failed to get le creds from disk")
	}

	if _, err := cryptohomeHelper.RemoveVault(ctx, testUser); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}

	leCredsAfterRemove, err := getLeCredsFromDisk(ctx, r)
	if err != nil {
		return errors.Wrap(err, "failed to get le creds from disk")
	}

	if diff := cmp.Diff(leCredsAfterRemove, leCredsBeforeRemove); diff == "" {
		return errors.Wrap(err, "LE cred not cleaned up successfully (-got +want): %s"+diff)
	}
	return nil
}

func ensurePINLockedOut(ctx context.Context, testUser string, cryptohomeClient *hwsec.CryptohomeClient) error {
	output, err := cryptohomeClient.GetKeyData(ctx, testUser, authFactorLabelPIN)
	if err != nil {
		return errors.Wrap(err, "failed to get key data")
	}
	exp := regexp.MustCompile("auth_locked: (true|false)\n")
	m := exp.FindStringSubmatch(output)
	if m == nil {
		return errors.Wrap(err, "Auth locked could not parsed from key data: %s "+output)
	}
	if m[1] != "true" {
		return errors.Wrap(err, "PIN marked not locked when it should have been")
	}
	return nil
}
