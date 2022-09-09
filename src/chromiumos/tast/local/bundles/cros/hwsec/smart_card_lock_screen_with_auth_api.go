// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
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
	smartCardLabel           = "smart-card-test-label"
	passwordAuthFactorLabel  = "fake_label"
	passwordAuthFactorSecret = "password"
	ownerUser                = "owner@owner.owner"
	testUser                 = "testUser@example.com"
	dbusName                 = "org.chromium.TestingCryptohomeKeyDelegate"
	testFile                 = "file"
	testFileContent          = "content"
	keySizeBits              = 2048
)

func SmartCardWithAuthAPI(ctx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set up KeyDelegate for the Smart Card.
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
		defer cleanupUSSExperiment()
	}

	// Prepare Smart Card config.
	authConfig, err := SetupSmartCard(ctx, s, userParam.smartCardAlgorithms)
	if err != nil {
		s.Fatal("Failed to run SetupSmartCard with error: ", err)
	}

	// Full login/authentication case
	lockScreenSmartCardWithAuthAPI(ctx, s, smartCardLabel /*isEphemeral=*/, false, authConfig)
	lockScreenSmartCardWithAuthAPI(ctx, s, smartCardLabel /*isEphemeral=*/, true, authConfig)
}

func lockScreenSmartCardWithAuthAPI(ctx context.Context, s *testing.State, keyLabel string, isEphemeral bool, authConfig *hwsec.AuthConfig) {
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

	// cleanup, err := setupUserWithSmartCard(ctx, testUser, isEphemeral, userParam, authConfig)
	// if err != nil {
	// 	s.Fatal("Failed to create the user: ", err)
	// }
	// defer cleanup(ctx)

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
