// Copyright 2022 The ChromiumOS Authors
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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	uda "chromiumos/system_api/user_data_auth_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrossVersionLogin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies login functionality across the version",
		Contacts: []string{
			"chingkang@google.com",
			"cros-hwsec@chromium.org",
		},
		SoftwareDeps: []string{"chrome", "tpm2_simulator"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Name: "",
			// These data are generated on betty but could be used on both betty and
			// amd64-generic. However it could not be used on board with dynamic tpm,
			// since its TPM is bound to different PCR (for example, reven-vmtest).
			// Note that if the data could not used on other boards in the future,
			// we would need to split them to different test sets.
			ExtraSoftwareDeps: []string{"no_tpm_dynamic"},
			Timeout:           20 * time.Minute,
			Val: []string{
				"R88-13597.108.0-custombuild20220717_betty_20220719",
				"R89-13729.85.0-custombuild20220715_betty_20220719",
				"R90-13816.106.0-custombuild20220712_betty_20220719",
				"R91-13904.0.0_betty_20220816",
				"R91-13904.98.0-custombuild20220712_betty_20220719",
				"R92-13982.0.0_betty_20220816",
				"R92-13982.89.0-custombuild20220712_betty_20220719",
				"R93-14092.0.0_betty_20220816",
				"R93-14092.106.0-custombuild20220713_betty_20220719",
				"R94-14150.0.0_betty_20220712",
				"R94-14150.592.0-custombuild20220714_betty_20220720",
				"R96-14268.0.0_betty_20220712",
				"R96-14268.94.0-custombuild20220714_betty_20220719",
				"R97-14324.0.0_betty_20220712",
				"R97-14324.81.0-custombuild20220715_betty_20220719",
				"R98-14388.0.0_betty_20220712",
				"R98-14388.65.0-custombuild20220715_betty_20220719",
				"R99-14469.4.0_betty_20220712",
				"R99-14469.76.0-custombuild20220717_betty_20220719",
				"R100-14526.0.0_betty_20220712",
				"R100-14526.122.0-custombuild20220718_betty_20220719",
				"R101-14588.0.0_betty_20220712",
				"R101-14588.134.0-custombuild20220718_betty_20220719",
				"R102-14695.0.0_betty_20220712",
				"R102-14695.114.0-custombuild20220718_betty_20220719",
				"R103-14816.99.0_betty_20220712",
			},
			ExtraData: []string{
				// See cross_version_login/README.md on how to create these.
				"cross_version_login/R88-13597.108.0-custombuild20220717_betty_20220719_config.json",
				"cross_version_login/R88-13597.108.0-custombuild20220717_betty_20220719_data.tar.gz",
				"cross_version_login/R89-13729.85.0-custombuild20220715_betty_20220719_config.json",
				"cross_version_login/R89-13729.85.0-custombuild20220715_betty_20220719_data.tar.gz",
				"cross_version_login/R90-13816.106.0-custombuild20220712_betty_20220719_config.json",
				"cross_version_login/R90-13816.106.0-custombuild20220712_betty_20220719_data.tar.gz",
				"cross_version_login/R91-13904.0.0_betty_20220816_config.json",
				"cross_version_login/R91-13904.0.0_betty_20220816_data.tar.gz",
				"cross_version_login/R91-13904.98.0-custombuild20220712_betty_20220719_config.json",
				"cross_version_login/R91-13904.98.0-custombuild20220712_betty_20220719_data.tar.gz",
				"cross_version_login/R92-13982.0.0_betty_20220816_config.json",
				"cross_version_login/R92-13982.0.0_betty_20220816_data.tar.gz",
				"cross_version_login/R92-13982.89.0-custombuild20220712_betty_20220719_config.json",
				"cross_version_login/R92-13982.89.0-custombuild20220712_betty_20220719_data.tar.gz",
				"cross_version_login/R93-14092.0.0_betty_20220816_config.json",
				"cross_version_login/R93-14092.0.0_betty_20220816_data.tar.gz",
				"cross_version_login/R93-14092.106.0-custombuild20220713_betty_20220719_config.json",
				"cross_version_login/R93-14092.106.0-custombuild20220713_betty_20220719_data.tar.gz",
				"cross_version_login/R94-14150.0.0_betty_20220712_config.json",
				"cross_version_login/R94-14150.0.0_betty_20220712_data.tar.gz",
				"cross_version_login/R94-14150.592.0-custombuild20220714_betty_20220720_config.json",
				"cross_version_login/R94-14150.592.0-custombuild20220714_betty_20220720_data.tar.gz",
				"cross_version_login/R96-14268.0.0_betty_20220712_config.json",
				"cross_version_login/R96-14268.0.0_betty_20220712_data.tar.gz",
				"cross_version_login/R96-14268.94.0-custombuild20220714_betty_20220719_config.json",
				"cross_version_login/R96-14268.94.0-custombuild20220714_betty_20220719_data.tar.gz",
				"cross_version_login/R97-14324.0.0_betty_20220712_config.json",
				"cross_version_login/R97-14324.0.0_betty_20220712_data.tar.gz",
				"cross_version_login/R97-14324.81.0-custombuild20220715_betty_20220719_config.json",
				"cross_version_login/R97-14324.81.0-custombuild20220715_betty_20220719_data.tar.gz",
				"cross_version_login/R98-14388.0.0_betty_20220712_config.json",
				"cross_version_login/R98-14388.0.0_betty_20220712_data.tar.gz",
				"cross_version_login/R98-14388.65.0-custombuild20220715_betty_20220719_config.json",
				"cross_version_login/R98-14388.65.0-custombuild20220715_betty_20220719_data.tar.gz",
				"cross_version_login/R99-14469.4.0_betty_20220712_config.json",
				"cross_version_login/R99-14469.4.0_betty_20220712_data.tar.gz",
				"cross_version_login/R99-14469.76.0-custombuild20220717_betty_20220719_config.json",
				"cross_version_login/R99-14469.76.0-custombuild20220717_betty_20220719_data.tar.gz",
				"cross_version_login/R100-14526.0.0_betty_20220712_config.json",
				"cross_version_login/R100-14526.0.0_betty_20220712_data.tar.gz",
				"cross_version_login/R100-14526.122.0-custombuild20220718_betty_20220719_config.json",
				"cross_version_login/R100-14526.122.0-custombuild20220718_betty_20220719_data.tar.gz",
				"cross_version_login/R101-14588.0.0_betty_20220712_config.json",
				"cross_version_login/R101-14588.0.0_betty_20220712_data.tar.gz",
				"cross_version_login/R101-14588.134.0-custombuild20220718_betty_20220719_config.json",
				"cross_version_login/R101-14588.134.0-custombuild20220718_betty_20220719_data.tar.gz",
				"cross_version_login/R102-14695.0.0_betty_20220712_config.json",
				"cross_version_login/R102-14695.0.0_betty_20220712_data.tar.gz",
				"cross_version_login/R102-14695.114.0-custombuild20220718_betty_20220719_config.json",
				"cross_version_login/R102-14695.114.0-custombuild20220718_betty_20220719_data.tar.gz",
				"cross_version_login/R103-14816.99.0_betty_20220712_config.json",
				"cross_version_login/R103-14816.99.0_betty_20220712_data.tar.gz",
			},
		}, {
			Name:              "tpm_dynamic",
			ExtraSoftwareDeps: []string{"tpm_dynamic"},
			Timeout:           10 * time.Minute,
			Val: []string{
				"R96-14268.0.0_reven-vmtest_20220712",
				"R96-14268.94.0-custombuild20220715_reven-vmtest_20220719",
				"R97-14324.0.0_reven-vmtest_20220712",
				"R97-14324.81.0-custombuild20220716_reven-vmtest_20220719",
				"R98-14388.0.0_reven-vmtest_20220712",
				"R98-14388.65.0-custombuild20220719_reven-vmtest_20220719",
				"R99-14469.4.0_reven-vmtest_20220712",
				"R99-14469.76.0-custombuild20220718_reven-vmtest_20220719",
				"R100-14526.0.0_reven-vmtest_20220712",
				"R100-14526.122.0-custombuild20220718_reven-vmtest_20220719",
				"R101-14588.0.0_reven-vmtest_20220712",
				"R101-14588.134.0-custombuild20220718_reven-vmtest_20220719",
				"R102-14695.0.0_reven-vmtest_20220712",
				"R102-14695.114.0-custombuild20220718_reven-vmtest_20220719",
				"R103-14816.99.0_reven-vmtest_20220712",
			},
			ExtraData: []string{
				"cross_version_login/R96-14268.0.0_reven-vmtest_20220712_config.json",
				"cross_version_login/R96-14268.0.0_reven-vmtest_20220712_data.tar.gz",
				"cross_version_login/R96-14268.94.0-custombuild20220715_reven-vmtest_20220719_config.json",
				"cross_version_login/R96-14268.94.0-custombuild20220715_reven-vmtest_20220719_data.tar.gz",
				"cross_version_login/R97-14324.0.0_reven-vmtest_20220712_config.json",
				"cross_version_login/R97-14324.0.0_reven-vmtest_20220712_data.tar.gz",
				"cross_version_login/R97-14324.81.0-custombuild20220716_reven-vmtest_20220719_config.json",
				"cross_version_login/R97-14324.81.0-custombuild20220716_reven-vmtest_20220719_data.tar.gz",
				"cross_version_login/R98-14388.0.0_reven-vmtest_20220712_config.json",
				"cross_version_login/R98-14388.0.0_reven-vmtest_20220712_data.tar.gz",
				"cross_version_login/R98-14388.65.0-custombuild20220719_reven-vmtest_20220719_config.json",
				"cross_version_login/R98-14388.65.0-custombuild20220719_reven-vmtest_20220719_data.tar.gz",
				"cross_version_login/R99-14469.4.0_reven-vmtest_20220712_config.json",
				"cross_version_login/R99-14469.4.0_reven-vmtest_20220712_data.tar.gz",
				"cross_version_login/R99-14469.76.0-custombuild20220718_reven-vmtest_20220719_config.json",
				"cross_version_login/R99-14469.76.0-custombuild20220718_reven-vmtest_20220719_data.tar.gz",
				"cross_version_login/R100-14526.0.0_reven-vmtest_20220712_config.json",
				"cross_version_login/R100-14526.0.0_reven-vmtest_20220712_data.tar.gz",
				"cross_version_login/R100-14526.122.0-custombuild20220718_reven-vmtest_20220719_config.json",
				"cross_version_login/R100-14526.122.0-custombuild20220718_reven-vmtest_20220719_data.tar.gz",
				"cross_version_login/R101-14588.0.0_reven-vmtest_20220712_config.json",
				"cross_version_login/R101-14588.0.0_reven-vmtest_20220712_data.tar.gz",
				"cross_version_login/R101-14588.134.0-custombuild20220718_reven-vmtest_20220719_config.json",
				"cross_version_login/R101-14588.134.0-custombuild20220718_reven-vmtest_20220719_data.tar.gz",
				"cross_version_login/R102-14695.0.0_reven-vmtest_20220712_config.json",
				"cross_version_login/R102-14695.0.0_reven-vmtest_20220712_data.tar.gz",
				"cross_version_login/R102-14695.114.0-custombuild20220718_reven-vmtest_20220719_config.json",
				"cross_version_login/R102-14695.114.0-custombuild20220718_reven-vmtest_20220719_data.tar.gz",
				"cross_version_login/R103-14816.99.0_reven-vmtest_20220712_config.json",
				"cross_version_login/R103-14816.99.0_reven-vmtest_20220712_data.tar.gz",
			},
		}, {
			// To test data migration from the current device to itself. This is for verifying the functionality of hwsec.CrossVersionLogin and hwsec.PrepareCrossVersionLoginData.
			Name:      "current",
			ExtraAttr: []string{"informational"},
			Timeout:   3 * time.Minute,
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

func prepareChallengeAuth(ctx context.Context, lf hwsec.LogFunc, config *util.CrossVersionLoginConfig) (func(), error) {
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
	keyDelegate, err := hwsec.NewCryptohomeKeyDelegate(
		lf, dbusConn, username, authConfig.ChallengeAlgs, rsaKey, authConfig.ChallengeSPKI)
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
func testConfig(ctx context.Context, lf hwsec.LogFunc, cryptohome *hwsec.CryptohomeClient, config *util.CrossVersionLoginConfig) error {
	// Exercise the regular login flow as driven by Chrome Login Screen, to catch
	// any regressions in APIs between Cryptohomed and Chrome.
	// This part of the test is only possible when the snapshot contains mountable
	// user data and not just keysets (see cross_version_login_data.go for
	// details).
	if config.VaultFSType != util.NoVaultFS {
		if err := testConfigViaChrome(ctx, config); err != nil {
			return errors.Wrap(err, "failed to test config via Chrome")
		}
	}

	// Test various aspects of the Cryptohome D-Bus API as well.
	if err := testConfigViaCryptohome(ctx, lf, cryptohome, config); err != nil {
		return errors.Wrap(err, "failed to test config via Cryptohome")
	}

	return nil
}

// testConfigViaChrome verifies the login functionality via Chrome Login Screen.
func testConfigViaChrome(ctx context.Context, config *util.CrossVersionLoginConfig) error {
	authConfig := config.AuthConfig
	username := authConfig.Username

	// The smart card authentication is not currently supported in this subtest,
	// as it'd require loading fake smart card middleware extensions in Chrome.
	if authConfig.AuthType != hwsec.PassAuth {
		return nil
	}

	// Check password login.
	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: authConfig.Password}), chrome.KeepState())
	if err != nil {
		return errors.Wrap(err, "failed to log in with password")
	}
	// TODO(b/237120336): Check cryptohome was not recreated, by reading some file
	// that was previously put into the snapshot.
	if err := cr.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to log out after password login")
	}

	// TODO(b/237120336): Check PIN login as well.
	return nil
}

