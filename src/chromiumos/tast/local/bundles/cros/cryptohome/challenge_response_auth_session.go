// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

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
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func clearDevicePolicy(ctx context.Context) error {
	if err := session.SetUpDevice(ctx); err != nil {
		return errors.Wrap(err, "failed resetting device ownership")
	}
	return nil
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ChallengeResponseAuthSession,
		Desc: "Test adding and authenticating AuthSession through Challenge Credentials",
		Contacts: []string{
			"thomascedeno@google.com",
			"cryptohome-core@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"tpm"},
	})
}

func ChallengeResponseAuthSession(ctx context.Context, s *testing.State) {
	const (
		dbusName        = "org.chromium.TestingCryptohomeKeyDelegate"
		testUser        = "cryptohome_test@chromium.org"
		testFile        = "file"
		testFileContent = "content"
		keyLabel        = "testkey"
		keySizeBits     = 2048
		keyAlg          = cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1
	)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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

	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for preparation: ", err)
	}

	if err := cryptohome.RemoveVault(ctx, testUser); err != nil {
		s.Fatal("Failed to remove old vault for preparation: ", err)
	}

	// Use a pseudorandom generator with a fixed seed, to make the values used by
	// the test predictable.
	randReader := rand.New(rand.NewSource(0 /* seed */))

	// Make sure the ephemeral users device policy is not set.
	// TODO(crbug.com/1054004); Remove after Tast starts to guarantee that.
	if err := clearDevicePolicy(ctx); err != nil {
		s.Fatal("Failed to clear device policy: ", err)
	}

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

	if err := cryptohome.CreateUserAuthSessionWithChallengeCredential(ctx, testUser, false, hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg)); err != nil {
		s.Fatal("Failed to create the user: ", err)
	}
	defer cryptohome.RemoveVault(ctxForCleanUp, testUser)

	// Mount the vault for the first time.
	authSessionID, err := cryptohome.AuthenticateAuthSessionWithChallengeCredential(ctx, testUser, false, false, hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg))
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}
	defer client.UnmountAll(ctxForCleanUp)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err == nil {
		s.Fatal("Secondary prepare attempt for the same user should fail, but succeeded")
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

	// Unmount and mount again.
	if err := client.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount vaults for re-mounting: ", err)
	}

	authSessionID, err = cryptohome.AuthenticateAuthSessionWithChallengeCredential(ctx, testUser, false, false, hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg))
	if err != nil {
		s.Fatal("Failed to authenticate persistent user: ", err)
	}
	defer client.InvalidateAuthSession(ctxForCleanUp, authSessionID)

	if err := client.PreparePersistentVault(ctx, authSessionID, false); err != nil {
		s.Fatal("Failed to prepare persistent vault: ", err)
	}

	// Verify that file is still there.
	if content, err := ioutil.ReadFile(filePath); err != nil {
		s.Fatal("Failed to read back test file: ", err)
	} else if bytes.Compare(content, []byte(testFileContent)) != 0 {
		s.Fatalf("Incorrect tests file content. got: %q, want: %q", content, testFileContent)
	}
}
