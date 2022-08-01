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
	"strings"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
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

func runCmdOrFailWithOut(cmd *testexec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Return programs's output on failures. Avoid line breaks in error
		// messages, to keep the Tast logs readable.
		outFlat := strings.Replace(string(out), "\n", " ", -1)
		return errors.Wrap(err, outFlat)
	}
	return nil
}

func decompressData(ctx context.Context, src string) error {
	// Use the "tar" program as it takes care of recursive unpacking,
	// preserving ownership, permissions and SELinux attributes.
	cmd := testexec.CommandContext(ctx, "/bin/tar",
		"--extract",              // extract files from an archive
		"--gzip",                 // filter the archive through gunzip
		"--preserve-permissions", // extract file permissions
		"--same-owner",           // extract file ownership
		"--file",                 // read from the file specified in the next argument
		src)
	// Set the work directory for "tar" at "/", so that it unpacks files
	// at correct locations.
	cmd.Dir = "/"
	return runCmdOrFailWithOut(cmd)
}

func compressData(ctx context.Context, dst string, paths, ignorePaths []string) error {
	// Use the "tar" program as it takes care of recursive packing,
	// preserving ownership, permissions and SELinux attributes.
	args := append([]string{
		"--acls",    // save the ACLs to the archive
		"--create",  // create a new archive
		"--gzip",    // filter the archive through gzip
		"--selinux", // save the SELinux context to the archive
		"--xattrs",  // save the user/root xattrs to the archive
		"--file",    // write to the file specified in the next argument
		dst})
	for _, p := range ignorePaths {
		// Exclude the specified patterns from archiving.
		args = append(args, "--exclude", p)
	}
	// Specify the input paths to archive.
	args = append(args, paths...)
	cmd := testexec.CommandContext(ctx, "/bin/tar", args...)
	return runCmdOrFailWithOut(cmd)
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

// CreateCrossVersionLoginData creates the compressed file of data that is used in cross-version login test.
func CreateCrossVersionLoginData(ctx context.Context, daemonController *hwsec.DaemonController, archivePath string) error {
	if err := stopHwsecDaemons(ctx, daemonController); err != nil {
		return err
	}
	defer ensureHwsecDaemons(ctx, daemonController)

	paths := []string{
		"/mnt/stateful_partition/unencrypted/tpm2-simulator/NVChip",
		"/home/.shadow",
		"/home/chronos",
	}
	// Skip packing the "mount" directories, since the file systems it's
	// used for don't allow taking snapshots. E.g., ext4 fscrypt complains
	// "Required key not available" when trying to read encrypted files.
	ignorePaths := []string{
		"/home/.shadow/*/mount",
	}
	if err := compressData(ctx, archivePath, paths, ignorePaths); err != nil {
		return errors.Wrap(err, "failed to compress the cryptohome data")
	}
	return nil
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

// LoadCrossVersionLoginData loads the data that is used in cross-version login test.
func LoadCrossVersionLoginData(ctx context.Context, daemonController *hwsec.DaemonController, archivePath string) error {
	if err := stopHwsecDaemons(ctx, daemonController); err != nil {
		return err
	}
	defer ensureHwsecDaemons(ctx, daemonController)

	// Remove the `/home/.shadow` first to prevent any unexpected file remaining.
	if err := os.RemoveAll("/home/.shadow"); err != nil {
		return errors.Wrap(err, "failed to remove old /home/.shadow data")
	}
	// Clean up `/home/chronos` as well (note that deleting this directory itself would fail).
	if err := removeAllChildren("/home/chronos"); err != nil {
		return errors.Wrap(err, "failed to remove old /home/chronos data")
	}

	if err := decompressData(ctx, archivePath); err != nil {
		return errors.Wrap(err, "failed to decompress the cryptohome data")
	}

	// Run `restorecon` to make sure SELinux attributes are correct after the decompression.
	if err := testexec.CommandContext(ctx, "restorecon", "-r", "/home/.shadow").Run(); err != nil {
		return errors.Wrap(err, "failed to restore selinux attributes")
	}
	return nil
}

func stopHwsecDaemons(ctx context.Context, daemonController *hwsec.DaemonController) error {
	if err := daemonController.TryStop(ctx, hwsec.UIDaemon); err != nil {
		return errors.Wrap(err, "failed to try to stop UI")
	}
	if err := daemonController.TryStopDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to try to stop high-level TPM daemons")
	}
	if err := daemonController.TryStopDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
		return errors.Wrap(err, "failed to try to stop low-level TPM daemons")
	}
	if err := daemonController.TryStop(ctx, hwsec.TPM2SimulatorDaemon); err != nil {
		return errors.Wrap(err, "failed to try to stop tpm2-simulator")
	}
	return nil
}

func ensureHwsecDaemons(ctx context.Context, daemonController *hwsec.DaemonController) {
	if err := daemonController.Ensure(ctx, hwsec.TPM2SimulatorDaemon); err != nil {
		testing.ContextLog(ctx, "Failed to ensure tpm2-simulator: ", err)
	}
	if err := daemonController.EnsureDaemons(ctx, hwsec.LowLevelTPMDaemons); err != nil {
		testing.ContextLog(ctx, "Failed to ensure low-level TPM daemons: ", err)
	}
	if err := daemonController.EnsureDaemons(ctx, hwsec.HighLevelTPMDaemons); err != nil {
		testing.ContextLog(ctx, "Failed to ensure high-level TPM daemons: ", err)
	}
	if err := daemonController.Ensure(ctx, hwsec.UIDaemon); err != nil {
		testing.ContextLog(ctx, "Failed to ensure UI: ", err)
	}
}

func createChallengeResponseData(ctx context.Context, lf LogFunc, cryptohome *hwsec.CryptohomeClient) (*CrossVersionLoginConfig, error) {
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

	keyDelegate, err := NewCryptohomeKeyDelegate(
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
func PrepareCrossVersionLoginData(ctx context.Context, lf LogFunc, cryptohome *hwsec.CryptohomeClient, daemonController *hwsec.DaemonController, dataPath, configPath string) (retErr error) {
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
	if err := CreateCrossVersionLoginData(ctx, daemonController, dataPath); err != nil {
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
