// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrepareCrossVersionLoginData,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Create snapshot of login-related data, which will be used in hwsec.CrossVersionLogin to mock the login data in older version (see go/cros-cross-version-login-testing)",
		Contacts: []string{
			"chingkang@google.com",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "tpm2_simulator"},
	})
}

func createChallengeResponseData(ctx context.Context, lf util.LogFunc, cryptohome *hwsec.CryptohomeClient) (*util.CrossVersionLoginConfig, error) {
	const (
		dbusName    = "org.chromium.TestingCryptohomeKeyDelegate"
		testUser    = "challenge_response_test@chromium.org"
		keyLabel    = "challenge_response_key_label"
		keySizeBits = 2048
		keyAlg      = cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1
	)
	randReader := rand.New(rand.NewSource(0 /* seed */))
	rsaKey, err := rsa.GenerateKey(randReader, keySizeBits)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate RSA key")
	}
	pubKeySPKIDER, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate SubjectPublicKeyInfo")
	}

	dbusConn, err := dbusutil.SystemBus()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to system D-Bus bus")
	}
	if _, err := dbusConn.RequestName(dbusName, 0 /* flags */); err != nil {
		return nil, errors.Wrap(err, "failed to request the well-known D-Bus name")
	}
	defer dbusConn.ReleaseName(dbusName)

	keyDelegate, err := util.NewCryptohomeKeyDelegate(
		lf, dbusConn, testUser, keyAlg, rsaKey, pubKeySPKIDER)
	if err != nil {
		return nil, errors.Wrap(err, "failed to export D-Bus key delegate")
	}
	defer keyDelegate.Close()

	authConfig := hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlg)
	// Create the challenge-response protected cryptohome.
	if err := cryptohome.MountVault(ctx, keyLabel, authConfig, true, hwsec.NewVaultConfig()); err != nil {
		return nil, errors.Wrap(err, "failed to create cryptohome")
	}
	if keyDelegate.ChallengeCallCnt == 0 {
		return nil, errors.New("no key challenges made during mount")
	}
	if _, err := cryptohome.CheckVault(ctx, keyLabel, authConfig); err != nil {
		return nil, errors.Wrap(err, "failed to check the key for the mounted cryptohome")
	}
	return util.NewChallengeAuthCrossVersionLoginConfig(authConfig, keyLabel, rsaKey), nil
}

func createPasswordData(ctx context.Context) (*util.CrossVersionLoginConfig, error) {
	// Add the new password login data
	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to log in by Chrome")
	}
	cr.Close(ctx)
	authConfig := hwsec.NewPassAuthConfig(cr.Creds().User, cr.Creds().Pass)
	return util.NewPassAuthCrossVersionLoginConfig(authConfig, "legacy-0"), nil
}

func PrepareCrossVersionLoginData(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	s.Log("Restarting ui job")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	var configList []util.CrossVersionLoginConfig

	defer func() {
		for _, config := range configList {
			username := config.AuthConfig.Username
			if err := cryptohome.UnmountAndRemoveVault(ctx, username); err != nil {
				s.Errorf("Failed to remove user vault for %q: %v", username, err)
			}
		}
	}()

	if config, err := createPasswordData(ctx); err != nil {
		s.Fatal("Failed to create password data: ", err)
	} else {
		configList = append(configList, *config)
	}

	s.Log("Restarting ui job")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	if config, err := createChallengeResponseData(ctx, s.Logf, cryptohome); err != nil {
		// We could not use latest tast to create challenge-response data before R96, so here we only log the error.
		s.Log("Failed to create challenge-response data: ", err)
	} else {
		configList = append(configList, *config)
	}

	const tmpDir = "/tmp/cross_version_login"
	dataPath := filepath.Join(tmpDir, "data.tar.gz")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		s.Fatalf("Failed to create directory %q: %v", tmpDir, err)
	}

	// Note that if the format of either CrossVersionLoginConfigData or CrossVersionLoginConfig is changed,
	// the hwsec.CrossVersionLogin should be modified and the generated data should be regenerated.
	// Create compressed data for mocking the login data in this version, which will be used in hwsec.CrossVersionLogin.
	if err := util.CreateCrossVersionLoginData(ctx, daemonController, dataPath); err != nil {
		s.Fatal("Failed to create cross-version-login data: ", err)
	}
	// Create JSON file of CrossVersionLoginConfig object in order to record which login method we needed to test in hwsec.CrossVersionLogin.
	configPath := filepath.Join(tmpDir, "config.json")
	configJSON, err := json.MarshalIndent(configList, "", "  ")
	s.Log(string(configJSON))
	if err != nil {
		s.Fatal("Failed to encode the cross-version-login config to json")
	}
	if err := ioutil.WriteFile(configPath, configJSON, 0644); err != nil {
		s.Fatalf("Failed to write file to %q: %v", configPath, err)
	}
}
