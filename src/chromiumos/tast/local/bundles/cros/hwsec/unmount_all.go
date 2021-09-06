// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Func: UnmountAll,
		Desc: "Verifies that cryptohome's Unmount() API works correctly by unmounting all user's home directory",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline"},
	})
}

func checkBothUnmounted(ctx context.Context, utility *hwsec.CryptohomeClient, cmdRunner hwsec.CmdRunner) error {
	// Check with IsMounted().
	if mounted, err := utility.IsMounted(ctx); err != nil {
		return errors.Wrap(err, "failed to check is mounted")
	} else if mounted {
		return errors.New("vault still mounted after unmount")
	}

	// Check with test files for first user.
	if exist, err := hwsec.DoesUserTestFileExist(ctx, utility, cmdRunner, util.FirstUsername, util.TestFileName1); err != nil {
		return errors.Wrap(err, "failed to check if first user's test file exist")
	} else if exist {
		return errors.New("first user's test file exists")
	}

	// Check with test files for second user.
	if exist, err := hwsec.DoesUserTestFileExist(ctx, utility, cmdRunner, util.SecondUsername, util.TestFileName1); err != nil {
		return errors.Wrap(err, "failed to check if second user's test file exist")
	} else if exist {
		return errors.New("second user's test file exists")
	}

	return nil
}

// UnmountAll tests that cryptohome's Unmount() correctly unmount all logged-in user's vault.
func UnmountAll(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	utility := helper.CryptohomeClient()

	// Take TPM Ownership before the test.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	// We check Unmount() works correctly whether the 2 vault is mounted during creation or subsequent remount.
	// Create 2 User/Vault -> Write Test File -> Unmount -> Remount -> Unmount
	// At each of the Unmount(), we check that it's correctly unmounted through IsMounted() and existence of test file.

	// Create 2 users for testing.
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create first user vault: ", err)
	}
	defer func() {
		// If we don't unmount, we can't remove the vault.
		// We don't care if it fails here, because we are not sure if the test completed.
		utility.UnmountAll(ctx)

		if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to remove first user vault: ", err)
		}
	}()
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.SecondUsername, util.SecondPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create second user vault: ", err)
	}
	defer func() {
		utility.UnmountAll(ctx)

		if _, err := utility.RemoveVault(ctx, util.SecondUsername); err != nil {
			s.Error("Failed to remove second user vault: ", err)
		}
	}()

	// Write test files to those 2 users' directory.
	if err := hwsec.WriteUserTestContent(ctx, utility, cmdRunner, util.FirstUsername, util.TestFileName1, util.TestFileContent); err != nil {
		s.Fatal("Failed to write first user's test content: ", err)
	}
	if err := hwsec.WriteUserTestContent(ctx, utility, cmdRunner, util.SecondUsername, util.TestFileName1, util.TestFileContent); err != nil {
		s.Fatal("Failed to write second user's test content: ", err)
	}

	// Now unmount.
	if err := utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount user vaults: ", err)
	}

	// Check if unmounted.
	if err := checkBothUnmounted(ctx, utility, cmdRunner); err != nil {
		s.Fatal("Still mounted: ", err)
	}

	// Now remount.
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount first user vault: ", err)
	}
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.SecondUsername, util.SecondPassword), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount second user vault: ", err)
	}

	// Both files should be there, right?
	if exist, err := hwsec.DoesUserTestFileExist(ctx, utility, cmdRunner, util.FirstUsername, util.TestFileName1); err != nil {
		s.Fatal("Failed to check if first user's test file exist: ", err)
	} else if !exist {
		s.Fatal("First user's test file disappeared")
	}
	if exist, err := hwsec.DoesUserTestFileExist(ctx, utility, cmdRunner, util.SecondUsername, util.TestFileName1); err != nil {
		s.Fatal("Failed to check if second user's test file exist: ", err)
	} else if !exist {
		s.Fatal("Second user's test file disappeared")
	}

	// Now unmount.
	if err := utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount user vaults: ", err)
	}

	// Check if unmounted.
	if err := checkBothUnmounted(ctx, utility, cmdRunner); err != nil {
		s.Fatal("Still mounted: ", err)
	}
}
