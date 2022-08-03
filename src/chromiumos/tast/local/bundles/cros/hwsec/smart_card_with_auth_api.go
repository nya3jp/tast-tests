// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"time"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
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
	// Specifies which group of encryption the smart card supports.
	smartCardAlgorithms []cpb.ChallengeSignatureAlgorithm
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SmartCardWithAuthAPI,
		Desc: "Checks that Smart Cards work with AuthSession, AuthFactor and USS",
		Contacts: []string{
			"thomascedeno@google.org", // Test author
			"cryptohome-core@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		//Data:         []string{"testcert.p12"},
		SoftwareDeps: []string{"tpm"},
		Params: []testing.Param{{
			Name: "smart_card_with_auth_factor_with_no_uss_rsassa_sha1",
			Val: smartCardWithAuthAPIParam{
				useUserSecretStash: false,
				useAuthFactor:      true,
				smartCardAlgorithms: []cpb.ChallengeSignatureAlgorithm{
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1,
				},
			},
		}, {
			Name: "smart_card_with_auth_factor_with_no_uss_rsassa_all",
			Val: smartCardWithAuthAPIParam{
				useUserSecretStash: false,
				useAuthFactor:      true,
				smartCardAlgorithms: []cpb.ChallengeSignatureAlgorithm{
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA256,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA384,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA512,
				},
			},
		}, {
			Name: "smart_card_with_auth_session_rsassa_sha1",
			Val: smartCardWithAuthAPIParam{
				useUserSecretStash: false,
				useAuthFactor:      false,
				smartCardAlgorithms: []cpb.ChallengeSignatureAlgorithm{
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1,
				},
			},
		}, {
			Name: "smart_card_with_auth_session_rsassa_all",
			Val: smartCardWithAuthAPIParam{
				useUserSecretStash: false,
				useAuthFactor:      false,
				smartCardAlgorithms: []cpb.ChallengeSignatureAlgorithm{
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA256,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA384,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA512,
				},
			},
		}, {
			Name: "smart_card_with_auth_factor_with_uss_rsassa_sha1",
			Val: smartCardWithAuthAPIParam{
				useUserSecretStash: true,
				useAuthFactor:      true,
				smartCardAlgorithms: []cpb.ChallengeSignatureAlgorithm{
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1,
				},
			},
		}, {
			Name: "smart_card_with_auth_factor_with_uss_rsassa_all",
			Val: smartCardWithAuthAPIParam{
				useUserSecretStash: true,
				useAuthFactor:      true,
				smartCardAlgorithms: []cpb.ChallengeSignatureAlgorithm{
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA256,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA384,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA512,
				},
			},
		}},
	})
}

// Some constants used across the test.
const (
	authFactorLabelSmartCard = "smart-card-test-label"
	passwordAuthFactorLabel  = "fake_label"
	passwordAuthFactorSecret = "password"
	ownerUser                = "owner@owner.owner"
	testUser                 = "testUser@example.com"
	dbusName                 = "org.chromium.TestingCryptohomeKeyDelegate"
	testFile                 = "file"
	testFileContent          = "content"
	keyLabel                 = "fake_label"
	keySizeBits              = 2048
)

func SmartCardWithAuthAPI(ctx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set up KeyDelegate for the Smart Card.
	userParam := s.Param().(smartCardWithAuthAPIParam)
	keyAlgs := userParam.smartCardAlgorithms

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	daemonController := helper.DaemonController()

	// Wait for cryptohomed becomes available if needed.
	if err := daemonController.Ensure(ctx, hwsec.CryptohomeDaemon); err != nil {
		s.Fatal("Failed to ensure cryptohomed: ", err)
	}

	// Use a pseudorandom generator with a fixed seed, to make the values used by
	// the test predictable.
	randReader := rand.New(rand.NewSource(0 /* seed */))

	rsaKey, err := rsa.GenerateKey(randReader, keySizeBits)
	if err != nil {
		s.Fatal("Failed to generate RSA key: ", err)
	}
	pubKeySPKIDER, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		s.Fatal("Failed to generate SubjectPublicKeyInfo: ", err)
	}

	dbusConn, err := dbusutil.SystemBus()
	if err != nil {
		s.Fatal("Failed to connect to system D-Bus bus: ", err)
	}
	if _, err := dbusConn.RequestName(dbusName, 0 /* flags */); err != nil {
		s.Fatal("Failed to request the well-known D-Bus name: ", err)
	}
	defer dbusConn.ReleaseName(dbusName)

	keyDelegate, err := util.NewCryptohomeKeyDelegate(
		s.Logf, dbusConn, testUser, keyAlgs, rsaKey, pubKeySPKIDER)
	if err != nil {
		s.Fatal("Failed to export D-Bus key delegate: ", err)
	}
	defer keyDelegate.Close()

	// Clean up obsolete state and possible mounts from prior tests, in case there's any.
	cmdRunner.Run(ctx, "rm -rf /home/.shadow/low_entropy_creds")
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if _, err := client.RemoveVault(ctx, testUser); err != nil {
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

	// Prepare Smart Card config.
	authConfig := hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlgs)
	// Full login/authentication case
	fullAuthenticationSmartCardWithAuthAPI(ctx, s, authConfig)
	// Lock screen case
	// lockScreenSmartCardWithAuthAPI(ctx, s, /*isEphemeral=*/ true, authConfig);
	// lockScreenSmartCardWithAuthAPI(ctx, s, /*isEphemeral=*/ false, authConfig);
}

