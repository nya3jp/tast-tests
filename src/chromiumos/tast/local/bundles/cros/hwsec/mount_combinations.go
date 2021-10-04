// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/storage/files"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MountCombinations,
		Desc: "Verifies we are able to signin/mount 2+ users with different combinations of pin/password",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline"},
		Timeout:      5 * time.Minute,
	})
}

type combinations struct {
	password1 string
	label1    string
	password2 string
	label2    string
}

// cleanupVault is a helper that unmount and removes a vault.
func cleanupVault(ctx context.Context, s *testing.State, utility *hwsec.CryptohomeClient, username string) {
	if err := utility.UnmountAndRemoveVault(ctx, username); err != nil {
		s.Errorf("Failed to remove user vault for %q: %v", username, err)
	}
}

// setupHomedirFiles set up the HomedirFiles for the given user.
func setupHomedirFiles(ctx context.Context, utility *hwsec.CryptohomeClient, r hwsec.CmdRunner, username string) (*files.HomedirFiles, error) {
	hf, err := files.NewHomedirFiles(ctx, utility, r, username)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create HomedirFiles in home directory of %q", username)
	}
	if err := hf.Clear(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to clear HomedirFiles in home directory of %q", username)
	}
	if err := hf.Step(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to step HomedirFiles during initialization for %q", username)
	}
	return hf, nil
}

// MountCombinations tests that we are able to signin/mount 2+ users with different combinations of pin/password.
func MountCombinations(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()

	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	utility := helper.CryptohomeClient()
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, utility)

	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		// Ensure we remove the user account after the test.
		if err := mountInfo.CleanUpMount(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to cleanup first user: ", err)
		}
		if err := mountInfo.CleanUpMount(ctx, util.SecondUsername); err != nil {
			s.Error("Failed to cleanup second user: ", err)
		}
	}(ctxForCleanUp)

	// Ensure clean cryptohome.
	if err := mountInfo.CleanUpMount(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to cleanup first user: ", err)
	}
	if err := mountInfo.CleanUpMount(ctx, util.SecondUsername); err != nil {
		s.Fatal("Failed to cleanup second user: ", err)
	}

	// Take TPM ownership before running the test.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	// Create 2 users for testing.
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create first user vault: ", err)
	}
	defer cleanupVault(ctx, s, utility, util.FirstUsername)
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.SecondUsername, util.SecondPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create second user vault: ", err)
	}
	defer cleanupVault(ctx, s, utility, util.SecondUsername)

	// Create the corresponding test files in the 2 users.
	hf1, err := setupHomedirFiles(ctx, utility, cmdRunner, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to create HomedirFiles for user1: ", err)
	}
	hf2, err := setupHomedirFiles(ctx, utility, cmdRunner, util.SecondUsername)
	if err != nil {
		s.Fatal("Failed to create HomedirFiles for user2: ", err)
	}

	// Add pins to them.
	if err = utility.AddVaultKey(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, util.FirstPin, util.PinLabel, false /* not low entropy credential */); err != nil {
		s.Fatal("Failed to add keys: ", err)
	}
	if err = utility.AddVaultKey(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, util.SecondPin, util.PinLabel, false /* not low entropy credential */); err != nil {
		s.Fatal("Failed to add keys: ", err)
	}

	// Unmount before testing.
	if err = utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount user vaults: ", err)
	}

	TestCombinations := []combinations{
		{util.FirstPassword, util.PasswordLabel, util.SecondPassword, util.PasswordLabel},
		{util.FirstPin, util.PinLabel, util.SecondPassword, util.PasswordLabel},
		{util.FirstPassword, util.PasswordLabel, util.SecondPin, util.PinLabel},
		{util.FirstPin, util.PinLabel, util.SecondPin, util.PinLabel},
	}

	for _, c := range TestCombinations {
		// Mount the first user.
		if err := utility.MountVault(ctx, c.label1, hwsec.NewPassAuthConfig(util.FirstUsername, c.password1), false, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Failed to mount first user vault: ", err)
		}

		// Mount the second user.
		if err := utility.MountVault(ctx, c.label2, hwsec.NewPassAuthConfig(util.SecondUsername, c.password2), false, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Failed to mount second user vault: ", err)
		}

		// Check if we are mounted.
		if mounted, err := utility.IsMounted(ctx); err != nil {
			s.Fatal("Failed to check is mounted: ", err)
		} else if !mounted {
			s.Errorf("Vault not mounted for label %q on first user and label %q on second user", c.label1, c.label2)
		}

		// Check that files in both vaults are correct.
		if err := hf1.Verify(ctx); err != nil {
			s.Errorf("Verify failed for user1 with label %q on first user and label %q on second user", c.label1, c.label2)
		}
		if err := hf2.Verify(ctx); err != nil {
			s.Errorf("Verify failed for user2 with label %q on first user and label %q on second user", c.label1, c.label2)
		}
		if err := hf1.Step(ctx); err != nil {
			s.Errorf("Step failed for user1 with label %q on first user and label %q on second user", c.label1, c.label2)
		}
		if err := hf2.Step(ctx); err != nil {
			s.Errorf("Step failed for user2 with label %q on first user and label %q on second user", c.label1, c.label2)
		}

		// Now unmount.
		if err := utility.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmount user vaults: ", err)
		}
	}
}
