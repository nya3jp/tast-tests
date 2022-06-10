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
	"path/filepath"
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies login functionality accross the version",
		Contacts: []string{
			"chingkang@google.com",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "tpm2_simulator"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "",
			// These data are generated on betty but could be used on both betty and
			// amd64-generic. However it could not be used on board with dynamic tpm,
			// since its TPM is bound to different PCR (for example, reven-vmtest).
			// Note that if the data could not used on other boards in the future,
			// we would need to split them to different test sets.
			ExtraSoftwareDeps: []string{"no_tpm_dynamic"},
			ExtraAttr:         []string{"group:mainline"},
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
				"R102-14695.0.0_betty_20220510",
			},
			ExtraData: []string{
				"cross_version_login/R91-13904.0.0_betty_20211216_config.json",
				"cross_version_login/R91-13904.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R92-13982.0.0_betty_20211216_config.json",
				"cross_version_login/R92-13982.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R93-14092.0.0_betty_20211216_config.json",
				"cross_version_login/R93-14092.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R94-14150.0.0_betty_20211216_config.json",
				"cross_version_login/R94-14150.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R96-14268.0.0_betty_20211216_config.json",
				"cross_version_login/R96-14268.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R97-14324.0.0_betty_20211216_config.json",
				"cross_version_login/R97-14324.0.0_betty_20211216_data.tar.gz",
				"cross_version_login/R98-14388.0.0_betty_20220105_config.json",
				"cross_version_login/R98-14388.0.0_betty_20220105_data.tar.gz",
				"cross_version_login/R99-14469.4.0_betty_20220322_config.json",
				"cross_version_login/R99-14469.4.0_betty_20220322_data.tar.gz",
				"cross_version_login/R100-14526.0.0_betty_20220322_config.json",
				"cross_version_login/R100-14526.0.0_betty_20220322_data.tar.gz",
				"cross_version_login/R101-14588.0.0_betty_20220322_config.json",
				"cross_version_login/R101-14588.0.0_betty_20220322_data.tar.gz",
				"cross_version_login/R102-14695.0.0_betty_20220510_config.json",
				"cross_version_login/R102-14695.0.0_betty_20220510_data.tar.gz",
			},
		}, {
			Name:              "tpm_dynamic",
			ExtraSoftwareDeps: []string{"tpm_dynamic"},
			ExtraAttr:         []string{"group:mainline"},
			Val: []string{
				"R96-14268.0.0_reven-vmtest_20220103",
				"R97-14324.0.0_reven-vmtest_20220103",
				"R98-14388.0.0_reven-vmtest_20220105",
				"R99-14469.4.0_reven-vmtest_20220322",
				"R100-14526.0.0_reven-vmtest_20220322",
				"R101-14588.0.0_reven-vmtest_20220322",
				"R102-14695.0.0_reven-vmtest_20220510",
			},
			ExtraData: []string{
				"cross_version_login/R96-14268.0.0_reven-vmtest_20220103_config.json",
				"cross_version_login/R96-14268.0.0_reven-vmtest_20220103_data.tar.gz",
				"cross_version_login/R97-14324.0.0_reven-vmtest_20220103_config.json",
				"cross_version_login/R97-14324.0.0_reven-vmtest_20220103_data.tar.gz",
				"cross_version_login/R98-14388.0.0_reven-vmtest_20220105_config.json",
				"cross_version_login/R98-14388.0.0_reven-vmtest_20220105_data.tar.gz",
				"cross_version_login/R99-14469.4.0_reven-vmtest_20220322_config.json",
				"cross_version_login/R99-14469.4.0_reven-vmtest_20220322_data.tar.gz",
				"cross_version_login/R100-14526.0.0_reven-vmtest_20220322_config.json",
				"cross_version_login/R100-14526.0.0_reven-vmtest_20220322_data.tar.gz",
				"cross_version_login/R101-14588.0.0_reven-vmtest_20220322_config.json",
				"cross_version_login/R101-14588.0.0_reven-vmtest_20220322_data.tar.gz",
				"cross_version_login/R102-14695.0.0_reven-vmtest_20220510_config.json",
				"cross_version_login/R102-14695.0.0_reven-vmtest_20220510_data.tar.gz",
			},
		}, {
			// To test data migration from the current device to itself. This is for verifying the functionality of hwsec.CrossVersionLogin and hwsec.PrepareCrossVersionLoginData.
			Name:      "current",
			ExtraAttr: []string{"group:mainline", "informational"},
			Val:       []string{},
			ExtraData: []string{},
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
	const (
		newPassword = "newPass"
		newLabel    = "newLabel"
	)
	authConfig := config.AuthConfig
	rsaKey := config.RsaKey
	keyLabel := config.KeyLabel
	username := authConfig.Username
	password := authConfig.Password

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
			lf, dbusConn, username, authConfig.ChallengeAlg, rsaKey, authConfig.ChallengeSPKI)
		if err != nil {
			return errors.Wrap(err, "failed to export D-Bus key delegate")
		}
		defer keyDelegate.Close()
	}

	if _, err := cryptohome.CheckVault(ctx, keyLabel, &authConfig); err != nil {
		return errors.Wrap(err, "failed to check vault")
	}
	if _, err := cryptohome.ListVaultKeys(ctx, username); err != nil {
		return errors.Wrap(err, "failed to list vault keys")
	}
	if authConfig.AuthType == hwsec.PassAuth {
		if err := cryptohome.AddVaultKey(ctx, username, password, config.KeyLabel, newPassword, newLabel, false); err != nil {
			return errors.Wrap(err, "failed to add key")
		}
		if _, err := cryptohome.CheckVault(ctx, newLabel, hwsec.NewPassAuthConfig(username, newPassword)); err != nil {
			return errors.Wrap(err, "failed to check vault with new key")
		}
		if err := cryptohome.RemoveVaultKey(ctx, username, password, newLabel); err != nil {
			return errors.Wrap(err, "failed to remove key")
		}
	}
	return nil
}

