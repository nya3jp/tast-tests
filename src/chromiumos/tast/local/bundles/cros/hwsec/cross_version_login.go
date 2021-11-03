// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
  "encoding/json"
  "fmt"
  "io/ioutil"

	"chromiumos/tast/common/testexec"
  //"chromiumos/tast/common/hwsec"
  "chromiumos/tast/errors"
	hwseclocal "chromiumos/tast/local/hwsec"
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
        "R91-13904.0.0_betty_20211124",
        "R92-13982.0.0_betty_20211124",
        "R93-14092.0.0_betty_20211124",
        "R94-14150.0.0_betty_20211124",
        "R96-14268.0.0_betty_20211124",
        "R97-14324.0.0_betty_20211124",
			},
			ExtraData: []string{
        "cross_version_login/R91-13904.0.0_betty_20211124_data.tar.xz",
        "cross_version_login/R92-13982.0.0_betty_20211124_data.tar.xz",
        "cross_version_login/R93-14092.0.0_betty_20211124_data.tar.xz",
        "cross_version_login/R94-14150.0.0_betty_20211124_data.tar.xz",
        "cross_version_login/R96-14268.0.0_betty_20211124_data.tar.xz",
        "cross_version_login/R97-14324.0.0_betty_20211124_data.tar.xz",
        "cross_version_login/R91-13904.0.0_betty_20211124_config.json",
        "cross_version_login/R92-13982.0.0_betty_20211124_config.json",
        "cross_version_login/R93-14092.0.0_betty_20211124_config.json",
        "cross_version_login/R94-14150.0.0_betty_20211124_config.json",
        "cross_version_login/R96-14268.0.0_betty_20211124_config.json",
        "cross_version_login/R97-14324.0.0_betty_20211124_config.json",
			},
		}},
	})
}

func isNewer(version, prefix string) (bool, error) {
  var a [3]int
  var b [3]int
  var dummy int
  if _, err := fmt.Sscanf(version, "%d.%d.%d", &a[0], &a[1], &a[2]); err != nil {
    return false, errors.Wrap(err, "failed to sscanf the version")
  }
  if _, err := fmt.Sscanf(prefix, "R%d-%d.%d.%d", &dummy, &b[0], &b[1], &b[2]); err != nil {
    return false, errors.Wrap(err, "failed to sscanf the prefix")
  }
  for i := 0; i < 3; i++ {
    if a[i] != b[i] {
      return a[i] > b[i], nil
    }
  }
  return false, nil
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
      s.Log("Skip testing login with: ", prefix)
      continue
    }
		s.Log("Test login with: ", prefix)
    dataName := fmt.Sprintf("cross_version_login/%s_data.tar.xz", prefix)
    configName := fmt.Sprintf("cross_version_login/%s_config.json", prefix)
		dataPath := s.DataPath(dataName)
    configPath := s.DataPath(configName)

		if err := hwseclocal.LoadCrossVersionLoginData(ctx, daemonController, dataPath); err != nil {
			s.Fatal("Failed to load login data: ", err)
		}
    configJson, err := ioutil.ReadFile(configPath)
    if err != nil {
      s.Fatalf("Failed to read open '%s': %v", configJson, err)
    }
    var config hwseclocal.CrossVersionLoginConfig
    if err := json.Unmarshal(configJson, &config); err != nil {
      s.Fatal("Failed tp read json: ", err)
    }

    for _, authConfig := range config.AuthConfigList {
      keyLabel := hwseclocal.CrossVersionLoginKeyLabel
      if authConfig.AuthType == 0 {
        if _, err := cryptohome.CheckVault(ctx, keyLabel, &authConfig); err != nil {
          s.Errorf("Failed to check vault for '%s' with auth type %d: %v", prefix, authConfig.AuthType, err)
        }
      }
      if _, err := cryptohome.ListVaultKeys(ctx, authConfig.Username); err != nil {
        s.Errorf("Failed to list vault keys for '%s' with auth type %d: %v", prefix, authConfig.AuthType, err)
      }
    }
	}
}
