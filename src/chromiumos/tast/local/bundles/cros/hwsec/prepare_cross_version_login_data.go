// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
  "encoding/json"
  "os"
  "io/ioutil"
	"math/rand"
  "path/filepath"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/common/hwsec"
  "chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PrepareCrossVersionLoginData,
		Desc: "Create data for cross-version login test",
		Contacts: []string{
			"chingkang@google.com",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func CreateChallengeResponseData(ctx context.Context, lf logFunc, cryptohome *hwsec.CryptohomeClient) (*hwsec.AuthConfig, error) {
	const (
		dbusName    = "org.chromium.TestingCryptohomeKeyDelegate"
		testUser    = "testuser"
		keyLabel    = hwseclocal.CrossVersionLoginKeyLabel
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

	keyDelegate, err := newCryptohomeKeyDelegate(
		lf, dbusConn, testUser, keyAlg, rsaKey, pubKeySPKIDER)
	if err != nil {
		return nil, errors.Wrap(err, "failed to export D-Bus key delegate")
	}
	defer keyDelegate.close()

  config := hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.dbusPath, pubKeySPKIDER, keyAlg)
	// Create the challenge-response protected cryptohome.
	if err := cryptohome.MountVault(ctx, keyLabel, config, true, hwsec.NewVaultConfig()); err != nil {
		return nil, errors.Wrap(err, "failed to create cryptohome")
	}
	if keyDelegate.challengeCallCnt == 0 {
	  return nil, errors.New("no key challenges made during mount")
	}

	// Authenticate while the cryptohome is still mounted (modeling the case of
	// the user unlocking the device from the Lock Screen).
	if _, err := cryptohome.CheckVault(ctx, keyLabel, config); err != nil {
		return nil, errors.Wrap(err, "failed to check the key for the mounted cryptohome")
	}
  return config, nil
}

func CreatePasswordData(ctx context.Context) (*hwsec.AuthConfig, error) {
  // Add the new password login data
  cr, err := chrome.New(ctx)
  if err != nil {
    return nil, errors.Wrap(err, "failed to log in by Chrome")
  }
  cr.Close(ctx)
  return hwsec.NewPassAuthConfig(cr.Creds().User, cr.Creds().Pass), nil
}

func PrepareCrossVersionLoginData(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
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

  var authConfigList []hwsec.AuthConfig

  if authConfig, err := CreatePasswordData(ctx); err != nil {
    s.Fatal("Failed to create password data", err)
  } else {
    authConfigList = append(authConfigList, *authConfig)
  }

  if authConfig, err := CreateChallengeResponseData(ctx, s.Logf, cryptohome); err != nil {
    // We could not use tast to create challenge-response data before R96, so here we only log the error.
    s.Log("Failed to create challenge-response data: ", err)
  } else {
    authConfigList = append(authConfigList, *authConfig)
  }

  // Create compressed data
  tmpDir := "/tmp/cross_version_login"
	dataPath := filepath.Join(tmpDir, "data.tar.xz")
  if err := os.MkdirAll(tmpDir, 0700); err != nil {
    s.Fatalf("Failed to create directory '%s': %v", tmpDir, err)
  }

	if err := hwseclocal.CreateCrossVersionLoginData(ctx, daemonController, dataPath); err != nil {
		s.Fatal("Failed to create cross-version-login data: ", err)
	}

  // Create cross-version-login config
	configPath := filepath.Join(tmpDir, "config.json")
  config := hwseclocal.CrossVersionLoginConfig{
    AuthConfigList: authConfigList,
  }
  configJson, err := json.MarshalIndent(config, "", "  ")
  s.Log(string(configJson))
  if err != nil {
    s.Fatal("Failed to encode the cross-version-login config to json")
  }
  if err := ioutil.WriteFile(configPath, configJson, 0644); err != nil {
    s.Fatalf("Failed to write file to '%s': %v", configPath, err)
  }
}

