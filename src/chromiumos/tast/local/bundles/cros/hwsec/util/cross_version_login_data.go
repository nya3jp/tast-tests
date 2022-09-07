// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// VaultKeyInfo contains the information of a vault key
type VaultKeyInfo struct {
	Password   string
	KeyLabel   string
	LowEntropy bool
}

// NewVaultKeyInfo creates VaultKeyInfo from password, label and lowEntropy
func NewVaultKeyInfo(password, label string, lowEntropy bool) *VaultKeyInfo {
	info := &VaultKeyInfo{
		Password:   password,
		KeyLabel:   label,
		LowEntropy: lowEntropy,
	}
	return info
}

// VaultFSType indicates the type of the file system used for the user vault.
type VaultFSType int

const (
	// NoVaultFS represents the absence of the user vault in the snapshot.
	NoVaultFS VaultFSType = iota
	// ECRYPTFSVaultFS represents the usage of eCryptfs for the user vault.
	ECRYPTFSVaultFS
)

// CrossVersionLoginConfig contains the information for cross-version login
type CrossVersionLoginConfig struct {
	AuthConfig     hwsec.AuthConfig
	RsaKey         *rsa.PrivateKey
	KeyLabel       string
	ExtraVaultKeys []VaultKeyInfo
	VaultFSType    VaultFSType
}

// NewPassAuthCrossVersionLoginConfig creates cross version-login config from password auth config
func NewPassAuthCrossVersionLoginConfig(authConfig *hwsec.AuthConfig, keyLabel string) *CrossVersionLoginConfig {
	config := &CrossVersionLoginConfig{
		AuthConfig: *authConfig,
		KeyLabel:   keyLabel,
	}
	return config
}

// AddVaultKeyData adds a vault key to cryptohome and store the VaultKeyInfo to the config
func (config *CrossVersionLoginConfig) AddVaultKeyData(ctx context.Context, cryptohome *hwsec.CryptohomeClient, info *VaultKeyInfo) error {
	authConfig := config.AuthConfig
	username := authConfig.Username
	password := authConfig.Password
	keyLabel := config.KeyLabel
	if err := cryptohome.AddVaultKey(ctx, username, password, keyLabel, info.Password, info.KeyLabel, info.LowEntropy); err != nil {
		return errors.Wrap(err, "failed to add key")
	}
	if _, err := cryptohome.CheckVault(ctx, info.KeyLabel, hwsec.NewPassAuthConfig(username, info.Password)); err != nil {
		return errors.Wrap(err, "failed to check vault with new key")
	}
	config.ExtraVaultKeys = append(config.ExtraVaultKeys, *info)
	return nil
}

// NewChallengeAuthCrossVersionLoginConfig creates cross-version login config from challenge auth config and rsa key
func NewChallengeAuthCrossVersionLoginConfig(authConfig *hwsec.AuthConfig, keyLabel string, rsaKey *rsa.PrivateKey) *CrossVersionLoginConfig {
	config := &CrossVersionLoginConfig{
		AuthConfig: *authConfig,
		KeyLabel:   keyLabel,
		RsaKey:     rsaKey,
	}
	return config
}

// removeAllChildren deletes all files and folders from the specified directory.
func removeAllChildren(dirPath string) error {
	dir, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return errors.Wrap(err, "failed to read dir")
	}
	firstErr := error(nil)
	for _, f := range dir {
		fullPath := path.Join([]string{dirPath, f.Name()}...)
		if err := os.RemoveAll(fullPath); err != nil {
			// Continue even after seeing an error, to at least attempt
			// deleting other files.
			firstErr = errors.Wrapf(err, "failed to remove %s", f)
		}
	}
	return firstErr
}

func createChallengeResponseData(ctx context.Context, lf hwsec.LogFunc, cryptohome *hwsec.CryptohomeClient) (*CrossVersionLoginConfig, error) {
	const (
		dbusName    = "org.chromium.TestingCryptohomeKeyDelegate"
		testUser    = "challenge_response_test@chromium.org"
		keyLabel    = "challenge_response_key_label"
		keySizeBits = 2048
	)
	// Enable all RSASSA algorithms, with SHA-1 at the top, as most real-world
	// smart cards support all of them.
	keyAlgs := []cpb.ChallengeSignatureAlgorithm{
		cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1,
		cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA256,
		cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA384,
		cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA512,
	}

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

	keyDelegate, err := hwsec.NewCryptohomeKeyDelegate(
		lf, dbusConn, testUser, keyAlgs, rsaKey, pubKeySPKIDER)
	if err != nil {
		return nil, errors.Wrap(err, "failed to export D-Bus key delegate")
	}
	defer keyDelegate.Close()

	authConfig := hwsec.NewChallengeAuthConfig(testUser, dbusName, keyDelegate.DBusPath, pubKeySPKIDER, keyAlgs)
	// Enforce the usage of ecryptfs, so that we can take a usable snapshot of encrypted files.
	vaultConfig := hwsec.NewVaultConfig()
	vaultConfig.Ecryptfs = true
	// Create the challenge-response protected cryptohome.
	if err := cryptohome.MountVault(ctx, keyLabel, authConfig, true, vaultConfig); err != nil {
		return nil, errors.Wrap(err, "failed to create cryptohome")
	}
	if keyDelegate.ChallengeCallCnt == 0 {
		return nil, errors.New("no key challenges made during mount")
	}
	if _, err := cryptohome.CheckVault(ctx, keyLabel, authConfig); err != nil {
		return nil, errors.Wrap(err, "failed to check the key for the mounted cryptohome")
	}

	// It's expected that the ecryptfs was used because we specified `Ecryptfs` in `vaultConfig`.
	ecryptfsExists, err := ecryptfsVaultExists(ctx, cryptohome, testUser)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check ecryptfs presence")
	}
	if !ecryptfsExists {
		return nil, errors.New("no ecryptfs user vault found")
	}

	config := NewChallengeAuthCrossVersionLoginConfig(authConfig, keyLabel, rsaKey)
	config.VaultFSType = ECRYPTFSVaultFS
	return config, nil
}

