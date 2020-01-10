// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"reflect"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeKeysAndMountSanity,
		Desc: "Checks that the mount and keys related APIs works",
		Contacts: []string{
			"zuan@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"tpm"},
	})
}

const firstUsername = "PierreDeFermat@example.com"
const firstIncorrectPassword = "ImJustGuessing~"
const firstPrimaryPassword = "F131dTooSm@ll2C0nt@1nMyP@ssw0rd!!"
const firstChangedPrimaryPassword = "a^n+b^n=c^n" // Got a great proof, but margin too small.
const firstPrimaryLabel = "primary"
const firstChangedPrimaryLabel = "changed"
const firstSecondaryPassword = "54321"
const firstSecondaryLabel = "secondary"

// unmountTestVault is a helper function that unmount the test vault. It expect the test vault to be mounted when it is called. If any error occurs, it'll return an error, otherwise nil is returned.
func unmountTestVault(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	if _, err := utility.Unmount(ctx, firstUsername); err != nil {
		return errors.Wrap(err, "failed to unmount vault: ")
	}
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault still mounted after unmount: ")
	}
	return nil
}

// checkMountState is a helper function that returns an error if the result from IsMounted() is not equal to state, or if we've problem calling IsMounted().
func checkMountState(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary, state bool) error {
	mounted, err := utility.IsMounted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to call IsMounted()")
	}
	if mounted != state {
		return errors.Errorf("Incorrect IsMounted() state %t, expected %t", mounted, state)
	}
	return nil
}

func checkUserVault(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	// ----------------------- CheckKeyEx -----------------------
	// CheckKeyEx should work with the correct password.
	if result, _ := utility.CheckVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel); !result {
		return errors.New("failed to CheckKeyEx() with the correct username and password")
	}

	// CheckKeyEx should fail with the incorrect password.
	if result, _ := utility.CheckVault(ctx, firstUsername, firstIncorrectPassword, firstPrimaryLabel); result {
		return errors.New("incorrect password passed CheckKeyEx()")
	}

	// ----------------------- ListKeysEx -----------------------
	// ListKeysEx should show the keys that we've in stock
	labels, err := utility.ListVaultKeys(ctx, firstUsername)
	if err != nil {
		return errors.Wrap(err, "failed to list keys")
	}
	if !reflect.DeepEqual(labels, []string{firstPrimaryLabel}) {
		return errors.Errorf("mismatch result from list keys, expected %q, got %q", []string{firstPrimaryLabel}, labels)
	}

	// ----------------------- AddKeyEx/RemoveKeyEx -----------------------
	// AddKeyEx shouldn't work if password is incorrect.
	if err := utility.AddVaultKey(ctx, firstUsername, firstIncorrectPassword, firstPrimaryLabel, firstSecondaryPassword, firstSecondaryLabel, true); err == nil {
		return errors.New("add key succeeded when it shouldn't")
	}

	// AddKey should work if everything is correct.
	if err := utility.AddVaultKey(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel, firstSecondaryPassword, firstSecondaryLabel, true); err != nil {
		return errors.Wrap(err, "failed to add keys")
	}

	// We should be able to mount with the new key.
	// Nothing should be mounted at first.
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault mounted before testing AddKeyEx")
	}
	// Next try to mount the vault with the new key.
	if err := utility.MountVault(ctx, firstUsername, firstSecondaryPassword, firstSecondaryLabel, false); err != nil {
		return errors.Wrap(err, "failed to mount vault with the added key")
	}
	// Should be mounted now.
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with added key")
	}
	// CheckKeyEx should work with the new key.
	if result, _ := utility.CheckVault(ctx, firstUsername, firstSecondaryPassword, firstSecondaryLabel); !result {
		return errors.New("failed to CheckKeyEx() with the correct (secondary) username and password")
	}
	// Now unmount it.
	if err := unmountTestVault(ctx, utility); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}

	// RemoveKeyEx should work.
	if err := utility.RemoveVaultKey(ctx, firstUsername, firstPrimaryPassword, firstSecondaryLabel); err != nil {
		return errors.Wrap(err, "failed to remove key")
	}

	// After remove key, mounting should fail.
	if err := utility.MountVault(ctx, firstUsername, firstSecondaryPassword, firstSecondaryLabel, false); err == nil {
		return errors.New("still can mount vault with removed key")
	}
	// Nothing should be mounted now since it's unmounted and the previous mount failed.
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault mounted after unmounting and failed mount")
	}

	// ----------------------- MigrateKeyEx -----------------------
	// MigrateKeyEx should work.
	if err := utility.ChangeVaultPassword(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel, firstChangedPrimaryPassword); err != nil {
		return errors.Wrap(err, "failed to change vault password")
	}
	// After changing the password, CheckKeyEx should work the new password but not the old.
	if result, _ := utility.CheckVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel); result {
		return errors.New("the old password still works")
	}
	if result, _ := utility.CheckVault(ctx, firstUsername, firstChangedPrimaryPassword, firstPrimaryLabel); !result {
		return errors.New("the new password doesn't work")
	}

	// Mounting with old password shouldn't work.
	if err := utility.MountVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel, false); err == nil {
		return errors.New("still can mount vault with old password")
	}
	// Mounting with new password should work.
	if err := utility.MountVault(ctx, firstUsername, firstChangedPrimaryPassword, firstPrimaryLabel, false); err != nil {
		return errors.Wrap(err, "failed to mount with new password")
	}
	// Should be mounted now.
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with changed password")
	}
	// MigrateKeyEx should work when vault is mounted.
	if err := utility.ChangeVaultPassword(ctx, firstUsername, firstChangedPrimaryPassword, firstPrimaryLabel, firstPrimaryPassword); err != nil {
		return errors.Wrap(err, "failed to change vault password back when mounted")
	}
	// Should still be mounted after changing password.
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after changing password while mounted")
	}

	// Testing with CheckKeyEx should be effective immediately without a remount.
	if result, _ := utility.CheckVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel); !result {
		return errors.New("the new password doesn't work after the password is changed back")
	}
	if result, _ := utility.CheckVault(ctx, firstUsername, firstChangedPrimaryPassword, firstPrimaryLabel); result {
		return errors.New("the old password still works after the password is changed back")
	}

	// Now unmount it.
	if err := unmountTestVault(ctx, utility); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	// Should be unmounted after unmount.
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault mounted after unmounting while testing MigrateKeyEx")
	}

	// Same stuff after unmount.
	if result, _ := utility.CheckVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel); !result {
		return errors.New("the new password doesn't work after the password is changed back")
	}
	if result, _ := utility.CheckVault(ctx, firstUsername, firstChangedPrimaryPassword, firstPrimaryLabel); result {
		return errors.New("the old password still works after the password is changed back")
	}

	// ----------------------- UpdateKeyEx -----------------------
	// We should be able to change key label with UpdateKeyEx
	if err := utility.ChangeVaultLabel(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel, firstChangedPrimaryLabel); err != nil {
		return errors.New("failed to change the vault password")
	}
	// After changing the label, CheckKeyEx should work the new label but not the old.
	if result, _ := utility.CheckVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel); result {
		return errors.New("the old label still works")
	}
	if result, _ := utility.CheckVault(ctx, firstUsername, firstPrimaryPassword, firstChangedPrimaryLabel); !result {
		return errors.New("the new label doesn't work")
	}

	// Mounting with old label shouldn't work.
	if err := utility.MountVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel, false); err == nil {
		return errors.New("still can mount vault with old label")
	}
	// Mounting with new label should work.
	if err := utility.MountVault(ctx, firstUsername, firstPrimaryPassword, firstChangedPrimaryLabel, false); err != nil {
		return errors.Wrap(err, "failed to mount with new label")
	}
	// Should be mounted now.
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with changed label")
	}
	// UpdateKeyEx should work when vault is mounted.
	if err := utility.ChangeVaultLabel(ctx, firstUsername, firstPrimaryPassword, firstChangedPrimaryLabel, firstPrimaryLabel); err != nil {
		return errors.Wrap(err, "failed to change vault label back when mounted")
	}
	// Should still be mounted after changing password.
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after changing label while mounted")
	}
	// Testing with CheckKeyEx should be effective immediately without a remount.
	if result, _ := utility.CheckVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel); !result {
		return errors.New("the new label doesn't work after changing label back while mounted")
	}
	// Note: In the current implementation, the old label still works after changing the label while mounted. We do not test this here.

	// Now unmount it.
	if err := unmountTestVault(ctx, utility); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	// Should be unmounted after unmount.
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault mounted after unmounting while testing UpdateKeyEx")
	}

	// Same stuff after unmount.
	if result, _ := utility.CheckVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel); !result {
		return errors.New("the new label doesn't work after the label is changed back")
	}
	if result, _ := utility.CheckVault(ctx, firstUsername, firstPrimaryPassword, firstChangedPrimaryLabel); result {
		return errors.New("the old label still works after the label is changed back")
	}
	return nil
}

