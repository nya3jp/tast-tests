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
	"reflect"
	"sort"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

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
				"R90-13816.106.0_betty_20220704",
				"R91-13904.0.0_betty_20220610",
				"R92-13982.0.0_betty_20220610",
				"R93-14092.0.0_betty_20220610",
				"R94-14150.0.0_betty_20220610",
				"R96-14268.0.0_betty_20220610",
				"R97-14324.0.0_betty_20220610",
				"R98-14388.0.0_betty_20220610",
				"R99-14469.4.0_betty_20220610",
				"R100-14526.0.0_betty_20220610",
				"R101-14588.0.0_betty_20220610",
				"R102-14695.0.0_betty_20220610",
			},
			ExtraData: []string{
				// See cross_version_login/README.md on how to create these.
				"cross_version_login/R90-13816.106.0_betty_20220704_config.json",
				"cross_version_login/R90-13816.106.0_betty_20220704_data.tar.gz",
				"cross_version_login/R91-13904.0.0_betty_20220610_config.json",
				"cross_version_login/R91-13904.0.0_betty_20220610_data.tar.gz",
				"cross_version_login/R92-13982.0.0_betty_20220610_config.json",
				"cross_version_login/R92-13982.0.0_betty_20220610_data.tar.gz",
				"cross_version_login/R93-14092.0.0_betty_20220610_config.json",
				"cross_version_login/R93-14092.0.0_betty_20220610_data.tar.gz",
				"cross_version_login/R94-14150.0.0_betty_20220610_config.json",
				"cross_version_login/R94-14150.0.0_betty_20220610_data.tar.gz",
				"cross_version_login/R96-14268.0.0_betty_20220610_config.json",
				"cross_version_login/R96-14268.0.0_betty_20220610_data.tar.gz",
				"cross_version_login/R97-14324.0.0_betty_20220610_config.json",
				"cross_version_login/R97-14324.0.0_betty_20220610_data.tar.gz",
				"cross_version_login/R98-14388.0.0_betty_20220610_config.json",
				"cross_version_login/R98-14388.0.0_betty_20220610_data.tar.gz",
				"cross_version_login/R99-14469.4.0_betty_20220610_config.json",
				"cross_version_login/R99-14469.4.0_betty_20220610_data.tar.gz",
				"cross_version_login/R100-14526.0.0_betty_20220610_config.json",
				"cross_version_login/R100-14526.0.0_betty_20220610_data.tar.gz",
				"cross_version_login/R101-14588.0.0_betty_20220610_config.json",
				"cross_version_login/R101-14588.0.0_betty_20220610_data.tar.gz",
				"cross_version_login/R102-14695.0.0_betty_20220610_config.json",
				"cross_version_login/R102-14695.0.0_betty_20220610_data.tar.gz",
			},
		}, {
			Name:              "tpm_dynamic",
			ExtraSoftwareDeps: []string{"tpm_dynamic"},
			Val: []string{
				"R96-14268.0.0_reven-vmtest_20220610",
				"R97-14324.0.0_reven-vmtest_20220610",
				"R98-14388.0.0_reven-vmtest_20220610",
				"R99-14469.4.0_reven-vmtest_20220610",
				"R100-14526.0.0_reven-vmtest_20220610",
				"R101-14588.0.0_reven-vmtest_20220610",
				"R102-14695.0.0_reven-vmtest_20220610",
			},
			ExtraData: []string{
				"cross_version_login/R96-14268.0.0_reven-vmtest_20220610_config.json",
				"cross_version_login/R96-14268.0.0_reven-vmtest_20220610_data.tar.gz",
				"cross_version_login/R97-14324.0.0_reven-vmtest_20220610_config.json",
				"cross_version_login/R97-14324.0.0_reven-vmtest_20220610_data.tar.gz",
				"cross_version_login/R98-14388.0.0_reven-vmtest_20220610_config.json",
				"cross_version_login/R98-14388.0.0_reven-vmtest_20220610_data.tar.gz",
				"cross_version_login/R99-14469.4.0_reven-vmtest_20220610_config.json",
				"cross_version_login/R99-14469.4.0_reven-vmtest_20220610_data.tar.gz",
				"cross_version_login/R100-14526.0.0_reven-vmtest_20220610_config.json",
				"cross_version_login/R100-14526.0.0_reven-vmtest_20220610_data.tar.gz",
				"cross_version_login/R101-14588.0.0_reven-vmtest_20220610_config.json",
				"cross_version_login/R101-14588.0.0_reven-vmtest_20220610_data.tar.gz",
				"cross_version_login/R102-14695.0.0_reven-vmtest_20220610_config.json",
				"cross_version_login/R102-14695.0.0_reven-vmtest_20220610_data.tar.gz",
			},
		}, {
			// To test data migration from the current device to itself. This is for verifying the functionality of hwsec.CrossVersionLogin and hwsec.PrepareCrossVersionLoginData.
			Name:      "current",
			ExtraAttr: []string{"informational"},
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

// arraysEqualUnordered checks the equality of two string arrays unorderedly
func arraysEqualUnordered(a, b []string) bool {
	sortedA := make([]string, len(a))
	copy(sortedA, a)
	sort.Strings(sortedA)

	sortedB := make([]string, len(b))
	copy(sortedB, b)
	sort.Strings(sortedB)
	return reflect.DeepEqual(sortedA, sortedB)
}

// testCheckKey tests that CheckVault() works as expected
func testCheckKey(ctx context.Context, cryptohome *hwsec.CryptohomeClient, username string, keyInfo *util.VaultKeyInfo, invalidPassword string) error {
	if _, err := cryptohome.CheckVault(ctx, keyInfo.KeyLabel, hwsec.NewPassAuthConfig(username, keyInfo.Password)); err != nil {
		return errors.Wrap(err, "failed to check vault")
	}
	if _, err := cryptohome.CheckVault(ctx, keyInfo.KeyLabel, hwsec.NewPassAuthConfig(username, invalidPassword)); err == nil {
		return errors.New("unexpectedly can check vault with invalid password")
	}
	return nil
}

// testRemoveKey tests that RemoveVaultKey() works as expected
func testRemoveKey(ctx context.Context, cryptohome *hwsec.CryptohomeClient, username, password string, keyInfo *util.VaultKeyInfo) error {
	if err := cryptohome.RemoveVaultKey(ctx, username, password, keyInfo.KeyLabel); err != nil {
		return errors.Wrap(err, "failed to remove key")
	}
	if _, err := cryptohome.CheckVault(ctx, keyInfo.KeyLabel, hwsec.NewPassAuthConfig(username, keyInfo.Password)); err == nil {
		return errors.New("unexpectedly can the check vault with removed key")
	}
	return nil
}

// testAddRemoveKey tests that AddVaultKey() and RemoveVaultKey() works as expected
func testAddRemoveKey(ctx context.Context, cryptohome *hwsec.CryptohomeClient, username, password, label string, keyInfo *util.VaultKeyInfo, invalidPassword string) error {
	if err := cryptohome.AddVaultKey(ctx, username, password, label, keyInfo.Password, keyInfo.KeyLabel, keyInfo.LowEntropy); err != nil {
		return errors.Wrap(err, "failed to add key")
	}
	if err := testCheckKey(ctx, cryptohome, username, keyInfo, invalidPassword); err != nil {
		return errors.Wrap(err, "failed to properly check key")
	}
	if err := testRemoveKey(ctx, cryptohome, username, password, keyInfo); err != nil {
		return errors.Wrap(err, "failed to properly remove key")
	}
	return nil
}

// testMigrateKey tests that ChangeVaultPassword() works as expected
func testMigrateKey(ctx context.Context, cryptohome *hwsec.CryptohomeClient, username, oldPassword, label, changedPassword, invalidPassword string) error {
	if err := cryptohome.ChangeVaultPassword(ctx, username, invalidPassword, label, changedPassword); err == nil {
		return errors.New("unexpectedly can change vault password with invalid password")
	}
	if err := cryptohome.ChangeVaultPassword(ctx, username, oldPassword, label, changedPassword); err != nil {
		return errors.Wrap(err, "failed to change vault password")
	}
	if err := testCheckKey(ctx, cryptohome, username, util.NewVaultKeyInfo(changedPassword, label, false), oldPassword); err != nil {
		return errors.Wrap(err, "failed to properly check key after password is changed")
	}
	if err := cryptohome.ChangeVaultPassword(ctx, username, changedPassword, label, oldPassword); err != nil {
		return errors.Wrap(err, "failed to change vault password back")
	}
	if err := testCheckKey(ctx, cryptohome, username, util.NewVaultKeyInfo(oldPassword, label, false), changedPassword); err != nil {
		return errors.Wrap(err, "failed to properly check key after password is changed back")
	}
	return nil
}

func hasSharedElement(lhs, rhs []string) bool {
	for _, l := range lhs {
		for _, r := range rhs {
			if l == r {
				return true
			}
		}
	}
	return false
}

func prepareChallengeAuth(ctx context.Context, lf util.LogFunc, config *util.CrossVersionLoginConfig) (func(), error) {
	authConfig := config.AuthConfig
	rsaKey := config.RsaKey
	username := authConfig.Username

	dbusConn, err := dbusutil.SystemBus()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to system D-Bus bus")
	}
	if _, err := dbusConn.RequestName(authConfig.KeyDelegateName, 0 /* flags */); err != nil {
		return nil, errors.Wrap(err, "failed to request the well-known D-Bus name")
	}
	keyDelegate, err := util.NewCryptohomeKeyDelegate(
		lf, dbusConn, username, authConfig.ChallengeAlg, rsaKey, authConfig.ChallengeSPKI)
	if err != nil {
		return nil, errors.Wrap(err, "failed to export D-Bus key delegate")
	}

	cleanup := func() {
		dbusConn.ReleaseName(authConfig.KeyDelegateName)
		keyDelegate.Close()
	}
	return cleanup, nil
}