func fullAuthenticationSmartCardWithAuthAPI(ctx context.Context, s *testing.State, authConfig *hwsec.AuthConfig) {
	userParam := s.Param().(smartCardWithAuthAPIParam)

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)

	// Setup a user for testing.
	cleanup, err := setupUserWithSmartCard(ctx, testUser /*isEphemeral=*/, false, userParam, authConfig)
	if err != nil {
		s.Fatal("Failed to run setupUserWithSmartCard with error: ", err)
	}
	defer cleanup(ctx)

	// Write a test file to verify persistence.
	userPath, err := cryptohome.UserPath(ctx, testUser)
	if err != nil {
		s.Fatal("Failed to get user vault path: ", err)
	}

	filePath := filepath.Join(userPath, testFile)
	if err := ioutil.WriteFile(filePath, []byte(testFileContent), 0644); err != nil {
		s.Fatal("Failed to write a file to the vault: ", err)
	}

	// Unmount recently mounted vaults.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	// Remount the specific vault, remount should succeed.
	// Ensure we can reauthenticate with correct Smart Card.
	authSessionID, err := authenticateWithSmartCard(ctx, testUser, userParam, authConfig)
	if err != nil {
		s.Fatal("Failed to run authenticateWithCorrectSmartCard with error: ", err)
	}

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}

	// Verify that file is still there.
	if content, err := ioutil.ReadFile(filePath); err != nil {
		s.Fatal("Failed to read back test file: ", err)
	} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
		s.Fatalf("Incorrect tests file content. got: %q, want: %q", content, testFileContent)
	}

	// Clear AuthSession and unmount previously mounted vault.
	client.InvalidateAuthSession(ctx, authSessionID)
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	// Remount should fail.
	// Failure occurs because of manually "corrputed" requestedUser
	requestedUser := "corrputed_testUser"
	if _, err = authenticateWithSmartCard(ctx, requestedUser, userParam, authConfig); err == nil {
		s.Fatal("Authentication with wrong credentials is expected to fail but succeeded: ", err)
	}

	// Verify that file is still there, but should not be readable.
	if _, err := ioutil.ReadFile(filePath); err == nil {
		s.Fatal("File is readable after unsuccessful authentication, but it expected to be unreadable: ", err)
	}
}

func lockScreenSmartCardWithAuthAPI(ctx context.Context, s *testing.State, isEphemeral bool, authConfig *hwsec.AuthConfig) {
	userParam := s.Param().(smartCardWithAuthAPIParam)

	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility := helper.CryptohomeClient()

	if isEphemeral {
		// Set up the first user as the owner. It is required to mount ephemeral.
		if err := hwseclocal.SetUpVaultAndUserAsOwner(ctx, s.DataPath("testcert.p12"), ownerUser, "password_not_needed", "label_not_needed", utility); err != nil {
			client.UnmountAll(ctx)
			client.RemoveVault(ctx, ownerUser)
			s.Fatal("Failed to setup vault and user as owner: ", err)
		}
		if err := client.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmount vaults for preparation: ", err)
		}
		defer client.RemoveVault(ctx, ownerUser)
	}

	cleanup, err := setupUserWithSmartCard(ctx, testUser, isEphemeral, userParam, authConfig)
	if err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cleanup(ctx)

	// Authenticate while the cryptohome is still mounted (modeling the case of
	// the user unlocking the device from the Lock Screen).
	// In this case the check should succeed.
	if _, err = utility.CheckVault(ctx, keyLabel, authConfig); err != nil {
		s.Fatal("Failed to check the key for the mounted cryptohome: ", err)
	}

	// "Corrput" key check request by intentionally providing wrong user.
	// In this case the check should fail.
	corruptAuthConfig := hwsec.NewChallengeAuthConfig("corrputed_testUser", dbusName, authConfig.KeyDelegatePath, authConfig.ChallengeSPKI, authConfig.ChallengeAlgs)
	if _, err = utility.CheckVault(ctx, keyLabel, corruptAuthConfig); err == nil {
		s.Fatal("Failure expected, but key check succeeded with wrong credentials: ", err)
	}
}

