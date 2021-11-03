// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrossVersionLogin,
		Desc: "Login test accross the version",
		Contacts: []string{
			"chingkang@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:              "betty",
			ExtraHardwareDeps: hwdep.D(hwdep.Model("betty")),
			Val: []string{
				"R91-13904.0.0_betty_20211130",
				"R92-13982.0.0_betty_20211130",
				"R93-14092.0.0_betty_20211130",
				"R94-14150.0.0_betty_20211130",
				"R96-14268.0.0_betty_20211130",
				"R97-14324.0.0_betty_20211130",
			},
			ExtraData: []string{
				"cross_version_login/R91-13904.0.0_betty_20211130_data.tar.xz",
				"cross_version_login/R91-13904.0.0_betty_20211130_config.json",
				"cross_version_login/R92-13982.0.0_betty_20211130_data.tar.xz",
				"cross_version_login/R92-13982.0.0_betty_20211130_config.json",
				"cross_version_login/R93-14092.0.0_betty_20211130_data.tar.xz",
				"cross_version_login/R93-14092.0.0_betty_20211130_config.json",
				"cross_version_login/R94-14150.0.0_betty_20211130_data.tar.xz",
				"cross_version_login/R94-14150.0.0_betty_20211130_config.json",
				"cross_version_login/R96-14268.0.0_betty_20211130_data.tar.xz",
				"cross_version_login/R96-14268.0.0_betty_20211130_config.json",
				"cross_version_login/R97-14324.0.0_betty_20211130_data.tar.xz",
				"cross_version_login/R97-14324.0.0_betty_20211130_config.json",
			},
		}},
	})
}

func isNewer(version, prefix string) (bool, error) {
	var a [3]int
	var b [3]int
	var placeholder int
	if _, err := fmt.Sscanf(version, "%d.%d.%d", &a[0], &a[1], &a[2]); err != nil {
		return false, errors.Wrap(err, "failed to sscanf the version")
	}
	if _, err := fmt.Sscanf(prefix, "R%d-%d.%d.%d", &placeholder, &b[0], &b[1], &b[2]); err != nil {
		return false, errors.Wrap(err, "failed to sscanf the prefix")
	}
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] > b[i], nil
		}
	}
	return false, nil
}

func testConfig(ctx context.Context, lf hwseclocal.LogFunc, cryptohome *hwsec.CryptohomeClient, config *hwseclocal.CrossVersionLoginConfig) error {
	authConfig := config.AuthConfig
	rsaKey := config.RsaKey
	const (
		dbusName    = "org.chromium.TestingCryptohomeKeyDelegate"
		testUser    = "testuser"
		keyLabel    = hwseclocal.CrossVersionLoginKeyLabel
		keySizeBits = 2048
		keyAlg      = cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1
	)

	if authConfig.AuthType == 1 {
		dbusConn, err := dbusutil.SystemBus()
		if err != nil {
			return errors.Wrap(err, "failed to connect to system D-Bus bus")
		}
		if _, err := dbusConn.RequestName(dbusName, 0 /* flags */); err != nil {
			return errors.Wrap(err, "failed to request the well-known D-Bus name")
		}
		defer dbusConn.ReleaseName(dbusName)

		keyDelegate, err := hwseclocal.NewCryptohomeKeyDelegate(
			lf, dbusConn, testUser, keyAlg, rsaKey, authConfig.ChallengeSPKI)
		if err != nil {
			return errors.Wrap(err, "failed to export D-Bus key delegate")
		}
		defer keyDelegate.Close()
	}

	if _, err := cryptohome.CheckVault(ctx, keyLabel, &authConfig); err != nil {
		return errors.Wrapf(err, "failed to check vault with auth type %d", authConfig.AuthType)
	}
	if _, err := cryptohome.ListVaultKeys(ctx, authConfig.Username); err != nil {
		return errors.Wrapf(err, "failed to list vault keys with auth type %d", authConfig.AuthType)
	}
	return nil
}

func CrossVersionLogin(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	daemonController := helper.DaemonController()
	cryptohome := helper.CryptohomeClient()

	lsb, err := lsbrelease.Load()
	if err != nil {
		s.Fatal("Failed to read lsbrelease: ", err)
	}
	version, ok := lsb[lsbrelease.Version]
	if !ok {
		s.Error("Failed to get ChromeOS Version")
	}

	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	// Creates backup data to recover state later.
	backupPath := "/tmp/cross_version_login_backup_data.tar.xz"
	if err := hwseclocal.CreateCrossVersionLoginData(ctx, daemonController, backupPath); err != nil {
		s.Fatal("Failed to backup login data: ", err)
	}
	defer func() {
		if err := hwseclocal.LoadCrossVersionLoginData(ctx, daemonController, backupPath); err != nil {
			s.Fatal("Failed to load login data: ", err)
		}
		if err := testexec.CommandContext(ctx, "rm", "-f", backupPath).Run(); err != nil {
			s.Fatal("Failed to remove backup data: ", err)
		}
	}()

	prefixs := s.Param().([]string)
	for _, prefix := range prefixs {
		if newer, err := isNewer(version, prefix); err != nil {
			s.Fatal("Failed to check if version is newer than prefix: ", err)
		} else if !newer {
			s.Logf("Skip testing login with %s because it is newer than current image", prefix)
			continue
		}
		s.Log("Test login with ", prefix)
		dataName := fmt.Sprintf("cross_version_login/%s_data.tar.xz", prefix)
		configName := fmt.Sprintf("cross_version_login/%s_config.json", prefix)
		dataPath := s.DataPath(dataName)
		configPath := s.DataPath(configName)

		configJSON, err := ioutil.ReadFile(configPath)
		if err != nil {
			s.Errorf("Failed to read open %q: %v", configJSON, err)
			continue
		}
		var configList []hwseclocal.CrossVersionLoginConfig
		if err := json.Unmarshal(configJSON, &configList); err != nil {
			s.Error("Failed tp read json: ", err)
			continue
		}

		func() {
			if err := hwseclocal.LoadCrossVersionLoginData(ctx, daemonController, dataPath); err != nil {
				s.Error("Failed to load login data: ", err)
				return
			}

			for _, config := range configList {
				if err := testConfig(ctx, s.Logf, cryptohome, &config); err != nil {
					s.Errorf("Failed to test version %q: %v", prefix, err)
				}
			}
		}()
	}
}