// testConfigViaCryptohome verifies the login functionality by making requests
// via Cryptohome CLI.
func testConfigViaCryptohome(ctx context.Context, lf hwsec.LogFunc, cryptohome *hwsec.CryptohomeClient, config *util.CrossVersionLoginConfig) error {
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
	if !cmp.Equal(labels, targetLabels, cmpopts.SortSlices(less)) {
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
		// VaultKeySet check
		if _, err := cryptohome.CheckVault(ctx, keyLabel, &authConfig); err != nil {
			return errors.Wrap(err, "failed to check vault")
		}
		// AuthSession check
		authID, err := cryptohome.StartAuthSession(ctx, username, false /* isEphemeral */, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		if err := cryptohome.AuthenticateChallengeCredentialWithAuthSession(ctx, authID, keyLabel, &authConfig); err != nil {
			return errors.Wrap(err, "failed to authenticate challenge credential with auth session")
		}
	case hwsec.PassAuth:
		// VaultKeySet check
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
		// AuthSession check
		authID, err := cryptohome.StartAuthSession(ctx, username, false /* isEphemeral */, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		if err := cryptohome.AuthenticateAuthSession(ctx, invalidPassword, keyLabel, authID, false); err == nil {
			return errors.Wrap(err, "unexpectedly authenticate auth session with invalid password")
		}
		authID, err = cryptohome.StartAuthSession(ctx, username, false /* isEphemeral */, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		if err := cryptohome.AuthenticateAuthSession(ctx, password, keyLabel, authID, false); err != nil {
			return errors.Wrap(err, "failed to authenticate auth session")
		}
	}

	if err := cryptohome.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount vaults")
	}
	if _, err := cryptohome.RemoveVault(ctx, username); err != nil {
		return errors.Wrap(err, "failed to remove vault")
	}
	return nil
}

// testVersion verifies the login functionality in the specific version.
func testVersion(ctx context.Context, lf hwsec.LogFunc, cryptohome *hwsec.CryptohomeClient, daemonController *hwsec.DaemonController, dataPath, configPath string) error {
	configJSON, err := ioutil.ReadFile(configPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read %q", configPath)
	}
	var configList []util.CrossVersionLoginConfig
	if err := json.Unmarshal(configJSON, &configList); err != nil {
		return errors.Wrap(err, "failed to read json")
	}

	if err := hwseclocal.LoadLoginData(ctx, daemonController, dataPath, true /*includeTpm*/); err != nil {
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
	if err := hwseclocal.SaveLoginData(ctx, daemonController, backupPath, true /*includeTpm*/); err != nil {
		s.Fatal("Failed to backup login data: ", err)
	}

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		// Load back the origin login data after the test.
		if err := hwseclocal.LoadLoginData(ctx, daemonController, backupPath, true /*includeTpm*/); err != nil {
			s.Fatal("Failed to load login data: ", err)
		}
	}(ctxForCleanUp)

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