func testVersion(ctx context.Context, lf util.LogFunc, cryptohome *hwsec.CryptohomeClient, daemonController *hwsec.DaemonController, dataPath, configPath string) error {
	configJSON, err := ioutil.ReadFile(configPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read open %q", configPath)
	}
	var configList []util.CrossVersionLoginConfig
	if err := json.Unmarshal(configJSON, &configList); err != nil {
		return errors.Wrap(err, "failed tp read json")
	}

	if err := util.LoadCrossVersionLoginData(ctx, daemonController, dataPath); err != nil {
		return errors.Wrap(err, "failed to load login data")
	}
	for _, config := range configList {
		if err := testConfig(ctx, lf, cryptohome, &config); err != nil {
			return errors.Wrapf(err, "failed to test auth type %d", config.AuthConfig.AuthType)
		}
	}
	return nil
}

func CrossVersionLogin(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
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

	const tmpDir = "/tmp/cross_version_login"
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		s.Fatalf("Failed to create directory %q: %v", tmpDir, err)
	}
	defer os.RemoveAll(tmpDir)
	// Creates backup data to recover state later.
	backupPath := filepath.Join(tmpDir, "backup_data.tar.xz")
	if err := util.CreateCrossVersionLoginData(ctx, daemonController, backupPath); err != nil {
		s.Fatal("Failed to backup login data: ", err)
	}
	defer func() {
		if err := util.LoadCrossVersionLoginData(ctx, daemonController, backupPath); err != nil {
			s.Fatal("Failed to load login data: ", err)
		}
	}()

	prefixs := s.Param().([]string)
	if len(prefixs) == 0 {
		// The prefixs is empty, so the local data migration would be tested instead.
		dataPath := filepath.Join(tmpDir, "data.tar.gz")
		configPath := filepath.Join(tmpDir, "config.json")
		s.Log("Preparing login data of current version")
		if err := util.PrepareCrossVersionLoginData(ctx, s.Logf, cryptohome, daemonController, dataPath, configPath); err != nil {
			s.Fatal("Failed to prepare login data for current version: ", err)
		}
		s.Log("Testing login with current version")
		if err := testVersion(ctx, s.Logf, cryptohome, daemonController, dataPath, configPath); err != nil {
			s.Fatal("Failed to test current version: ", err)
		}
	} else {
		// Run test with prefix, Rxx-x.x.x_<board>_<date>, e.g. R91-13904.0.0_betty_20211206.
		for _, prefix := range prefixs {
			prefixVersion, err := prefixToVersion(prefix)
			if err != nil {
				s.Fatal("Failed to convert prefix to version: ", err)
			}
			if !isNewer(version, prefixVersion) {
				s.Logf("Skipping testing login with %s because it is newer than current image", prefix)
				continue
			}
			s.Log("Testing login with ", prefix)
			dataName := fmt.Sprintf("cross_version_login/%s_data.tar.gz", prefix)
			configName := fmt.Sprintf("cross_version_login/%s_config.json", prefix)
			dataPath := s.DataPath(dataName)
			configPath := s.DataPath(configName)

			if err := testVersion(ctx, s.Logf, cryptohome, daemonController, dataPath, configPath); err != nil {
				s.Errorf("Failed to test version %q: %v", prefix, err)
			}
		}
	}
}
