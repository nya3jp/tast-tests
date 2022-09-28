// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"math/rand"
	"time"

	cpb "chromiumos/system_api/cryptohome_proto"
	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	cryptohomelocal "chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// smartCardWithAuthAPIParam contains the test parameters which are different
// between the types of backing store.
type smartCardWithAuthAPIParam struct {
	// Specifies whether to use user secret stash.
	useUserSecretStash bool
	// Specifies whether to use AuthFactor or AuthSession based API's.
	useAuthFactor bool
	// Specifies which group of encryption the smart card supports.
	smartCardAlgorithms []cpb.ChallengeSignatureAlgorithm
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SmartCardWithAuthAPI,
		Desc: "Checks that Smart Cards work with AuthSession, AuthFactor and USS",
		Contacts: []string{
			"thomascedeno@google.com", // Test author
			"cryptohome-core@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
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
				useUserSecretStash:  false,
				useAuthFactor:       true,
				smartCardAlgorithms: hwsec.SmartCardAlgorithms,
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
				useUserSecretStash:  false,
				useAuthFactor:       false,
				smartCardAlgorithms: hwsec.SmartCardAlgorithms,
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
				useUserSecretStash:  true,
				useAuthFactor:       true,
				smartCardAlgorithms: hwsec.SmartCardAlgorithms,
			},
		}},
	})
}

// Some constants used across the test.
const (
	smartCardLabel = "smart-card-test-label"
	testUser       = "testUser@example.com"
	dbusName       = "org.chromium.TestingCryptohomeKeyDelegate"
	testFile       = "file"
	keySizeBits    = 2048
)

func SmartCardWithAuthAPI(ctx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	userParam := s.Param().(smartCardWithAuthAPIParam)

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
		defer cleanupUSSExperiment(ctx)
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

	keyDelegate, err := hwsec.NewCryptohomeKeyDelegate(
		s.Logf, dbusConn, testUser, userParam.smartCardAlgorithms, rsaKey, pubKeySPKIDER)
	if err != nil {
		s.Fatal("Failed to export D-Bus key delegate: ", err)
	}
	defer keyDelegate.Close()

	// Prepare Smart Card config.
	authConfig := hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, userParam.smartCardAlgorithms)

	// Setup a user for testing.
	cleanup, err := setupUserWithSmartCard(ctx, testUser /*isEphemeral=*/, false, userParam, authConfig)
	if err != nil {
		s.Fatal("Failed to run setupUserWithSmartCard with error: ", err)
	}
	defer cleanup(ctx)

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
	if err := cryptohomelocal.VerifyFileForPersistence(ctx, testUser); err != nil {
		s.Fatal("Failed to read back test file: ", err)
	}

	// Clear AuthSession and unmount previously mounted vault.
	client.InvalidateAuthSession(ctx, authSessionID)
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	// Remount should fail.
	// Failure occurs because of manually "corrputed_user".
	if _, err = authenticateWithSmartCard(ctx, "corrputed_user", userParam, authConfig); err == nil {
		s.Fatal("Authentication with wrong credentials is expected to fail but succeeded: ", err)
	}

	// Verify that file is still there, but should not be readable.
	if err := cryptohomelocal.VerifyFileUnreadability(ctx, testUser); err != nil {
		s.Fatal("File is readable after unsuccessful authentication, but it expected to be unreadable: ", err)
	}
}

// setupUserWithSmartCard sets up a user with a password and a Smart Card auth factor.
func setupUserWithSmartCard(ctx context.Context, testUser string, isEphemeral bool, userParam smartCardWithAuthAPIParam, authConfig *hwsec.AuthConfig) (func(ctx context.Context) error, error) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Start an Auth session and get an authSessionID.
	_, authSessionID, err := cryptohome.StartAuthSession(ctx, testUser, isEphemeral, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start auth session for Smart Card authentication")
	}
	defer cryptohome.InvalidateAuthSession(ctx, authSessionID)

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

	// Add a Smart Card auth factor to the user.
	if userParam.useAuthFactor {
		err = cryptohome.AddSmartCardAuthFactor(ctx, authSessionID, smartCardLabel, authConfig)
	} else {
		err = cryptohome.AddChallengeCredentialsWithAuthSession(ctx, testUser, authSessionID, smartCardLabel, authConfig)
	}
	if err != nil {
		cleanup(ctx)
		return nil, errors.Wrap(err, "failed to add smart card credential")
	}

	// Write a test file to verify persistence.
	if err := cryptohomelocal.WriteFileForPersistence(ctx, testUser); err != nil {
		return nil, errors.Wrap(err, "failed to write test file")
	}

	return cleanup, nil
}

// authenticateWithSmartCard authenticates a given user with the correct Smart Card.
func authenticateWithSmartCard(ctx context.Context, testUser string, userParam smartCardWithAuthAPIParam, authConfig *hwsec.AuthConfig) (string, error) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	// Authenticate a new auth session via the new added Smart Card auth factor.
	_, authSessionID, err := cryptohome.StartAuthSession(ctx, testUser /*isEphemeral=*/, false, uda.AuthIntent_AUTH_INTENT_DECRYPT)
	if err != nil {
		return "", errors.Wrap(err, "failed to start auth session for Smart Card authentication")
	}

	if userParam.useAuthFactor {
		if err := cryptohome.AuthenticateSmartCardAuthFactor(ctx, authSessionID, smartCardLabel, authConfig); err != nil {
			return "", errors.Wrap(err, "failed to authenticate with AuthFactor")
		}
	} else {
		if err = cryptohome.AuthenticateChallengeCredentialWithAuthSession(ctx, authSessionID, smartCardLabel, authConfig); err != nil {
			return "", errors.Wrap(err, "failed to authenticate with AuthSession")
		}
	}
	return authSessionID, nil
}
