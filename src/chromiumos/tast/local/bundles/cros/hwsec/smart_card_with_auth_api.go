// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
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
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SmartCardWithAuthAPI,
		Desc: "Checks that Smart Cards work with AuthSession, AuthFactor and USS",
		Contacts: []string{
			"thomascedeno@google.org", // Test author
			"cryptohome-core@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"tpm"},
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
	authFactorLabelSmartCard = "smart-card-test-label"
	passwordAuthFactorLabel  = "fake_label"
	passwordAuthFactorSecret = "password"
	testUser1                = "testUser1@example.com"
	dbusName                 = "org.chromium.TestingCryptohomeKeyDelegate"
	keySizeBits              = 2048
)

func SmartCardWithAuthAPI(ctx context.Context, s *testing.State) {
	userParam := s.Param().(smartCardWithAuthAPIParam)
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set up KeyDelegate for the Smart Card.
	keyAlgs := []cpb.ChallengeSignatureAlgorithm{cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1}

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
		s.Logf, dbusConn, testUser1, keyAlgs, rsaKey, pubKeySPKIDER)
	if err != nil {
		s.Fatal("Failed to export D-Bus key delegate: ", err)
	}
	defer keyDelegate.Close()

	// Clean up obsolete state, in case there's any.
	cmdRunner.Run(ctx, "rm -rf /home/.shadow/low_entropy_creds")
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}
	if _, err := client.RemoveVault(ctx, testUser1); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Prepare Smart Card config.
	authConfig := hwsec.NewChallengeAuthConfig(testUser1, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlgs)

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
	if err = setupUserWithSmartCard(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, authConfig); err != nil {
		s.Fatal("Failed to run setupUserWithSmartCard with error: ", err)
	}
	defer removeSmartCardCredential(ctx, ctxForCleanUp, testUser1, cmdRunner, helper)

	// Ensure we can authenticate with correct Smart Card.
	if err = authenticateWithSmartCard(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, authConfig); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectSmartCard with error: ", err)
	}

	// Ensure that testUser1 still works wth Smart Card.
	if err = authenticateWithSmartCard(ctx, ctxForCleanUp, testUser1, cmdRunner, helper, userParam, authConfig); err != nil {
		s.Fatal("Failed to run authenticateWithCorrectSmartCard with error: ", err)
	}
}

// setupUserWithSmartCard sets up a user with a password and a Smart Card auth factor.
func setupUserWithSmartCard(ctx, ctxForCleanUp context.Context, userName string, cmdRunner *hwseclocal.CmdRunnerLocal, helper *hwseclocal.CmdHelperLocal, userParam smartCardWithAuthAPIParam, authConfig *hwsec.AuthConfig) error {
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
		err = cryptohomeHelper.AddSmartCardAuthFactor(ctx, authSessionID, authFactorLabelSmartCard, authConfig)
	} else {
		err = cryptohomeHelper.AddChallengeCredentialsWithAuthSession(ctx, userName, authSessionID, authConfig)
	}
	if err != nil {
		return errors.Wrap(err, "failed to add smart card credential")
	}
	return nil
}

// authenticateWithSmartCard authenticates a given user with the correct Smart Card.
func authenticateWithSmartCard(ctx, ctxForCleanUp context.Context, testUser string, r *hwseclocal.CmdRunnerLocal, helper *hwseclocal.CmdHelperLocal, userParam smartCardWithAuthAPIParam, authConfig *hwsec.AuthConfig) error {
	cryptohomeHelper := helper.CryptohomeClient()

	// Authenticate a new auth session via the new added Smart Card auth factor.
	authSessionID, err := cryptohomeHelper.StartAuthSession(ctx, testUser, false /*ephemeral*/)
	if err != nil {
		return errors.Wrap(err, "failed to start auth session for Smart Card authentication")
	}
	defer cryptohomeHelper.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if userParam.useAuthFactor {
		err = cryptohomeHelper.AuthenticateSmartCardAuthFactor(ctx, authSessionID, authFactorLabelSmartCard, authConfig)
	} else {
		err = cryptohomeHelper.AuthenticateChallengeCredentialWithAuthSession(ctx, authSessionID, authConfig)
	}
	return nil
}

// removeSmartCardCredential removes testUser.
func removeSmartCardCredential(ctx, ctxForCleanUp context.Context, testUser string, r *hwseclocal.CmdRunnerLocal, helper *hwseclocal.CmdHelperLocal) error {
	cryptohomeHelper := helper.CryptohomeClient()

	if _, err := cryptohomeHelper.RemoveVault(ctx, testUser); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}