// testConfig verifies the login functionality of specific auth config from CrossVersionLoginConfig.
func testConfig(ctx context.Context, lf util.LogFunc, cryptohome *hwsec.CryptohomeClient, config *util.CrossVersionLoginConfig) error {
	const (
		newPasswordLabel = "newPasswordLabel"
		newPinLabel      = "newPinLabel"
		newPassword      = "testpass"
		newPin           = "654321"
		invalidPassword  = "wrongPass"
		invalidPin       = "000000"
		changedPassword  = "changedPass"
	)
	authConfig := config.AuthConfig
	keyLabel := config.KeyLabel
	username := authConfig.Username
	password := authConfig.Password

	// Preparation and pre-check
	var targetLabels = []string{keyLabel}
	for _, vaultKey := range config.ExtraVaultKeys {
		targetLabels = append(targetLabels, vaultKey.KeyLabel)
	}
	if usedLabels := []string{newPasswordLabel, newPinLabel}; hasSharedElement(targetLabels, usedLabels) {
		return errors.Errorf("Some labels in config are identical to the labels we would use: %q vs %q", targetLabels, usedLabels)
	}

	switch authConfig.AuthType {
	case hwsec.ChallengeAuth:
		cleanup, err := prepareChallengeAuth(ctx, lf, config)
		if err != nil {
			return errors.Wrap(err, "failed to prepare challenge auth")
		}
		defer cleanup()
	case hwsec.PassAuth:
		var targetPasswords = []string{password}
		for _, vaultKey := range config.ExtraVaultKeys {
			targetPasswords = append(targetPasswords, vaultKey.Password)
		}
		if usedPasswords := []string{changedPassword, invalidPassword, invalidPin}; hasSharedElement(targetPasswords, usedPasswords) {
			return errors.Errorf("some passwords in config are identical to the passwords we would use: %q vs %q", targetPasswords, usedPasswords)
		}
	default:
		return errors.Errorf("unknown auth type %d", authConfig.AuthType)
	}

	// Common check
	labels, err := cryptohome.ListVaultKeys(ctx, username)
	if err != nil {
		return errors.Wrap(err, "failed to list keys")
	}
	less := func(a, b string) bool { return a < b }
	if diff := cmp.Diff(labels, targetLabels, cmpopts.SortSlices(less)); diff != "" {
		return errors.Errorf("mismatch result from list keys, got %q, expected %q", labels, targetLabels)
	}
	for _, label := range targetLabels {
		if _, err := cryptohome.GetKeyData(ctx, username, label); err != nil {
			return errors.Wrapf(err, "failed to get data of key %q", label)
		}
	}

	// Auth-type specific check
	switch authConfig.AuthType {
	case hwsec.ChallengeAuth:
		if _, err := cryptohome.CheckVault(ctx, keyLabel, &authConfig); err != nil {
			return errors.Wrap(err, "failed to check vault")
		}
	case hwsec.PassAuth:
		if err := testCheckKey(ctx, cryptohome, username, util.NewVaultKeyInfo(password, keyLabel, false), invalidPassword); err != nil {
			return errors.Wrap(err, "failed to properly check key")
		}
		for _, vaultKey := range config.ExtraVaultKeys {
			keyForm := "password"
			invalidSecret := invalidPassword
			if vaultKey.LowEntropy {
				keyForm = "pin"
				invalidSecret = invalidPin
			}
			if err := testCheckKey(ctx, cryptohome, username, &vaultKey, invalidSecret); err != nil {
				return errors.Wrapf(err, "failed to properly check key with extra %s key", keyForm)
			}
			if err := testRemoveKey(ctx, cryptohome, username, password, &vaultKey); err != nil {
				return errors.Wrapf(err, "failed to properly remove key with extra %s key", keyForm)
			}
		}

		if err := testAddRemoveKey(ctx, cryptohome, username, password, keyLabel, util.NewVaultKeyInfo(newPassword, newPasswordLabel, false), invalidPassword); err != nil {
			return errors.Wrap(err, "failed to properly add or remove password key")
		}
		if err := testAddRemoveKey(ctx, cryptohome, username, password, keyLabel, util.NewVaultKeyInfo(newPin, newPinLabel, true), invalidPin); err != nil {
			return errors.Wrap(err, "failed to properly add or remove pin key")
		}
		if err := testMigrateKey(ctx, cryptohome, username, password, keyLabel, changedPassword, invalidPassword); err != nil {
			return errors.Wrap(err, "failed to properly migrate key")
		}
	}

	if _, err := cryptohome.RemoveVault(ctx, username); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}

// testVersion verifies the login functionality in the specific version.
func testVersion(ctx context.Context, lf util.LogFunc, cryptohome *hwsec.CryptohomeClient, daemonController *hwsec.DaemonController, dataPath, configPath string) error {
	configJSON, err := ioutil.ReadFile(configPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q", configPath)
	}
	var configList []util.CrossVersionLoginConfig
	if err := json.Unmarshal(configJSON, &configList); err != nil {
		return errors.Wrap(err, "failed to read json")
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