func createPasswordData(ctx context.Context, cryptohome *hwsec.CryptohomeClient, supportsLE bool) (*CrossVersionLoginConfig, error) {
	const (
		extraPass  = "extraPass"
		extraLabel = "extraLabel"
		pin        = "123456"
		pinLabel   = "pinLabel"
	)
	// Add the new password login data. Enforce the usage of ecryptfs, so that we can take a usable snapshot of encrypted files.
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--cryptohome-use-old-encryption-for-testing"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to log in by Chrome")
	}
	cr.Close(ctx)
	username := cr.Creds().User
	password := cr.Creds().Pass

	labels, err := cryptohome.ListVaultKeys(ctx, username)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list key label")
	}
	if len(labels) != 1 {
		return nil, errors.Errorf("expected exactly 1 label, but got %v", labels)
	}

	authConfig := hwsec.NewPassAuthConfig(username, password)
	config := NewPassAuthCrossVersionLoginConfig(authConfig, labels[0])
	ecryptfsExists, err := ecryptfsVaultExists(ctx, cryptohome, username)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check ecryptfs presence")
	}
	if ecryptfsExists {
		config.VaultFSType = ECRYPTFSVaultFS
	}

	if err := config.AddVaultKeyData(ctx, cryptohome, NewVaultKeyInfo(extraPass, extraLabel, false)); err != nil {
		return nil, errors.Wrap(err, "failed to add vault key data of password")
	}
	if supportsLE {
		if err := config.AddVaultKeyData(ctx, cryptohome, NewVaultKeyInfo(pin, pinLabel, true)); err != nil {
			return nil, errors.Wrap(err, "failed to add vault key data of pin")
		}
	}
	return config, nil
}

// ecryptfsVaultExists returns whether ecryptfs vault exists for the given user.
func ecryptfsVaultExists(ctx context.Context, cryptohome *hwsec.CryptohomeClient, username string) (bool, error) {
	sanitizedUsername, err := cryptohome.GetSanitizedUsername(ctx, username, false /* useDBus */)
	if err != nil {
		return false, errors.Wrap(err, "failed to sanitize username")
	}
	ecryptfsVaultDir := fmt.Sprintf("/home/.shadow/%s/vault", sanitizedUsername)
	if _, err := os.Stat(ecryptfsVaultDir); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to perform stat")
	}
	return true, nil
}

// PrepareCrossVersionLoginData prepares the login data and config for CrossVersionLogin and saves them to dataPath and configPath respectively
func PrepareCrossVersionLoginData(ctx context.Context, lf hwsec.LogFunc, cryptohome *hwsec.CryptohomeClient, daemonController *hwsec.DaemonController, dataPath, configPath string) (retErr error) {
	var configList []CrossVersionLoginConfig

	defer func() {
		for _, config := range configList {
			username := config.AuthConfig.Username
			if err := cryptohome.UnmountAndRemoveVault(ctx, username); err != nil {
				if retErr == nil {
					retErr = errors.Wrapf(err, "failed to remove user vault for %q", username)
				} else {
					testing.ContextLogf(ctx, "Failed to remove user vaultf for %q: %v", username, err)
				}
			}
		}
	}()

	supportsLE, err := cryptohome.SupportsLECredentials(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get supported policies")
	} else if !supportsLE {
		testing.ContextLog(ctx, "Device does not support PinWeaver")
	}

	if err := daemonController.Restart(ctx, hwsec.UIDaemon); err != nil {
		return errors.Wrap(err, "failed to restart UI")
	}
	config, err := createPasswordData(ctx, cryptohome, supportsLE)
	if err != nil {
		return errors.Wrap(err, "failed to create password data")
	}
	configList = append(configList, *config)

	if err := daemonController.Restart(ctx, hwsec.UIDaemon); err != nil {
		return errors.Wrap(err, "failed to restart UI")
	}
	config, err = createChallengeResponseData(ctx, lf, cryptohome)
	if err != nil {
		// We could not use latest tast to create challenge-response data before R96, so here we only log the error.
		testing.ContextLog(ctx, "Failed to create challenge-response data: ", err)
	} else {
		configList = append(configList, *config)
	}

	// Note that if the format of either CrossVersionLoginConfigData or CrossVersionLoginConfig is changed,
	// the hwsec.CrossVersionLogin should be modified and the generated data should be regenerated.
	// Create compressed data for mocking the login data in this version, which will be used in hwsec.CrossVersionLogin.
	if err := hwseclocal.SaveLoginData(ctx, daemonController, dataPath, true /*includeTpm*/); err != nil {
		return errors.Wrap(err, "failed to create cross-version-login data")
	}
	// Create JSON file of CrossVersionLoginConfig object in order to record which login method we needed to test in hwsec.CrossVersionLogin.
	configJSON, err := json.MarshalIndent(configList, "", "  ")
	testing.ContextLog(ctx, "Generated config: ", string(configJSON))
	if err != nil {
		return errors.Wrap(err, "failed to encode the cross-version-login config to json")
	}
	if err := ioutil.WriteFile(configPath, configJSON, 0644); err != nil {
		return errors.Wrapf(err, "failed to write file to %q", configPath)
	}
	return nil
}
