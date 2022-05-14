// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChallengeResponseMount,
		Desc: "Checks that the cryptohome challenge-response mount works",
		Contacts: []string{
			"emaxx@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"tpm"},
	})
}

func ChallengeResponseMount(ctx context.Context, s *testing.State) {
	const (
		dbusName    = "org.chromium.TestingCryptohomeKeyDelegate"
		testUser    = "cryptohome_test@chromium.org"
		keyLabel    = "testkey"
		keySizeBits = 2048
		keyAlg      = cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1
	)

	cmdRunner := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	utility := helper.CryptohomeClient()

	// Make sure the test starts from a missing cryptohome.
	utility.UnmountAndRemoveVault(ctx, testUser)
	// Clean up the cryptohome created by this test, if any, during shutdown.
	defer utility.UnmountAndRemoveVault(ctx, testUser)

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

	// Create the challenge-response protected cryptohome.
	if err := utility.MountVault(ctx, keyLabel, hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create cryptohome: ", err)
	}
	if keyDelegate.ChallengeCallCnt == 0 {
		s.Fatal("No key challenges made during mount")
	}

	// Authenticate while the cryptohome is still mounted (modeling the case of
	// the user unlocking the device from the Lock Screen).
	if _, err := utility.CheckVault(ctx, keyLabel, hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg)); err != nil {
		s.Fatal("Failed to check the key for the mounted cryptohome: ", err)
	}

	if _, err := utility.Unmount(ctx, testUser); err != nil {
		s.Fatal("Failed to unmount cryptohome: ", err)
	}

	// Mount the existing challenge-response protected cryptohome.
	keyDelegate.ChallengeCallCnt = 0
	if err := utility.MountVault(ctx, keyLabel, hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount existing cryptohome: ", err)
	}
	if keyDelegate.ChallengeCallCnt == 0 {
		s.Fatal("No key challenges made during mount")
	}
}
