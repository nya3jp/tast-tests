// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrossVersionLogin,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies login functionality accross the version",
		Contacts: []string{
			"chingkang@google.com",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "tpm2_simulator"},
		Attr:         []string{"group:mainline"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "",
			// These data are generated on betty but could be used on both betty and
			// amd64-generic. However it could not be used on board with dynamic tpm,
			// since its TPM is bound to different PCR (for example, reven-vmtest).
			// Note that if the data could not used on other boards in the future,
			// we would need to split them to different test sets.
			ExtraSoftwareDeps: []string{"no_tpm_dynamic"},
			Val: []string{
				"R91-13904.0.0_betty_20211216",
				"R92-13982.0.0_betty_20211216",
				"R93-14092.0.0_betty_20211216",
				"R94-14150.0.0_betty_20211216",
				"R96-14268.0.0_betty_20211216",
				"R97-14324.0.0_betty_20211216",
				"R98-14388.0.0_betty_20220105",
				"R99-14469.4.0_betty_20220322",
				"R100-14526.0.0_betty_20220322",
				"R101-14588.0.0_betty_20220322",
			},
			ExtraData: []string{
				"cross_version_login/R91-13904.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R91-13904.0.0_betty_20211216_config.json",
				"cross_version_login/R92-13982.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R92-13982.0.0_betty_20211216_config.json",
				"cross_version_login/R93-14092.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R93-14092.0.0_betty_20211216_config.json",
				"cross_version_login/R94-14150.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R94-14150.0.0_betty_20211216_config.json",
				"cross_version_login/R96-14268.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R96-14268.0.0_betty_20211216_config.json",
				"cross_version_login/R97-14324.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R97-14324.0.0_betty_20211216_config.json",
				"cross_version_login/R98-14388.0.0_betty_20220105_data.tar.gz",
				"cross_version_login/R98-14388.0.0_betty_20220105_config.json",
				"cross_version_login/R99-14469.4.0_betty_20220322_data.tar.gz",
				"cross_version_login/R99-14469.4.0_betty_20220322_config.json",
				"cross_version_login/R100-14526.0.0_betty_20220322_data.tar.gz",
				"cross_version_login/R100-14526.0.0_betty_20220322_config.json",
				"cross_version_login/R101-14588.0.0_betty_20220322_data.tar.gz",
				"cross_version_login/R101-14588.0.0_betty_20220322_config.json",
			},
		}, {
			Name:              "tpm_dynamic",
			ExtraSoftwareDeps: []string{"tpm_dynamic"},
			Val: []string{
				"R96-14268.0.0_reven-vmtest_20220103",
				"R97-14324.0.0_reven-vmtest_20220103",
				"R98-14388.0.0_reven-vmtest_20220105",
				"R99-14469.4.0_reven-vmtest_20220322",
				"R100-14526.0.0_reven-vmtest_20220322",
				"R101-14588.0.0_reven-vmtest_20220322",
			},
			ExtraData: []string{
				"cross_version_login/R96-14268.0.0_reven-vmtest_20220103_config.json",
				"cross_version_login/R96-14268.0.0_reven-vmtest_20220103_data.tar.gz",
				"cross_version_login/R97-14324.0.0_reven-vmtest_20220103_config.json",
				"cross_version_login/R97-14324.0.0_reven-vmtest_20220103_data.tar.gz",
				"cross_version_login/R98-14388.0.0_reven-vmtest_20220105_config.json",
				"cross_version_login/R98-14388.0.0_reven-vmtest_20220105_data.tar.gz",
				"cross_version_login/R99-14469.4.0_reven-vmtest_20220322_data.tar.gz",
				"cross_version_login/R99-14469.4.0_reven-vmtest_20220322_config.json",
				"cross_version_login/R100-14526.0.0_reven-vmtest_20220322_data.tar.gz",
				"cross_version_login/R100-14526.0.0_reven-vmtest_20220322_config.json",
				"cross_version_login/R101-14588.0.0_reven-vmtest_20220322_data.tar.gz",
				"cross_version_login/R101-14588.0.0_reven-vmtest_20220322_config.json",
			},
		}},
	})
}

