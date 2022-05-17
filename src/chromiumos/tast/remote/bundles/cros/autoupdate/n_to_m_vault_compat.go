// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/autoupdate/util"
	"chromiumos/tast/testing"
)

type vaultType int64

type params struct {
	VaultType vaultType
}

const (
	noneVaultType vaultType = iota
	ecryptfsVaultType
	fscryptV1VaultType
	defaultVaultType
)

const (
	sleepTimeN2M = 10 * time.Second
	userName     = "foo@bar.baz"
	userPassword = "secret"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NToMVaultCompat,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify cross version vault's compatibility",
		Contacts: []string{
			"dlunev@google.com", // Test author
			"chromeos-storage@google.com",
		},
		Attr:         []string{"group:autoupdate"},
		SoftwareDeps: []string{"tpm", "reboot", "chrome", "auto_update_stable"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
		Timeout: util.TotalTestTime + 2*sleepTimeN2M,
		Params: []testing.Param{{
			Name: "default",
			Val:  &params{VaultType: defaultVaultType},
		}, {
			Name: "ecryptfs",
			Val:  &params{VaultType: ecryptfsVaultType},
		}, {
			Name:              "fscrypt_v1",
			ExtraSoftwareDeps: []string{"use_fscrypt_v2"},
			Val:               &params{VaultType: fscryptV1VaultType},
		}},
	})
}

func NToMVaultCompat(ctx context.Context, s *testing.State) {
	dut := s.DUT()

	env, err := util.NewHwsecEnv(dut)
	if err != nil {
		s.Fatal("Failed to create hwsec env: ", err)
	}

	vtype := s.Param().(*params).VaultType

	ops := &util.Operations{
		PreUpdate: func(ctx context.Context) error {
			return util.ClearTpm(ctx, env)
		},
		PostUpdate: func(ctx context.Context) error {
			return createVault(ctx, env, dut, vtype)
		},
		PostRollback: func(ctx context.Context) error {
			return verifyVault(ctx, env, dut, vtype)
		},
		CleanUp: func(ctx context.Context) {
			cleanupVault(ctx, env)
		},
	}

	if err := util.NToMTest(ctx, dut, s.OutDir(), s.RPCHint(), ops, 3 /*deltaM*/); err != nil {
		s.Fatal("Failed to run cross version test: ", err)
	}
}

func createVault(ctx context.Context, env *util.HwsecEnv, dut *dut.DUT, vaultType vaultType) error {
	testing.ContextLog(ctx, "Creating test vault")
	if err := prepareVault(ctx, dut, env.Utility, vaultType /*create=*/, true, userName, userPassword); err != nil {
		return errors.New("can't create vault")
	}
	defer env.Utility.UnmountAll(ctx)

	if _, err := env.CmdRunner.Run(ctx, "sh", "-c", fmt.Sprintf("echo -n %q > %q", util.TestFileContent, util.EncstatefulFile)); err != nil {
		return errors.Wrap(err, "failed to write encstatefule test content")
	}
	if err := hwsec.WriteUserTestContent(ctx, env.Utility, env.CmdRunner, userName, util.TestFile, util.TestFileContent); err != nil {
		return errors.Wrap(err, "failed to write user vault test content")
	}

	return nil
}

func verifyVault(ctx context.Context, env *util.HwsecEnv, dut *dut.DUT, vaultType vaultType) error {
	testing.ContextLog(ctx, "Verifying vault")
	if err := prepareVault(ctx, dut, env.Utility, vaultType /*create=*/, false, userName, userPassword); err != nil {
		return errors.New("can't mount vault")
	}
	defer env.Utility.UnmountAll(ctx)

	// Encstateful shouldn't be recreated.
	if content, err := env.CmdRunner.Run(ctx, "cat", util.EncstatefulFile); err != nil {
		return errors.Wrap(err, "failed to read encstateful test content")
	} else if !bytes.Equal(content, []byte(util.TestFileContent)) {
		return errors.Errorf("unexpected encstateful test file content: got %q, want %q", string(content), util.TestFileContent)
	}

	// User vault should already exist and shouldn't be recreated.
	if content, err := hwsec.ReadUserTestContent(ctx, env.Utility, env.CmdRunner, userName, util.TestFile); err != nil {
		return errors.Wrap(err, "failed to read user vault test content")
	} else if !bytes.Equal(content, []byte(util.TestFileContent)) {
		return errors.Errorf("unexpected user vault test file content: got %q, want %q", string(content), util.TestFileContent)
	}

	return nil
}

func cleanupVault(ctx context.Context, env *util.HwsecEnv) {
	env.Utility.UnmountAll(ctx)
	env.Utility.RemoveVault(ctx, userName)
}

func prepareVault(ctx context.Context, dut *dut.DUT, utility *hwsec.CryptohomeClient, vtype vaultType, create bool, username, password string) error {
	// None is a wrong type.
	if vtype == noneVaultType || vtype > defaultVaultType {
		return errors.Errorf("unsupported type: %v", vtype)
	}

	// To create V1, we need to negate the flag enabling v2.
	if vtype == fscryptV1VaultType && create {
		if err := dut.Conn().CommandContext(ctx, "initctl", "stop", "cryptohomed").Run(); err != nil {
			return errors.Wrap(err, "can't stop cryptohomed to change arguments")
		}
		testing.Sleep(ctx, sleepTimeN2M)
		if err := dut.Conn().CommandContext(ctx, "initctl", "start", "cryptohomed", "CRYPTOHOMED_ARGS=--negate_fscrypt_v2_for_test").Run(); err != nil {
			return errors.Wrap(err, "can't start cryptohomed with changed arguments")
		}
		testing.Sleep(ctx, sleepTimeN2M)
	}

	config := hwsec.NewVaultConfig()
	if vtype == ecryptfsVaultType {
		config.Ecryptfs = true
	}
	if err := utility.MountVault(ctx, "password", hwsec.NewPassAuthConfig(username, password), create, config); err != nil {
		return errors.Wrap(err, "failed to create user vault for testing")
	}
	return nil
}
