// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"io"
	"os"
	"path/filepath"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
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
