// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto/rsa"
	"os"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// CrossVersionLoginKeyLabel is the key label used in cross-version login tests.
	CrossVersionLoginKeyLabel string = "legacy-0"
)

// CrossVersionLoginConfig contains the information for cross-version login
type CrossVersionLoginConfig struct {
	AuthConfig hwsec.AuthConfig
	RsaKey     *rsa.PrivateKey
}

// NewPassAuthCrossVersionLoginConfig creates cross version-login config from password auth config
func NewPassAuthCrossVersionLoginConfig(authConfig *hwsec.AuthConfig) *CrossVersionLoginConfig {
	config := &CrossVersionLoginConfig{}
	config.AuthConfig = *authConfig
	return config
}

// NewChallengeAuthCrossVersionLoginConfig creates cross-version login config from challenge auth config and rsa key
func NewChallengeAuthCrossVersionLoginConfig(authConfig *hwsec.AuthConfig, rsaKey *rsa.PrivateKey) *CrossVersionLoginConfig {
	config := &CrossVersionLoginConfig{}
	config.AuthConfig = *authConfig
	config.RsaKey = rsaKey
	return config
}

// CreateCrossVersionLoginData creates the compressed file of data that is used in cross-version login test.
func CreateCrossVersionLoginData(ctx context.Context, daemonController *hwsec.DaemonController, filePath string) error {
	if err := stopHwsecDaemons(ctx, daemonController); err != nil {
		return err
	}
	defer ensureHwsecDaemons(ctx, daemonController)

	args := []string{
		"--exclude=/home/.shadow/*/mount",
		"-Jcvf",
		filePath,
		"/mnt/stateful_partition/unencrypted/tpm2-simulator/NVChip",
		"/home/.shadow/",
	}
	if err := testexec.CommandContext(ctx, "tar", args...).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to compress the cryptohome data")
	}
	return nil
}

// LoadCrossVersionLoginData loads the data that is used in cross-version login test.
func LoadCrossVersionLoginData(ctx context.Context, daemonController *hwsec.DaemonController, filePath string) error {
	if err := stopHwsecDaemons(ctx, daemonController); err != nil {
		return err
	}
	defer ensureHwsecDaemons(ctx, daemonController)

	// Remove the `/home/.shadow` first to prevent any unexpected file remaining.
	if err := os.RemoveAll("/home/.shadow"); err != nil {
		return errors.Wrap(err, "failed to remove old data")
	}

	if err := testexec.CommandContext(ctx, "tar", "Jxvf", filePath, "-C", "/").Run(); err != nil {
		return errors.Wrap(err, "failed to decompress the cryptohome data")
	}
	return nil
}

func stopHwsecDaemons(ctx context.Context, daemonController *hwsec.DaemonController) error {
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
}
