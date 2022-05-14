// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LockScreenChallengeResponseAuthSession,
		Desc: "Tests for Lock Screen verification with Smart Card backend, for both persistent and ephemeral users",
		Contacts: []string{
			"thomascedeno@google.com",
			"cryptohome-core@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"testcert.p12"},
		SoftwareDeps: []string{"tpm"},
	})
}

// LockScreenChallengeResponseAuthSession initializes and calls the respective tests.
func LockScreenChallengeResponseAuthSession(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		expectAuthSuccess bool
		isEphemeral       bool
	}{
		{true, true},
		{true, false},
		{false, true},
		{false, false},
	} {
		lockScreenLogin(ctx, s, tc.expectAuthSuccess, tc.isEphemeral)
	}
}

func lockScreenLogin(ctx context.Context, s *testing.State, expectAuthSuccess, isEphemeral bool) {
	const (
		ownerUser       = "owner@owner.owner"
		keyLabel        = "fake_label"
		dbusName        = "org.chromium.TestingCryptohomeKeyDelegate"
		testUser        = "cryptohome_test@chromium.org"
		testFile        = "file"
		testFileContent = "content"
		keySizeBits     = 2048
		keyAlg          = cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1
	)

	// Initialize the underlying KeyDelegate for challenge.
	cmdRunner := hwseclocal.NewCmdRunner()
	client := hwsec.NewCryptohomeClient(cmdRunner)
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
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

	keyDelegate, err := util.NewCryptohomeKeyDelegate(
		s.Logf, dbusConn, testUser, keyAlg, rsaKey, pubKeySPKIDER)
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
		if err := hwseclocal.SetUpVaultAndUserAsOwner(ctx, s.DataPath("testcert.p12"), ownerUser, "whatever", "whatever", helper.CryptohomeClient()); err != nil {
			client.UnmountAll(ctx)
			client.RemoveVault(ctx, ownerUser)
			s.Fatal("Failed to setup vault and user as owner: ", err)
		}
		if err := client.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmount vaults for preparation: ", err)
		}
		defer client.RemoveVault(ctx, ownerUser)
	}

	if err := cryptohome.CreateUserAuthSessionWithChallengeCredential(ctx, testUser, isEphemeral, hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg)); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}

	// Authenticate while the cryptohome is still mounted (modeling the case of
	// the user unlocking the device from the Lock Screen).
	requestedUser := testUser
	if !expectAuthSuccess {
		// "Corrput" key check request by intentionally providing wrong user.
		requestedUser = "corrputed_testUser"
	}
	_, err = utility.CheckVault(ctx, keyLabel, hwsec.NewChallengeAuthConfig(requestedUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg))

	// Verify.
	if expectAuthSuccess && err != nil {
		s.Fatal("Failed to check the key for the mounted cryptohome: ", err)
	} else if !expectAuthSuccess && err == nil {
		s.Fatal("Key check succeed with wrong credentials: ", err)
	}

}