// setupUserWithSmartCard sets up a user with a password and a Smart Card auth factor.
func setupUserWithSmartCard(ctx context.Context, testUser string, isEphemeral bool, userParam smartCardWithAuthAPIParam, authConfig *hwsec.AuthConfig) (func(ctx context.Context) error, error) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	authSessionID, err := cryptohome.StartAuthSession(ctx, testUser, isEphemeral)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start auth session for Smart Card authentication")
	}
	defer cryptohome.InvalidateAuthSession(ctx, authSessionID)
	testing.ContextLog(ctx, "Auth session ID: ", authSessionID)

	cleanup := func(ctx context.Context) error {
		if err := cryptohome.UnmountAndRemoveVault(ctx, testUser); err != nil {
			return errors.Wrap(err, "failed to remove and unmount vault")
		}
		return nil
	}

	if isEphemeral { // Ephemeral AuthSession
		if err := cryptohome.PrepareEphemeralVault(ctx, authSessionID); err != nil {
			return nil, errors.Wrap(err, "failed to prepare ephemeral vault")
		}

	} else { // Persistent AuthSession
		if err = cryptohome.CreatePersistentUser(ctx, authSessionID); err != nil {
			return nil, errors.Wrap(err, "failed to create persistent user with auth session")
		}

		if err = cryptohome.PreparePersistentVault(ctx, authSessionID, false); err != nil {
			return nil, errors.Wrap(err, "failed to prepare persistent user with auth session")
		}
	}

	if userParam.useAuthFactor {
		err = cryptohome.AddAuthFactor(ctx, authSessionID, passwordAuthFactorLabel, passwordAuthFactorSecret)
	} else {
		err = cryptohome.AddCredentialsWithAuthSession(ctx, testUser, passwordAuthFactorSecret, passwordAuthFactorLabel, authSessionID, false /*kiosk*/)
	}

	if err != nil {
		cleanup(ctx)
		return nil, errors.Wrap(err, "failed to add password auth factor")
	}

	// Add a Smart Card auth factor to the user.
	if userParam.useAuthFactor {
		err = cryptohome.AddSmartCardAuthFactor(ctx, authSessionID, authFactorLabelSmartCard, authConfig)
	} else {
		err = cryptohome.AddChallengeCredentialsWithAuthSession(ctx, testUser, authSessionID, authConfig)
	}
	if err != nil {
		cleanup(ctx)
		return nil, errors.Wrap(err, "failed to add smart card credential")
	}
	return cleanup, nil
}

// authenticateWithSmartCard authenticates a given user with the correct Smart Card.
func authenticateWithSmartCard(ctx context.Context, testUser string, userParam smartCardWithAuthAPIParam, authConfig *hwsec.AuthConfig) (string, error) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	// Authenticate a new auth session via the new added Smart Card auth factor.
	authSessionID, err := cryptohome.StartAuthSession(ctx, testUser /*isEphemeral=*/, false)
	if err != nil {
		return "", errors.Wrap(err, "failed to start auth session for Smart Card authentication")
	}

	if userParam.useAuthFactor {
		if err := cryptohome.AuthenticateSmartCardAuthFactor(ctx, authSessionID, authFactorLabelSmartCard, authConfig); err != nil {
			return "", errors.Wrap(err, "failed to authenticate with AuthFactor")
		}
	} else {
		if err = cryptohome.AuthenticateChallengeCredentialWithAuthSession(ctx, authSessionID, authConfig); err != nil {
			return "", errors.Wrap(err, "failed to authenticate with AuthSession")
		}
	}
	return authSessionID, nil
}
