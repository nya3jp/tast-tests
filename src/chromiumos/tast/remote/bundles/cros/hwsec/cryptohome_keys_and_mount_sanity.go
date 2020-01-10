// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"reflect"
	"sort"

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

// unmountTestVault is a helper function that unmount the test vault. It expect the test vault to be mounted when it is called. If any error occurs, it'll return an error, otherwise nil is returned.
func unmountTestVault(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	if _, err := utility.Unmount(ctx, hwsec.FirstUsername); err != nil {
		return errors.Wrap(err, "failed to unmount vault: ")
	}
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault still mounted after unmount: ")
	}
	return nil
}

// checkMountState is a helper function that returns an error if the result from IsMounted() is not equal to expected, or if we've problem calling IsMounted().
func checkMountState(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary, expected bool) error {
	actuallyMounted, err := utility.IsMounted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to call IsMounted()")
	}
	if actuallyMounted != expected {
		return errors.Errorf("incorrect IsMounted() state %t, expected %t", actuallyMounted, expected)
	}
	return nil
}

func checkKeysLabels(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary, target []string) error {
	labels, err := utility.ListVaultKeys(ctx, hwsec.FirstUsername)
	if err != nil {
		return errors.Wrap(err, "failed to list keys")
	}
	// Sort the 2 slices so that we can compare them.
	sort.Strings(labels)
	sortedTarget := make([]string, len(target))
	copy(sortedTarget, target)
	sort.Strings(sortedTarget)

	if !reflect.DeepEqual(labels, sortedTarget) {
		return errors.Errorf("mismatch result from list keys, expected %q, got %q", sortedTarget, labels)
	}

	return nil
}

// testCheckKeyEx test that CheckKeyEx() works as expected.
func testCheckKeyEx(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary, username, label, correctPassword, incorrectPassword string) error {
	// CheckKeyEx should work with the correct password.
	if result, _ := utility.CheckVault(ctx, username, correctPassword, label); !result {
		return errors.New("failed to CheckKeyEx() with the correct username and password")
	}

	// CheckKeyEx should fail with the incorrect password.
	if result, _ := utility.CheckVault(ctx, username, incorrectPassword, label); result {
		return errors.New("incorrect password passed CheckKeyEx()")
	}

	return nil
}

// testListKeysEx test that ListKeysEx() works as expected.
func testListKeysEx(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	// ListKeysEx should show the keys that we've in stock.
	if err := checkKeysLabels(ctx, utility, []string{hwsec.PasswordLabel}); err != nil {
		return errors.Wrap(err, "list of keys is incorrect at first")
	}

	return nil
}

// testAddRemoveKeyEx test that AddKeyEx() and RemoveKeyEx() works as expected.
func testAddRemoveKeyEx(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	// AddKeyEx shouldn't work if password is incorrect.
	if err := utility.AddVaultKey(ctx, hwsec.FirstUsername, hwsec.IncorrectPassword, hwsec.PasswordLabel, hwsec.FirstPin, hwsec.PinLabel, true); err == nil {
		return errors.New("add key succeeded when it shouldn't")
	}

	// AddKey should work if everything is correct.
	if err := utility.AddVaultKey(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PasswordLabel, hwsec.FirstPin, hwsec.PinLabel, true); err != nil {
		return errors.Wrap(err, "failed to add keys")
	}

	// After adding the key, we should be able to see it.
	if err := checkKeysLabels(ctx, utility, []string{hwsec.PasswordLabel, hwsec.PinLabel}); err != nil {
		return errors.Wrap(err, "list of keys is incorrect after adding key")
	}

	// We should be able to mount with the new key.
	// Nothing should be mounted at first.
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault mounted before testing AddKeyEx")
	}
	// Next try to mount the vault with the new key.
	if err := utility.MountVault(ctx, hwsec.FirstUsername, hwsec.FirstPin, hwsec.PinLabel, false); err != nil {
		return errors.Wrap(err, "failed to mount vault with the added key")
	}
	// Should be mounted now.
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with added key")
	}
	// CheckKeyEx should work with the new key.
	if result, _ := utility.CheckVault(ctx, hwsec.FirstUsername, hwsec.FirstPin, hwsec.PinLabel); !result {
		return errors.New("failed to CheckKeyEx() with the correct (secondary) username and password")
	}
	// Now unmount it.
	if err := unmountTestVault(ctx, utility); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}

	// RemoveKeyEx should work.
	if err := utility.RemoveVaultKey(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PinLabel); err != nil {
		return errors.Wrap(err, "failed to remove key")
	}

	// After remove key, mounting should fail.
	if err := utility.MountVault(ctx, hwsec.FirstUsername, hwsec.FirstPin, hwsec.PinLabel, false); err == nil {
		return errors.New("still can mount vault with removed key")
	}
	// Nothing should be mounted now since it's unmounted and the previous mount failed.
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault mounted after unmounting and failed mount")
	}

	return nil
}

