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
		Func: ChallengeResponseAuthSession,
		Desc: "Tests for creation and remounting through AuthSession and Smart Card backed authentication",
		Contacts: []string{
			"thomascedeno@google.com",
			"cryptohome-core@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"tpm"},
	})
}

// ChallengeResponseAuthSession initializes and calls the respective tests.
func ChallengeResponseAuthSession(ctx context.Context, s *testing.State) {
	for _, tc := range []struct {
		expectRemountSuccess bool
	}{
		{true},
		{false},
	} {
		persistentCreationAndRemount(ctx, s, tc.expectRemountSuccess)
	}
}

func persistentCreationAndRemount(ctx context.Context, s *testing.State, expectRemountSuccess bool) {
	const (
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

	if err := cryptohome.CreateUserAuthSessionWithChallengeCredential(ctx, testUser, false /*isEphemeral*/, hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg)); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}

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

	// Reauthenticate and remount the specific vault.
	if expectRemountSuccess { // Remount should succeed.
		authSessionID, err := cryptohome.AuthenticateAuthSessionWithChallengeCredential(ctx, testUser, false /*isEphemeral*/, hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg))
		if err != nil {
			s.Fatal("Failed to authenticate persistent user: ", err)
		}
		defer client.InvalidateAuthSession(ctx, authSessionID)

		if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
			s.Fatal("Failed to prepare persistent vault: ", err)
		}
		defer client.UnmountAll(ctx)

		// Verify that file is still there.
		if content, err := ioutil.ReadFile(filePath); err != nil {
			s.Fatal("Failed to read back test file: ", err)
		} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
			s.Fatalf("Incorrect tests file content. got: %q, want: %q", content, testFileContent)
		}

	} else { // Remount should fail.
		// Failure occurs because of manually "corrputed" testUser
		fakeTestUser := "corrputed_testUser"
		_, err = cryptohome.AuthenticateAuthSessionWithChallengeCredential(ctx, fakeTestUser, false /*isEphemeral*/, hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg))
		if err == nil {
			s.Fatal("Authenticated persistent user with wrong credentials: ", err)
		}

		// Verify that file is still there, but should not be readable.
		if _, err := ioutil.ReadFile(filePath); err == nil {
			s.Fatal("File is readable after unmount and unsuccessful authentication: ", err)
		}
	}
}
