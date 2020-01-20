// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LockToSingleUserMountUntilReboot,
		Desc: "Checks that LockToSingleUserMountUntilReboot method works",
		Contacts: []string{
			"zuan@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"tpm"},
	})
}

// create2VaultsForTesting will create 2 vaults for testing.
func create2VaultsForTesting(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	// Create 2 vault for testing.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true); err != nil {
		return errors.Wrap(err, "failed to create first user")
	}
	if err := utility.MountVault(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, true); err != nil {
		return errors.Wrap(err, "failed to create second user")
	}

	// Unmount the vault before further testing.
	if err := utility.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount vault before testing")
	}

	// Add a second vault key to simulate other login methods such as pin.
	if err := utility.AddVaultKey(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, util.FirstPin, util.PinLabel, false); err != nil {
		return errors.Wrap(err, "failed to add key to vault for first user")
	}
	if err := utility.AddVaultKey(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, util.SecondPin, util.PinLabel, false); err != nil {
		return errors.Wrap(err, "failed to add key to vault for second user")
	}

	return nil
}

// cleanupVault will delete the first and second user's vault.
func cleanupVault(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	// Remove the vault.
	if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
		return errors.Wrap(err, "failed to remove first user's vault")
	}
	if _, err := utility.RemoveVault(ctx, util.SecondUsername); err != nil {
		return errors.Wrap(err, "failed to remove second user's vault")
	}
	return nil
}

// checkVaultWorks will check that the vault specified by username, password and pin works in both mounting and unlock (CheckKeyEx).
func checkVaultWorks(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary, username, password, pin string) error {
	if err := utility.MountVault(ctx, username, password, util.PasswordLabel, false); err != nil {
		return errors.Wrap(err, "failed to mount with password")
	}
	if err := utility.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount vault")
	}
	if err := utility.MountVault(ctx, username, pin, util.PinLabel, false); err != nil {
		return errors.Wrap(err, "failed to mount with password")
	}
	if err := utility.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount vault")
	}

	if result, _ := utility.CheckVault(ctx, username, password, util.PasswordLabel); !result {
		return errors.New("failed to check key with password")
	}

	if result, _ := utility.CheckVault(ctx, username, pin, util.PinLabel); !result {
		return errors.New("failed to check key with pin")
	}

	return nil
}

// checkBothVaultIsOperational will check that both first user's vault and second user's vault is operational.
func checkBothVaultIsOperational(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	// Now test that logging in on both user works as expected.
	if err := checkVaultWorks(ctx, utility, util.FirstUsername, util.FirstPassword, util.FirstPin); err != nil {
		return errors.Wrap(err, "first user's vault doesn't work")
	}
	if err := checkVaultWorks(ctx, utility, util.SecondUsername, util.SecondPassword, util.SecondPin); err != nil {
		return errors.Wrap(err, "second user's vault doesn't work")
	}

	return nil
}

// checkOthersAreBlocked will check that users other than util.FirstUsername is blocked from mounting and CheckKeyEx (unlock).
func checkOthersAreBlocked(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	// Mount should fail.
	if err := utility.MountVault(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, false); err == nil {
		return errors.Wrap(err, "second user is mountable with password after locking to single user")
	}
	if err := utility.MountVault(ctx, util.SecondUsername, util.SecondPin, util.PinLabel, false); err == nil {
		return errors.Wrap(err, "second user is mountable with pin after locking to single user")
	}

	// CheckKeyEx should fail too.
	if result, _ := utility.CheckVault(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel); result {
		return errors.New("check key succeeded after locking to single user")
	}
	if result, _ := utility.CheckVault(ctx, util.SecondUsername, util.SecondPin, util.PinLabel); result {
		return errors.New("check key succeeded with pin after locking to single user")
	}

	// Shouldn't be able to create new users.
	if err := utility.MountVault(ctx, util.ThirdUsername, util.ThirdPassword, util.PasswordLabel, true); err == nil {
		return errors.Wrap(err, "create user succeeded after locking to single user")
	}
	return nil
}

func LockToSingleUserMountUntilReboot(ctx context.Context, s *testing.State) {
	// Standard initializations.
	r, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("CmdRunner creation error: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(r)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	helper, err := hwsecremote.NewHelper(utility, r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	// Create the vaults for testing.
	if err := create2VaultsForTesting(ctx, utility); err != nil {
		s.Fatal("Failed to initialize vaults for testing: ", err)
	}
	defer func() {
		if err := cleanupVault(ctx, utility); err != nil {
			s.Error("Failed to cleanup vault: ", err)
		}
	}()

	// Before starting the actual test, check that everything is alright.
	if err := checkBothVaultIsOperational(ctx, utility); err != nil {
		s.Fatal("Initially created vaults doesn't work: ", err)
	}

	// Now mount the first user and lock to single user mount.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPin, util.PinLabel, false); err != nil {
		s.Fatal("Failed to mount the user for lock to single user mount")
	}
	if err := utility.LockToSingleUserMountUntilReboot(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to lock to single user mount")
	}

	// Check that the other user is blocked.
	if err := checkOthersAreBlocked(ctx, utility); err != nil {
		s.Fatal("Other users are not blocked")
	}

	// Unmount the first user's vault.
	if _, err := utility.Unmount(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to unmount first user's vault after locking")
	}

	// The first user's vault should still works (CheckKeyEx and MountEx).
	if err := checkVaultWorks(ctx, utility, util.FirstUsername, util.FirstPassword, util.FirstPin); err != nil {
		s.Fatal("The first user's vault doesn't work after locking")
	}

	// Now reboot and check that the effects wear off.
	if err := helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}

	if err := checkBothVaultIsOperational(ctx, utility); err != nil {
		s.Fatal("Vaults doesn't work after reboot: ", err)
	}
}