// isNewer compares which version (e.g 13904.0.0, 13904.94.0) is newer.
func isNewer(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// prefixToVersion convert prefix to [3]int (e.g R91-13904.0.0... to [13904, 0, 0])
func prefixToVersion(prefix string) ([3]int, error) {
	var version [3]int
	var milestone int
	if _, err := fmt.Sscanf(prefix, "R%d-%d.%d.%d", &milestone, &version[0], &version[1], &version[2]); err != nil {
		return [3]int{}, errors.Wrap(err, "failed to sscanf the prefix")
	}
	return version, nil
}

// testConfig verifies the login functionality of specific auth config from CrossVersionLoginConfig.
func testConfig(ctx context.Context, lf util.LogFunc, cryptohome *hwsec.CryptohomeClient, config *util.CrossVersionLoginConfig) error {
	authConfig := config.AuthConfig
	rsaKey := config.RsaKey
	keyLabel := config.KeyLabel

	if authConfig.AuthType == hwsec.ChallengeAuth {
		dbusConn, err := dbusutil.SystemBus()
		if err != nil {
			return errors.Wrap(err, "failed to connect to system D-Bus bus")
		}
		if _, err := dbusConn.RequestName(authConfig.KeyDelegateName, 0 /* flags */); err != nil {
			return errors.Wrap(err, "failed to request the well-known D-Bus name")
		}
		defer dbusConn.ReleaseName(authConfig.KeyDelegateName)

		keyDelegate, err := util.NewCryptohomeKeyDelegate(
			lf, dbusConn, authConfig.Username, authConfig.ChallengeAlg, rsaKey, authConfig.ChallengeSPKI)
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
	versionString, ok := lsb[lsbrelease.Version]
	if !ok {
		s.Error("Failed to get ChromeOS Version")
	}
	var version [3]int
	if _, err := fmt.Sscanf(versionString, "%d.%d.%d", &version[0], &version[1], &version[2]); err != nil {
		s.Fatal("Failed to sscanf the version string: ", err)
	}

	// Creates backup data to recover state later.
	const backupPath = "/tmp/cross_version_login_backup_data.tar.xz"
	if err := util.CreateCrossVersionLoginData(ctx, daemonController, backupPath); err != nil {
		s.Fatal("Failed to backup login data: ", err)
	}
	defer func() {
		if err := util.LoadCrossVersionLoginData(ctx, daemonController, backupPath); err != nil {
			s.Fatal("Failed to load login data: ", err)
		}
		if err := os.Remove(backupPath); err != nil {
			s.Fatal("Failed to remove backup data: ", err)
		}
	}()

	// Run test with prefix, Rxx-x.x.x_<board>_<date>, e.g. R91-13904.0.0_betty_20211206.
	prefixs := s.Param().([]string)
	for _, prefix := range prefixs {
		prefixVersion, err := prefixToVersion(prefix)
		if err != nil {
			s.Fatal("Failed to convert prefix to version: ", err)
		}
		if !isNewer(version, prefixVersion) {
			s.Logf("Skip testing login with %s because it is newer than current image", prefix)
			continue
		}
		s.Log("Test login with ", prefix)
		dataName := fmt.Sprintf("cross_version_login/%s_data.tar.gz", prefix)
		configName := fmt.Sprintf("cross_version_login/%s_config.json", prefix)
		dataPath := s.DataPath(dataName)
		configPath := s.DataPath(configName)

		configJSON, err := ioutil.ReadFile(configPath)
		if err != nil {
			s.Errorf("Failed to read open %q: %v", configJSON, err)
			continue
		}
		var configList []util.CrossVersionLoginConfig
		if err := json.Unmarshal(configJSON, &configList); err != nil {
			s.Error("Failed tp read json: ", err)
			continue
		}

		if err := util.LoadCrossVersionLoginData(ctx, daemonController, dataPath); err != nil {
			s.Error("Failed to load login data: ", err)
			continue
		}
		for _, config := range configList {
			if err := testConfig(ctx, s.Logf, cryptohome, &config); err != nil {
				s.Errorf("Failed to test version %q: %v", prefix, err)
			}
		}
	}
}
