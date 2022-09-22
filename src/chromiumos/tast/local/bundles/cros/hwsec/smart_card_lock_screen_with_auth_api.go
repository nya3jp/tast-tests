// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"math/rand"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SmartCardLockScreenAuthSession,
		Desc: "Tests for Lock Screen verification with Smart Card backend, for both persistent and ephemeral users, based on the old AuthSession based CheckKey verificaiton",
		Contacts: []string{
			"thomascedeno@google.com",
			"cryptohome-core@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"testcert.p12"},
		SoftwareDeps: []string{"tpm"},
		Params: []testing.Param{
			{
				Name: "rsassa_sha1",
				Val: []cpb.ChallengeSignatureAlgorithm{
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1,
				},
			},
			{
				Name: "rsassa_all",
				Val:  hwsec.SmartCardAlgorithms,
			}},
	})
}

// SmartCardLockScreenAuthSession initializes and calls the respective tests.
func SmartCardLockScreenAuthSession(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		isEphemeral bool
	}{
		{true},
		{false},
	} {
		lockScreenLogin(ctx, s, tc.isEphemeral)
	}
}

func lockScreenLogin(ctx context.Context, s *testing.State, isEphemeral bool) {
	const (
		ownerUser   = "owner@owner.owner"
		keyLabel    = "smart-card-label"
		dbusName    = "org.chromium.TestingCryptohomeKeyDelegate"
		testUser    = "cryptohome_test@chromium.org"
		keySizeBits = 2048
	)
	keyAlgs := s.Param().([]cpb.ChallengeSignatureAlgorithm)

	// Initialize the underlying KeyDelegate for challenge.
	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	utility := helper.CryptohomeClient()
	daemonController := helper.DaemonController()

	// Ensure cryptohomed is started and wait for it to be available.
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

	keyDelegate, err := hwsec.NewCryptohomeKeyDelegate(
		s.Logf, dbusConn, testUser, keyAlgs, rsaKey, pubKeySPKIDER)
	if err != nil {
		s.Fatal("Failed to export D-Bus key delegate: ", err)
	}
	defer keyDelegate.Close()

	// Clean up possible mounts from prior tests.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	if err := cryptohome.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

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

	authConfig := hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlgs)
	cleanup, err := cryptohome.CreateUserAuthSessionWithChallengeCredential(ctx, testUser, keyLabel, isEphemeral, authConfig)
	if err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cleanup(ctx)

	// Authenticate while the cryptohome is still mounted (modeling the case of
	// the user unlocking the device from the Lock Screen).
	// In this case the check should succeed.
	_, err = utility.CheckVault(ctx, keyLabel, authConfig)
	// Verify.
	if err != nil {
		s.Fatal("Failed to check the key for the mounted cryptohome: ", err)
	}

	// "Corrput" key check request by intentionally providing wrong user.
	// In this case the check should fail.
	corruptedUsername := "corrputed_testUser"
	_, err = utility.CheckVault(ctx, keyLabel, hwsec.NewChallengeAuthConfig(corruptedUsername, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlgs))
	// Verify.
	if err == nil {
		s.Fatal("Failure expected, but key check succeeded with wrong credentials: ", err)
	}

}
