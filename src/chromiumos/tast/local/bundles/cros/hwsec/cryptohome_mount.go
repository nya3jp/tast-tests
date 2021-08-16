// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeMount,
		Desc: "Tests Cryptohome's ability to mount the user folder",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@google.com",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline"},
	})
}

// CryptohomeMount checks that cryptohome could mount the user folder correctly.
func CryptohomeMount(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	const (
		user         = util.FirstUsername
		goodPassword = util.FirstPassword
		badPassword  = util.SecondPassword
	)

	defer func(ctx context.Context) {
		// Ensure we remove the user account after the test.
		if err := mountInfo.CleanUpMount(ctx, user); err != nil {
			s.Error("Failed to cleanup: ", err)
		}
	}(ctx)

	// Ensure clean cryptohome.
	if err := mountInfo.CleanUpMount(ctx, user); err != nil {
		s.Fatal("Failed to cleanup: ", err)
	}

	// Create the account.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, goodPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user vault: ", err)
	}

	// Check the mount point information.
	if err := checkUserMountInfo(ctx, mountInfo, user, true); err != nil {
		s.Fatal("User mount point check failed after create account: ", err)
	}

	// Unmount the vault.
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}

	// Check the mount point information.
	if err := checkUserMountInfo(ctx, mountInfo, user, false); err != nil {
		s.Fatal("User mount point check failed after unmount: ", err)
	}

	// Mount with bad password should fail.
	err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, badPassword), false, hwsec.NewVaultConfig())
	var exitErr *hwsec.CmdExitError
	if !errors.As(err, &exitErr) {
		s.Fatalf("Unexpected mount error: got %q; want *hwsec.CmdExitError", err)
	}
	if exitErr.ExitCode != 3 {
		s.Fatalf("Unexpected mount exit code: got %d; want %d", exitErr.ExitCode, 3)
	}

	// Check the mount point information.
	if err := checkUserMountInfo(ctx, mountInfo, user, false); err != nil {
		s.Fatal("User mount point check failed after mount with bad password: ", err)
	}

	// Mount with good password should success.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, goodPassword), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount user vault: ", err)
	}

	// Check the mount point information.
	if err := checkUserMountInfo(ctx, mountInfo, user, true); err != nil {
		s.Error("User mount point check failed after mount with good password: ", err)
	}
}

// checkUserMountInfo checks the mount point information for user and checks the mount status with wantMounted.
func checkUserMountInfo(ctx context.Context, mountInfo *hwsec.CryptohomeMountInfo, user string, wantMounted bool) error {
	mounted, err := mountInfo.IsMounted(ctx, user)
	if err != nil {
		return errors.Wrap(err, "failed to get mount info")
	}
	if mounted != wantMounted {
		return errors.Errorf("unexpected mounted: got %t; want %t", mounted, wantMounted)
	}
	return nil
}
