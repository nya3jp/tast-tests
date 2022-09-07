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
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChallengeResponseAuthSession,
		Desc: "Tests for creation and remounting through AuthSession and Smart Card backed authentication",
		Contacts: []string{
			"thomascedeno@google.com",
			"cryptohome-core@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
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
				Val: []cpb.ChallengeSignatureAlgorithm{
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA256,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA384,
					cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA512,
				},
			}},
	})
}

// ChallengeResponseAuthSession initializes the persistent Creation and remounting of
// Challenge Credentials through AuthSession
func ChallengeResponseAuthSession(ctx context.Context, s *testing.State) {
	const (
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

	authConfig := hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlgs)
	cleanup, err := cryptohome.CreateUserAuthSessionWithChallengeCredential(ctx, testUser, false /*isEphemeral*/, authConfig)
	if err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cleanup(ctx)

	// Write a test file to verify persistence.
	if err := cryptohome.WriteFileForPersistence(ctx, testUser); err != nil {
		s.Fatal("Failed to write test file: ", err)
	}

	// Unmount recently mounted vaults.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	// Reauthenticate and remount the specific vault.
	// Remount should succeed.
	authSessionID, err := cryptohome.AuthenticateAuthSessionWithChallengeCredential(ctx, testUser, false /*isEphemeral*/, authConfig)
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}

	// Verify that file is still there.
	if err := cryptohome.VerifyFileForPersistence(ctx, testUser); err != nil {
		s.Fatal("Failed to verify file persistence: ", err)
	}

	// Clear AuthSession and unmount previously mounted vault.
	client.InvalidateAuthSession(ctx, authSessionID)
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	// Remount should fail.
	// Failure occurs because of manually "corrputed" requestedUser
	requestedUser := "corrputed_testUser"
	_, err = cryptohome.AuthenticateAuthSessionWithChallengeCredential(ctx, requestedUser, false /*isEphemeral*/, authConfig)
	if err == nil {
		s.Fatal("Authentication with wrong credentials is expected to fail but succeeded: ", err)
	}

	// Unauthenticated authsession should not allow the file to be read.
	if err := cryptohome.VerifyFileUnreadability(ctx, testUser); err != nil {
		s.Fatal("File is readable when it should not be: ", err)
	}
}
