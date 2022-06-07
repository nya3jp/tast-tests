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
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

// pinWeaverWithAuthAPIParam contains the test parameters which are different
// between the types of backing store.
type pinWeaverWithAuthAPIParam struct {
	// Specifies whether to useUSS.
	// This, for now, also assumes that AuthSession would be used with AuthFactors.
	useUSS bool
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
				useUSS:                true,
				useLegacyAddAPIForPin: false,
			},
		}, {
			Name: "pin_weaver_with_auth_session",
			Val: pinWeaverWithAuthAPIParam{
				useUSS:                false,
				useLegacyAddAPIForPin: false,
			},
		}, {
			Name: "pin_weaver_with_auth_session_legacy_pin_add",
			Val: pinWeaverWithAuthAPIParam{
				useUSS:                false,
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

	if userParam.useUSS {
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
	setupUserWithPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s)
	defer removeLeCredential(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s)
	authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s /*shouldAuthenticate=*/, true)
	// After the PIN lock out we should not be able to authenticate with correct PIN.
	authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s)

	// // Setup a user 2 for testing. This user will be removed and the le_credential file will be checked.
	setupUserWithPIN(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, s)
	// defer removeLeCredential(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, s)
	// // After the PIN lock out we should not be able to authenticate with correct PIN.
	authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, s)
	authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, s /*shouldAuthenticate=*/, true)

	// Ensure that user 1 still works.
	authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s /*shouldAuthenticate=*/, true)
	authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s)

	/** Running test where we try to almost lock out PIN with 4 attempts twice, but the user is able to log back in **/
	// Attempt four wrong PIN.
	attemptWrongPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s /*attempts=*/, 4)
	// After the PIN lock out we should not be able to authenticate with correct PIN.
	authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s /*shouldAuthenticate=*/, true)
	// Attempt four wrong PIN.
	attemptWrongPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s /*attempts=*/, 4)
	// After the PIN lock out we should not be able to authenticate with correct PIN.
	authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s /*shouldAuthenticate=*/, true)

	/** Test whether the attempt counter persists after reboot **/
	// Attempt four wrong PIN.
	attemptWrongPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s /*attempts=*/, 4)
	// Because Cr50 stores state in the firmware, that persists across reboots, this test
	// needs to run before and after a reboot.
	helper.Reboot(ctx)

	// Lockout the PIN this time.
	attemptWrongPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s /*attempts=*/, 2)
	// After the PIN lock out we should not be able to authenticate with correct PIN.
	// authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s,/*shouldAuthenticate=*/false)
	ensurePINLockedOut(ctx, testUser1, client, s)

	/** Ensure that testUser2 can still use PIN **/
	// After the PIN lock out we should not be able to authenticate with correct PIN.
	authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, s /*shouldAuthenticate=*/, true)

	/** Unlock PIN **/
	// After the PIN lock out we should not be able to authenticate with correct PIN.
	authenticateWithCorrectPassword(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s)

	// After the PIN lock out we should not be able to authenticate with correct PIN.
	authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s /*shouldAuthenticate=*/, true)

	// Remove the added PIN and check to see if le_credential file was updated.
	removeLeCredential(ctx, ctxForCleanUp, testUser2, cmdRunner, helper, s)

	/** Ensure test user 1 can still login with PIN**/
	// After the PIN lock out we should not be able to authenticate with correct PIN.
	authenticateWithCorrectPIN(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, s /*shouldAuthenticate=*/, true)

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
func setupUserWithPIN(ctx, ctxForCleanUp context.Context, userName string, cmdRunner *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()
	userParam := s.Param().(pinWeaverWithAuthAPIParam)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, userName /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start Auth session: ", err)
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctx, authSessionID)

	if err := cryptohomeHelper.CreatePersistentUser(ctx, authSessionID); err != nil {
		s.Fatal("Failed to create persistent user: ", err)
	}

	if err := cryptohomeHelper.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer cryptohomeHelper.Unmount(ctx, userName)

	if userParam.useUSS {
		if err := cryptohomeHelper.AddAuthFactor(ctx, authSessionID, passwordAuthFactorLabel, passwordAuthFactorSecret); err != nil {
			s.Fatal("Failed to add credentials with AuthSession: ", err)
		}
	} else {
		if err := cryptohomeHelper.AddCredentialsWithAuthSession(ctx, passwordAuthFactorLabel, passwordAuthFactorSecret, authSessionID /*kiosk=*/, false); err != nil {
			s.Fatal("Failed to add credentials with AuthSession: ", err)
		}
	}

	leCredsBeforeAdd, err := getLeCredsFromDisk(ctx, cmdRunner)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	// Add a PIN auth factor to the user.
	if userParam.useLegacyAddAPIForPin {
		if err := cryptohomeHelper.AddVaultKey(ctx, userName, passwordAuthFactorSecret, passwordAuthFactorLabel, correctPINSecret, authFactorLabelPIN, true); err != nil {
			s.Fatal("Failed to add le credential: ", err)
		}

	} else {
		if userParam.useUSS {
			if err := cryptohomeHelper.AddPinAuthFactor(ctx, authSessionID, authFactorLabelPIN, correctPINSecret); err != nil {
				s.Fatal("Failed to create persistent user: ", err)
			}
		} else {
			if err := cryptohomeHelper.AddPinCredentialsWithAuthSession(ctx, authFactorLabelPIN, correctPINSecret, authSessionID); err != nil {
				s.Fatal("Failed to create persistent user: ", err)
			}
		}
	}
	leCredsAfterAdd, err := getLeCredsFromDisk(ctx, cmdRunner)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if diff := cmp.Diff(leCredsAfterAdd, leCredsBeforeAdd); diff == "" {
		s.Fatal("LE cred not added successfully")
	}

}