// testMigrateKeyEx test that MigrateKeyEx() works correctly. Note: MigrateKeyEx() changes the vault password.
func testMigrateKeyEx(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	// MigrateKeyEx should work.
	if err := utility.ChangeVaultPassword(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PasswordLabel, hwsec.FirstChangedPassword); err != nil {
		return errors.Wrap(err, "failed to change vault password")
	}
	// After changing the password, CheckKeyEx should work the new password but not the old.
	if err := testCheckKeyEx(ctx, utility, hwsec.FirstUsername, hwsec.PasswordLabel, hwsec.FirstChangedPassword, hwsec.FirstPassword); err != nil {
		return errors.Wrap(err, "incorrect CheckKeyEx behaviour right after MigrateKeyEx")
	}

	// Mounting with old password shouldn't work.
	if err := utility.MountVault(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PasswordLabel, false); err == nil {
		return errors.New("still can mount vault with old password")
	}
	// Mounting with new password should work.
	if err := utility.MountVault(ctx, hwsec.FirstUsername, hwsec.FirstChangedPassword, hwsec.PasswordLabel, false); err != nil {
		return errors.Wrap(err, "failed to mount with new password")
	}
	// Should be mounted now.
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with changed password")
	}
	// MigrateKeyEx should work when vault is mounted.
	if err := utility.ChangeVaultPassword(ctx, hwsec.FirstUsername, hwsec.FirstChangedPassword, hwsec.PasswordLabel, hwsec.FirstPassword); err != nil {
		return errors.Wrap(err, "failed to change vault password back when mounted")
	}
	// Should still be mounted after changing password.
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after changing password while mounted")
	}

	// Testing with CheckKeyEx should be effective immediately without a remount.
	if err := testCheckKeyEx(ctx, utility, hwsec.FirstUsername, hwsec.PasswordLabel, hwsec.FirstPassword, hwsec.FirstChangedPassword); err != nil {
		return errors.Wrap(err, "incorrect CheckKeyEx behaviour right after password is changed back")
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
	if err := testCheckKeyEx(ctx, utility, hwsec.FirstUsername, hwsec.PasswordLabel, hwsec.FirstPassword, hwsec.FirstChangedPassword); err != nil {
		return errors.Wrap(err, "incorrect CheckKeyEx behaviour after the password is changed back")
	}

	return nil
}

// testUpdateKeyEx checks that UpdateKeyEx works correctly.
func testUpdateKeyEx(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	// We should be able to change key label with UpdateKeyEx.
	if err := utility.ChangeVaultLabel(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PasswordLabel, hwsec.ChangedPasswordLabel); err != nil {
		return errors.New("failed to change the vault password")
	}
	// After changing the label, CheckKeyEx should work the new label but not the old.
	if result, _ := utility.CheckVault(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PasswordLabel); result {
		return errors.New("the old label still works")
	}
	if result, _ := utility.CheckVault(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.ChangedPasswordLabel); !result {
		return errors.New("the new label doesn't work")
	}

	// We should be able to see the changed label.
	if err := checkKeysLabels(ctx, utility, []string{hwsec.ChangedPasswordLabel}); err != nil {
		return errors.Wrap(err, "list of keys is incorrect after UpdateKeyEx")
	}

	// Mounting with old label shouldn't work.
	if err := utility.MountVault(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PasswordLabel, false); err == nil {
		return errors.New("still can mount vault with old label")
	}
	// Mounting with new label should work.
	if err := utility.MountVault(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.ChangedPasswordLabel, false); err != nil {
		return errors.Wrap(err, "failed to mount with new label")
	}
	// Should be mounted now.
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with changed label")
	}
	// UpdateKeyEx should work when vault is mounted.
	if err := utility.ChangeVaultLabel(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.ChangedPasswordLabel, hwsec.PasswordLabel); err != nil {
		return errors.Wrap(err, "failed to change vault label back when mounted")
	}
	// Should still be mounted after changing password.
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after changing label while mounted")
	}
	// Testing with CheckKeyEx should be effective immediately without a remount.
	if result, _ := utility.CheckVault(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PasswordLabel); !result {
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
	if result, _ := utility.CheckVault(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PasswordLabel); !result {
		return errors.New("the new label doesn't work after the label is changed back")
	}
	if result, _ := utility.CheckVault(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.ChangedPasswordLabel); result {
		return errors.New("the old label still works after the label is changed back")
	}
	return nil
}

// checkUserVault checks that the vault/keys related API works correctly.
func checkUserVault(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary) error {
	if err := testCheckKeyEx(ctx, utility, hwsec.FirstUsername, hwsec.PasswordLabel, hwsec.FirstPassword, hwsec.IncorrectPassword); err != nil {
		return errors.Wrap(err, "test on CheckKeyEx failed")
	}

	if err := testListKeysEx(ctx, utility); err != nil {
		return errors.Wrap(err, "test on ListKeysEx failed")
	}

	if err := testAddRemoveKeyEx(ctx, utility); err != nil {
		return errors.Wrap(err, "test on AddKeyEx/RemoveKeyEx failed")
	}

	if err := testMigrateKeyEx(ctx, utility); err != nil {
		return errors.Wrap(err, "test on MigrateKeyEx failed")
	}

	if err := testUpdateKeyEx(ctx, utility); err != nil {
		return errors.Wrap(err, "test on UpdateKeyEx failed")
	}

	return nil
}

// CryptohomeKeysAndMountSanity exercizes and tests the correctness of cryptohome's key and vault related APIs when the DUT goes through various states (ownership not taken, ownership taken, after reboot).
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
		if err := utility.MountVault(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PasswordLabel, true); err != nil {
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
		if _, err := utility.RemoveVault(ctx, hwsec.FirstUsername); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
		// Double check that it's not mountable.
		if err := utility.MountVault(ctx, hwsec.FirstUsername, hwsec.FirstPassword, hwsec.PasswordLabel, false); err == nil {
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