func CryptohomeKeysAndMountSanity(ctx context.Context, s *testing.State) {
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

	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	// Create the user and check it is correctly mounted and can be unmounted.
	func() {
		if err := utility.MountVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel, true); err != nil {
			s.Fatal("Failed to create user: ", err)
		}
		// Unmount within this closure, because we want to have the thing unmounted for checkUserVault() to work.
		defer func() {
			if err := unmountTestVault(ctx, utility); err != nil {
				s.Fatal("Failed to unmount: ", err)
			}
		}()

		if err := checkMountState(ctx, utility, true); err != nil {
			s.Fatal("Vault is not mounted: ", err)
		}
	}()

	// Cleanup the created vault.
	defer func() {
		// Remove the vault.
		if _, err := utility.RemoveVault(ctx, firstUsername); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
		// Double check that it's not mountable.
		if err := utility.MountVault(ctx, firstUsername, firstPrimaryPassword, firstPrimaryLabel, false); err == nil {
			s.Fatal("Still can mount vault after removing vault: ")
		}
		// Since vault is removed and the mount failed, nothing should be mounted now.
		if err := checkMountState(ctx, utility, false); err != nil {
			s.Fatal(err, "Vault mounted after removing vault: ")
		}
	}()

	// Check that everything is alright.
	if err := checkUserVault(ctx, utility); err != nil {
		s.Fatal("Check user failed: ", err)
	}

	// Now take ownership.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Time out waiting for TPM to be ready: ", err)
	}

	// Check that everything is alright again.
	if err := checkUserVault(ctx, utility); err != nil {
		s.Fatal("Check user failed: ", err)
	}

	// Reboot to check that everything is alright after reboot.
	if err := helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}

	// Check that everything is alright yet another time.
	if err := checkUserVault(ctx, utility); err != nil {
		s.Fatal("Check user failed: ", err)
	}
}
