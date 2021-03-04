// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
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
		Attr:         []string{"group:mainline", "informational"},
	})
}

// CryptohomeTestAuth checks that cryptohome could test the user authorization correctly.
func CryptohomeTestAuth(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec helper: ", err)
	}
	cryptohome := helper.CryptohomeClient()
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	const (
		user        = util.FirstUsername
		password    = util.FirstPassword
		badPassword = util.SecondPassword
	)

	// Ensure that the user directory is unmounted and does not exist.
	if err := mountInfo.CleanUpMount(ctx, user); err != nil {
		s.Fatal("Failed to cleanup: ", err)
	}

	defer func() {
		// Ensure we remove the user account after the test.
		if err := mountInfo.CleanUpMount(ctx, user); err != nil {
			s.Fatal("Failed to cleanup: ", err)
		}
	}()

	// Mount the test user account, which ensures that the vault is created, and that the mount succeeds.
	if err := cryptohome.MountVault(ctx, user, password, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault: ", err)
	}

	// Test credentials when the user's directory is mounted.
	if _, err := cryptohome.CheckVault(ctx, user, password, util.PasswordLabel); err != nil {
		s.Fatal("Valid credentials should authenticate while mounted: ", err)
	}

	// Make sure that an incorrect password fails.
	if _, err := cryptohome.CheckVault(ctx, user, badPassword, util.PasswordLabel); err == nil {
		s.Fatal("Invalid credentials should not authenticate while mounted")
	}

	// Unmount the directory
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}

	// Ensure that the user directory is not mounted
	mounted, err := mountInfo.IsMounted(ctx, user)
	if err == nil && mounted {
		s.Fatal("Cryptohome did not unmount the user")
	}

	// Test valid credentials when the user's directory is not mounted
	if _, err := cryptohome.CheckVault(ctx, user, password, util.PasswordLabel); err != nil {
		s.Fatal("Valid credentials should authenticate while unmounted: ", err)
	}

	// Test invalid credentials fails while not mounted.
	if _, err := cryptohome.CheckVault(ctx, user, badPassword, util.PasswordLabel); err == nil {
		s.Fatal("Invalid credentials should not authenticate while unmounted")
	}

	// Re-mount existing test user vault, verifying that the mount succeeds.
	if err := cryptohome.MountVault(ctx, user, password, util.PasswordLabel, false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault: ", err)
	}
}
