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
	"chromiumos/tast/remote/bundles/cros/autoupdate/autoupdatelib"
	hwsecremote "chromiumos/tast/remote/hwsec"
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
	sleepTimeN2M    = 10 * time.Second
	userName        = "foo@bar.baz"
	userPassword    = "secret"
	testFile        = "compat_testing_file"
	encstatefulFile = "/mnt/stateful_partition/encrypted/file"
	testFileContent = "content"
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
		SoftwareDeps: []string{"tpm", "reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
		Timeout: autoupdatelib.TotalTestTime + 2*sleepTimeN2M,
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

type hwsecEnv struct {
	cmdRunner *hwsecremote.CmdRunnerRemote
	helper    *hwsecremote.CmdHelperRemote
	utility   *hwsec.CryptohomeClient
}

func NToMVaultCompat(ctx context.Context, s *testing.State) {
	var err error
	env := &hwsecEnv{}
	env.cmdRunner = hwsecremote.NewCmdRunner(s.DUT())
	env.helper, err = hwsecremote.NewHelper(env.cmdRunner, s.DUT())
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	env.utility = env.helper.CryptohomeClient()

	ops := &autoupdatelib.Operations{
		PreUpdate: func(ctx context.Context, s *testing.State) {
			clearTpm(ctx, s, env)
		},
		PostUpdate: func(ctx context.Context, s *testing.State) {
			createVault(ctx, s, env)
		},
		PostRollback: func(ctx context.Context, s *testing.State) {
			verifyVault(ctx, s, env)
		},
		CleanUp: func(ctx context.Context, s *testing.State) {
			cleanupVault(ctx, s, env)
		},
	}

	autoupdatelib.NToMTest(ctx, s, ops, 3 /*deltaM*/)
}

func clearTpm(ctx context.Context, s *testing.State, env *hwsecEnv) {
	// Resets the TPM states before running the tests.
	if err := env.helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	if err := env.helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}
}

func createVault(ctx context.Context, s *testing.State, env *hwsecEnv) {
	s.Log("Creating test vault")
	vtype := s.Param().(*params).VaultType
	if err := prepareVault(ctx, s.DUT(), env.utility, vtype /*create=*/, true, userName, userPassword); err != nil {
		s.Fatal("Can't create vault: ", err)
	}
	defer env.utility.UnmountAll(ctx)

	if _, err := env.cmdRunner.Run(ctx, "sh", "-c", fmt.Sprintf("echo -n %q > %q", testFileContent, encstatefulFile)); err != nil {
		s.Fatal("Failed to write encstatefule test content: ", err)
	}
	if err := hwsec.WriteUserTestContent(ctx, env.utility, env.cmdRunner, userName, testFile, testFileContent); err != nil {
		s.Fatal("Failed to write user vault test content: ", err)
	}
}

func verifyVault(ctx context.Context, s *testing.State, env *hwsecEnv) {
	s.Log("Verifying vault")
	vtype := s.Param().(*params).VaultType
	if err := prepareVault(ctx, s.DUT(), env.utility, vtype /*create=*/, false, userName, userPassword); err != nil {
		s.Fatal("Can't mount vault: ", err)
	}
	defer env.utility.UnmountAll(ctx)

	// Encstateful shouldn't be recreated.
	if content, err := env.cmdRunner.Run(ctx, "cat", encstatefulFile); err != nil {
		s.Fatal("Failed to read encstateful test content: ", err)
	} else if !bytes.Equal(content, []byte(testFileContent)) {
		s.Fatalf("Unexpected encstateful test file content: got %q, want %q", string(content), testFileContent)
	}

	// User vault should already exist and shouldn't be recreated.
	if content, err := hwsec.ReadUserTestContent(ctx, env.utility, env.cmdRunner, userName, testFile); err != nil {
		s.Fatal("Failed to read user vault test content: ", err)
	} else if !bytes.Equal(content, []byte(testFileContent)) {
		s.Fatalf("Unexpected user vault test file content: got %q, want %q", string(content), testFileContent)
	}
}

func cleanupVault(ctx context.Context, s *testing.State, env *hwsecEnv) {
	env.utility.UnmountAll(ctx)
	env.utility.RemoveVault(ctx, userName)
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