// attemptWrongPIN attempts to try wrong PIN for authentication for given number of attempts.
func attemptWrongPIN(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State, numberOfWrongAttempts int) {
	cryptohomeHelper := helper.CryptohomeClient()
	userParam := s.Param().(pinWeaverWithAuthAPIParam)

	// Authenticate a new auth session via the new added PIN auth factor and mount the user.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session for PIN authentication: ", err)
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Supply invalid credentials five times to trigger firmware lockout of the credential.
	for i := 0; i < numberOfWrongAttempts; i++ {
		if userParam.useUSS {
			if err := cryptohomeHelper.AuthenticatePinAuthFactor(ctx, authSessionID, authFactorLabelPIN, incorrectPINSecret); err == nil {
				s.Fatal("Authenticate auth factor succeeded but should have failed")
			}
		} else {
			if err := cryptohomeHelper.AuthenticatePinWithAuthSession(ctx, incorrectPINSecret, authFactorLabelPIN, authSessionID); err == nil {
				s.Fatal("Authenticate auth factor succeeded but should have failed")
			}
		}
	}
}

// authenticateWithCorrectPIN authenticates a given user with the correct PIN.
func authenticateWithCorrectPIN(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State, shouldAuthenticate bool) {
	cryptohomeHelper := helper.CryptohomeClient()
	userParam := s.Param().(pinWeaverWithAuthAPIParam)

	// Authenticate a new auth session via the new added PIN auth factor and mount the user.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session for PIN authentication: ", err)
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Here is the logic for the following if statement:
	// shouldAuthenticate | err!=nil (aka did not authenticate) | result
	// 		0						0								1 (error)
	// 		1						0								0 (expected)
	// 		0						1								0 (expected)
	// 		1						1								1 (error)
	if userParam.useUSS {
		if err := cryptohomeHelper.AuthenticatePinAuthFactor(ctx, authSessionID, authFactorLabelPIN, correctPINSecret); (err != nil) == shouldAuthenticate {
			s.Fatal("Authenticate auth factor with PIN failed: ", err)
		}
	} else {
		if err := cryptohomeHelper.AuthenticatePinWithAuthSession(ctx, correctPINSecret, authFactorLabelPIN, authSessionID); (err != nil) == shouldAuthenticate {
			s.Fatal("Authenticate auth factor with PIN failed: ", err)
		}
	}
}

// authenticateWithCorrectPassword authenticates a given user with the correct PIN.
func authenticateWithCorrectPassword(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()
	userParam := s.Param().(pinWeaverWithAuthAPIParam)

	// Authenticate a new auth session via the new added PIN auth factor and mount the user.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser /*ephemeral=*/, false)
	if err != nil {
		s.Fatal("Failed to start auth session for PIN authentication: ", err)
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	// Authenticate with correct password.
	if userParam.useUSS {
		if err := cryptohomeHelper.AuthenticateAuthFactor(ctx, authSessionID, passwordAuthFactorLabel, passwordAuthFactorSecret); err != nil {
			s.Fatal("Authenticate auth factor with password failed")
		}
	} else {
		if err := cryptohomeHelper.AuthenticateAuthSession(ctxForCleanUp, passwordAuthFactorSecret, authSessionID /*kiosk_mount=*/, false); err != nil {
			s.Fatal("Failed to authenticate AuthSession: ", err)
		}
	}

	if err := cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID); err != nil {
		s.Fatal("Failed to invalidate AuthSession: ", err)
	}
}

// removeLeCredential removes testUser and checks to see if the leCreds on disk was updated.
func removeLeCredential(ctx, ctxForCleanUp context.Context, testUser string, r *hwsecremote.CmdRunnerRemote, helper *hwsecremote.CmdHelperRemote, s *testing.State) {
	cryptohomeHelper := helper.CryptohomeClient()

	leCredsBeforeRemove, err := getLeCredsFromDisk(ctx, r)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if _, err := cryptohomeHelper.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	leCredsAfterRemove, err := getLeCredsFromDisk(ctx, r)
	if err != nil {
		s.Fatal("Failed to get le creds from disk: ", err)
	}

	if diff := cmp.Diff(leCredsAfterRemove, leCredsBeforeRemove); diff == "" {
		s.Fatal("LE cred not cleaned up successfully (-got +want): ", diff)
	}
}

func ensurePINLockedOut(ctx context.Context, testUser string, cryptohomeClient *hwsec.CryptohomeClient, s *testing.State) {
	output, err := cryptohomeClient.GetKeyData(ctx, testUser, authFactorLabelPIN)
	if err != nil {
		s.Fatal("Failed to get key data: ", err)
	}
	exp := regexp.MustCompile("auth_locked: (true|false)\n")
	m := exp.FindStringSubmatch(output)
	if m == nil {
		s.Fatalf("Auth locked could not parsed from key data: %s ", output)
	}
	if m[1] != "true" {
		s.Fatal("PIN marked not locked when it should have been")
	}
}
