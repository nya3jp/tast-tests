// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	"chromiumos/tast/local/cryptohome"
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
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

type combinations struct {
	password1 string
	label1    string
	password2 string
	label2    string
}

// cleanupVault is a helper that unmount and removes a vault.
func cleanupVault(ctx context.Context, s *testing.State, utility *hwsec.UtilityCryptohomeBinary, username string) {
	if err := utility.UnmountAndRemoveVault(ctx, username); err != nil {
		s.Errorf("Failed to remove user vault for %q: %w", username, err)
	}
}

// MountCombinations tests that we are able to signin/mount 2+ users with different combinations of pin/password.
func MountCombinations(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}

	utility, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}

	helper, err := hwseclocal.NewHelper(utility)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	// Resets the TPM, system, and user states before running the tests.
	if err := hwseclocal.ResetTPMAndSystemStates(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err = cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	// Create 2 users for testing.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true); err != nil {
		s.Fatal("Failed to create first user vault: ", err)
	}
	defer cleanupVault(ctx, s, utility, util.FirstUsername)
	if err := utility.MountVault(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, true); err != nil {
		s.Fatal("Failed to create second user vault: ", err)
	}
	defer cleanupVault(ctx, s, utility, util.SecondUsername)

	// Add pins to them.
	if err := utility.AddVaultKey(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, util.FirstPin, util.PinLabel, true); err != nil {
		s.Fatal("Faiiled to add keys: ", err)
	}
	if err := utility.AddVaultKey(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, util.SecondPin, util.PinLabel, true); err != nil {
		s.Fatal("Faiiled to add keys: ", err)
	}

	// Unmount before testing.
	if err := utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to unmount user vaults: ", err)
	}

	TestCombinations := []combinations{
		combinations{util.FirstPassword, util.PasswordLabel, util.SecondPassword, util.PasswordLabel},
		combinations{util.FirstPin, util.PinLabel, util.SecondPassword, util.PasswordLabel},
		combinations{util.FirstPassword, util.PasswordLabel, util.SecondPin, util.PinLabel},
		combinations{util.FirstPin, util.PinLabel, util.SecondPin, util.PinLabel},
	}

	for _, c := range TestCombinations {
		// Mount the first user.
		if err := utility.MountVault(ctx, util.FirstUsername, c.password1, c.label1, false); err != nil {
			s.Fatal("Failed to mount first user vault: ", err)
		}

		// Mount the second user.
		if err := utility.MountVault(ctx, util.SecondUsername, c.password2, c.label2, false); err != nil {
			s.Fatal("Failed to mount second user vault: ", err)
		}

		// Check if we are mounted.
		if mounted, err := utility.IsMounted(ctx); err != nil {
			s.Fatal("Failed to check is mounted: ", err)
		} else if !mounted {
			s.Errorf("Vault not mounted for label %q on first user and label %q on second user", c.label1, c.label2)
		}

		// Now unmount.
		if err := utility.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to unmount user vaults: ", err)
		}
	}
}
