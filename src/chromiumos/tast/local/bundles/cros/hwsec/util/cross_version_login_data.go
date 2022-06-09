// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"math/rand"
	"path/filepath"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

// CrossVersionLoginConfig contains the information for cross-version login
type CrossVersionLoginConfig struct {
	AuthConfig hwsec.AuthConfig
	RsaKey     *rsa.PrivateKey
	KeyLabel   string
}

// NewPassAuthCrossVersionLoginConfig creates cross version-login config from password auth config
func NewPassAuthCrossVersionLoginConfig(authConfig *hwsec.AuthConfig, keyLabel string) *CrossVersionLoginConfig {
	config := &CrossVersionLoginConfig{
		AuthConfig: *authConfig,
		KeyLabel:   keyLabel,
	}
	return config
}

func decompressData(src string) error {
	r, err := os.Open(src)
	if err != nil {
		return errors.Wrapf(err, "failed to open compressed data %q", src)
	}
	defer r.Close()

	gr, err := gzip.NewReader(r)
	if err != nil {
		return errors.Wrap(err, "failed to create gzip reader")
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "failed to read compressed data")
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(hdr.Name, 0777); err != nil {
				return errors.Wrapf(err, "failed to create directory %q", hdr.Name)
			}
		case tar.TypeReg:
			dir := filepath.Dir(hdr.Name)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return errors.Wrapf(err, "failed to create directory %q", dir)
			}
			f, err := os.OpenFile(hdr.Name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return errors.Wrapf(err, "failed to create file %q", hdr.Name)
			}
			defer f.Close()
			if _, err = io.Copy(f, tr); err != nil {
				return errors.Wrapf(err, "failed to decompress %q", hdr.Name)
			}
		}
	}
	return nil
}

func compressData(dst string, paths []string) error {
	w, err := os.Create(dst)
	if err != nil {
		return errors.Wrapf(err, "failed to create file %q", dst)
	}
	defer w.Close()
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, path := range paths {
		err := filepath.Walk(path, func(fn string, info os.FileInfo, err error) error {
			if err != nil {
				return errors.Wrapf(err, "failed to walk %q", fn)
			}
			// Ignore mount directories since we could not migrate them.
			if info.IsDir() && info.Name() == "mount" {
				return filepath.SkipDir
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			if err := archiveFile(tw, fn, info); err != nil {
				return errors.Wrapf(err, "failed to archive file %q", fn)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func archiveFile(tw *tar.Writer, fn string, info os.FileInfo) error {
	f, err := os.Open(fn)
	if err != nil {
		return errors.Wrap(err, "failed to open file")
	}
	defer f.Close()

	hdr, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return errors.Wrap(err, "failed to generate file header")
	}
	// Use the full path instead of basename.
	hdr.Name = fn

	if err := tw.WriteHeader(hdr); err != nil {
		return errors.Wrap(err, "failed to write header")
	}
	if _, err := io.Copy(tw, f); err != nil {
		return errors.Wrap(err, "failed to copy file to archive")
	}
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

// CreateCrossVersionLoginData creates the compressed file of data that is used in cross-version login test.
func CreateCrossVersionLoginData(ctx context.Context, daemonController *hwsec.DaemonController, archivePath string) error {
	if err := stopHwsecDaemons(ctx, daemonController); err != nil {
		return err
	}
	defer ensureHwsecDaemons(ctx, daemonController)

	files := []string{
		"/mnt/stateful_partition/unencrypted/tpm2-simulator/NVChip",
		"/home/.shadow",
	}
	if err := compressData(archivePath, files); err != nil {
		return errors.Wrap(err, "failed to compress the cryptohome data")
	}
	return nil
}

// LoadCrossVersionLoginData loads the data that is used in cross-version login test.
func LoadCrossVersionLoginData(ctx context.Context, daemonController *hwsec.DaemonController, archivePath string) error {
	if err := stopHwsecDaemons(ctx, daemonController); err != nil {
		return err
	}
	defer ensureHwsecDaemons(ctx, daemonController)

	// Remove the `/home/.shadow` first to prevent any unexpected file remaining.
	if err := os.RemoveAll("/home/.shadow"); err != nil {
		return errors.Wrap(err, "failed to remove old data")
	}

	if err := decompressData(archivePath); err != nil {
		return errors.Wrap(err, "failed to decompress the cryptohome data")
	}

	// decompressData do not restore selinux attributes. Running `restorecon` should do the trick.
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

	keyDelegate, err := NewCryptohomeKeyDelegate(
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
	return NewChallengeAuthCrossVersionLoginConfig(authConfig, keyLabel, rsaKey), nil
}

func createPasswordData(ctx context.Context) (*CrossVersionLoginConfig, error) {
	// Add the new password login data
	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to log in by Chrome")
	}
	cr.Close(ctx)
	authConfig := hwsec.NewPassAuthConfig(cr.Creds().User, cr.Creds().Pass)
	return NewPassAuthCrossVersionLoginConfig(authConfig, "legacy-0"), nil
}

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

	if err := daemonController.Restart(ctx, hwsec.UIDaemon); err != nil {
		return errors.Wrap(err, "failed to restart UI")
	}

	if config, err := createPasswordData(ctx); err != nil {
		return errors.Wrap(err, "failed to create password data")
	} else {
		configList = append(configList, *config)
	}

	if err := daemonController.Restart(ctx, hwsec.UIDaemon); err != nil {
		return errors.Wrap(err, "failed to restart UI")
	}

	if config, err := createChallengeResponseData(ctx, lf, cryptohome); err != nil {
		// We could not use latest tast to create challenge-response data before R96, so here we only log the error.
		return errors.Wrap(err, "failed to create challenge-response data")
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
		return errors.Wrapf(err, "failed to encode the cross-version-login config to json")
	}
	if err := ioutil.WriteFile(configPath, configJSON, 0644); err != nil {
		return errors.Wrapf(err, "failed to write file to %q", configPath)
	}
	return nil
}
