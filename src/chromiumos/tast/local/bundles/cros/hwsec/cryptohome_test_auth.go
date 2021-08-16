// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeTestAuth,
		Desc: "Tests Cryptohome's ability to test the user authorization",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@google.com",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline"},
	})
}

// CryptohomeTestAuth checks that cryptohome could test the user authorization correctly.
func CryptohomeTestAuth(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	const (
		user              = util.FirstUsername
		password          = util.FirstPassword
		badPassword       = util.SecondPassword
		mountFailExitCode = 3
	)

	// Ensure that the user directory is unmounted and does not exist.
	if err := mountInfo.CleanUpMount(ctx, user); err != nil {
		s.Fatal("Failed to cleanup: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		// Ensure we remove the user account after the test.
		if err := mountInfo.CleanUpMount(ctx, user); err != nil {
			s.Fatal("Failed to cleanup: ", err)
		}
	}(cleanupCtx)

	// Mount the test user account, which ensures that the vault is created, and that the mount succeeds.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, password), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault: ", err)
	}

	// Test credentials when the user's directory is mounted.
	if _, err := cryptohome.CheckVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, password)); err != nil {
		s.Fatal("Should access the vault with the valid credentials while mounted: ", err)
	}

	// Make sure that an incorrect password fails.
	_, err := cryptohome.CheckVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, badPassword))
	var exitErr *hwsec.CmdExitError
	if !errors.As(err, &exitErr) {
		s.Fatal("Should deny access the vault with the invalid credentials while mounted: ", err)
	}
	if exitErr.ExitCode != mountFailExitCode {
		s.Fatalf("Unexpected mount exit code: got %d; want %d", exitErr.ExitCode, mountFailExitCode)
	}

	// Unmount the directory
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}

	// Ensure that the user directory is not mounted
	mounted, err := mountInfo.IsMounted(ctx, user)
	if err != nil {
		s.Fatal("Failed to get mount info: ", err)
	}
	if mounted {
		s.Fatal("Cryptohome did not unmount the user")
	}

	// Test valid credentials when the user's directory is not mounted
	if _, err := cryptohome.CheckVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, password)); err != nil {
		s.Fatal("Should access the vault with the valid credentials while unmounted: ", err)
	}

	// Test invalid credentials fails while not mounted.
	_, err = cryptohome.CheckVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, badPassword))
	if !errors.As(err, &exitErr) {
		s.Fatal("Should deny access the vault with the invalid credentials while unmounted: ", err)
	}
	if exitErr.ExitCode != mountFailExitCode {
		s.Fatalf("Unexpected mount exit code: got %d; want %d", exitErr.ExitCode, mountFailExitCode)
	}

	// Re-mount existing test user vault, verifying that the mount succeeds.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, password), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault: ", err)
	}
}
