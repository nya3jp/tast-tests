// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"reflect"
	"sort"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeKeysAndMountValidity,
		Desc: "Checks that the mount and keys related APIs works",
		Contacts: []string{
			"zuan@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"group:hwsec_destructive_func"},
		SoftwareDeps: []string{"tpm", "reboot"},
		// Skip "enguarde" due to the reboot issue when removing the key. Please see b/151057300.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("enguarde")),
		Timeout:      15 * time.Minute,
	})
}

// unmountTestVault is a helper function that unmount the test vault. It expects
// the test vault to be mounted when it is called.
func unmountTestVault(ctx context.Context, utility *hwsec.CryptohomeClient) error {
	if _, err := utility.Unmount(ctx, util.FirstUsername); err != nil {
		return errors.Wrap(err, "failed to unmount vault")
	}
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault still mounted after unmount")
	}
	return nil
}

// checkMountState is a helper function that returns an error if the result
// from IsMounted() is not equal to expected, or if we've problem calling
// IsMounted().
func checkMountState(ctx context.Context, utility *hwsec.CryptohomeClient, expected bool) error {
	actuallyMounted, err := utility.IsMounted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to call IsMounted()")
	}
	if actuallyMounted != expected {
		return errors.Errorf("incorrect IsMounted() state %t, expected %t", actuallyMounted, expected)
	}
	return nil
}

// checkKeysLabels checks if the list of key labels matches target and returns
// an error if it doesn't.
func checkKeysLabels(ctx context.Context, utility *hwsec.CryptohomeClient, target []string) error {
	labels, err := utility.ListVaultKeys(ctx, util.FirstUsername)
	if err != nil {
		return errors.Wrap(err, "failed to list keys")
	}
	// Sort and compare the actual and expected keys.
	sort.Strings(labels)
	sortedTarget := make([]string, len(target))
	copy(sortedTarget, target)
	sort.Strings(sortedTarget)

	if !reflect.DeepEqual(labels, sortedTarget) {
		return errors.Errorf("mismatch result from list keys, got %q, expected %q", labels, sortedTarget)
	}

	return nil
}

// testCheckKeyEx tests that CheckKeyEx() works as expected.
func testCheckKeyEx(ctx context.Context, utility *hwsec.CryptohomeClient, username, label, correctPassword, incorrectPassword string) error {
	// CheckKeyEx should work with the correct password.
	result, err := utility.CheckVault(ctx, label, hwsec.NewPassAuthConfig(username, correctPassword))
	if err != nil {
		return errors.Wrap(err, "call to CheckKeyEx with the correct username and password resulted in an error")
	}
	if !result {
		return errors.New("failed to CheckKeyEx() with the correct username and password")
	}

	// CheckKeyEx should fail with the incorrect password.
	result, err = utility.CheckVault(ctx, label, hwsec.NewPassAuthConfig(username, incorrectPassword))
	if err == nil {
		return errors.New("call to CheckKeyEx() is successful with incorrect password ")
	}
	if result {
		return errors.New("incorrect password passed CheckKeyEx()")
	}

	return nil
}

// testListKeysEx tests that ListKeysEx() in cryptohome's API works as expected.
func testListKeysEx(ctx context.Context, utility *hwsec.CryptohomeClient) error {
	// ListKeysEx should show the keys that are in stock.
	if err := checkKeysLabels(ctx, utility, []string{util.PasswordLabel}); err != nil {
		return errors.Wrap(err, "list of keys is incorrect")
	}

	return nil
}

// testAddRemoveKeyEx tests that AddKeyEx() and RemoveKeyEx() work as expected.
func testAddRemoveKeyEx(ctx context.Context, utility *hwsec.CryptohomeClient) error {
	// AddKeyEx shouldn't work if password is incorrect.
	if err := utility.AddVaultKey(ctx, util.FirstUsername, util.IncorrectPassword, util.PasswordLabel, util.FirstPin, util.PinLabel, false); err == nil {
		return errors.New("add key succeeded when it shouldn't")
	}

	// AddKey should work if everything is correct.
	if err := utility.AddVaultKey(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, util.FirstPin, util.PinLabel, false); err != nil {
		return errors.Wrap(err, "failed to add keys")
	}
	if err := checkKeysLabels(ctx, utility, []string{util.PasswordLabel, util.PinLabel}); err != nil {
		return errors.Wrap(err, "list of keys is incorrect after adding key")
	}

	// Mount and unmount with the new key.
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault mounted before testing AddKeyEx")
	}
	if err := utility.MountVault(ctx, util.PinLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPin), false, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to mount vault with the added key")
	}
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with added key")
	}

	// CheckKeyEx should work correctly with both the new and old key.
	if err := testCheckKeyEx(ctx, utility, util.FirstUsername, util.PasswordLabel, util.FirstPassword, util.IncorrectPassword); err != nil {
		return errors.Wrap(err, "old key malfunctions while mounted with added key")
	}
	if err := testCheckKeyEx(ctx, utility, util.FirstUsername, util.PinLabel, util.FirstPin, util.IncorrectPassword); err != nil {
		return errors.Wrap(err, "new key malfunctions while mounted with added key")
	}
	// Now unmount it.
	if err := unmountTestVault(ctx, utility); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}

	// The old key should still work as expected.
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), false, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to mount vault with the old key after adding pin")
	}
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with old key after adding pin")
	}
	// CheckKeyEx should work correctly with both the new and old key.
	if err := testCheckKeyEx(ctx, utility, util.FirstUsername, util.PasswordLabel, util.FirstPassword, util.IncorrectPassword); err != nil {
		return errors.Wrap(err, "old key malfunctions while mounted with old key")
	}
	if err := testCheckKeyEx(ctx, utility, util.FirstUsername, util.PinLabel, util.FirstPin, util.IncorrectPassword); err != nil {
		return errors.Wrap(err, "new key malfunctions while mounted with old key")
	}
	if err := unmountTestVault(ctx, utility); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}

	// RemoveKeyEx should work, and test everything is alright after removing the key.
	if err := utility.RemoveVaultKey(ctx, util.FirstUsername, util.FirstPassword, util.PinLabel); err != nil {
		return errors.Wrap(err, "failed to remove key")
	}
	if err := utility.MountVault(ctx, util.PinLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPin), false, hwsec.NewVaultConfig()); err == nil {
		return errors.New("still can mount vault with removed key")
	}
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault mounted after unmounting and failed mount")
	}

	return nil
}

// testMigrateKeyEx tests that MigrateKeyEx() works correctly.
// Note: MigrateKeyEx() changes the vault password.
func testMigrateKeyEx(ctx context.Context, utility *hwsec.CryptohomeClient) error {
	// MigrateKeyEx should work, and check if both new and old password behave as expected.
	if err := utility.ChangeVaultPassword(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, util.FirstChangedPassword); err != nil {
		return errors.Wrap(err, "failed to change vault password")
	}
	if err := testCheckKeyEx(ctx, utility, util.FirstUsername, util.PasswordLabel, util.FirstChangedPassword, util.FirstPassword); err != nil {
		return errors.Wrap(err, "incorrect CheckKeyEx behaviour right after MigrateKeyEx")
	}
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), false, hwsec.NewVaultConfig()); err == nil {
		return errors.New("still can mount vault with old password")
	}

	// Mount with new password and try MigrateKeyEx() again.
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstChangedPassword), false, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to mount with new password")
	}
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with changed password")
	}
	if err := utility.ChangeVaultPassword(ctx, util.FirstUsername, util.FirstChangedPassword, util.PasswordLabel, util.FirstPassword); err != nil {
		return errors.Wrap(err, "failed to change vault password back when mounted")
	}
	if err := checkMountState(ctx, utility, true); err != nil {
		return errors.Wrap(err, "vault not mounted after changing password while mounted")
	}

	// Calling CheckKeyEx(), which tries to authenticate with the new secret,
	// should be effective immediately without a remount.
	if err := testCheckKeyEx(ctx, utility, util.FirstUsername, util.PasswordLabel, util.FirstPassword, util.FirstChangedPassword); err != nil {
		return errors.Wrap(err, "incorrect CheckKeyEx behaviour right after password is changed back")
	}

	// The new secret should continue to work when we try to authenticate
	// through CheckKeyEx() API, even after we unmount.
	if err := unmountTestVault(ctx, utility); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	if err := checkMountState(ctx, utility, false); err != nil {
		return errors.Wrap(err, "vault mounted after unmounting while testing MigrateKeyEx")
	}
	if err := testCheckKeyEx(ctx, utility, util.FirstUsername, util.PasswordLabel, util.FirstPassword, util.FirstChangedPassword); err != nil {
		return errors.Wrap(err, "incorrect CheckKeyEx behaviour after the password is changed back")
	}

	return nil
}

// checkUserVault checks that the vault/keys related API works correctly.
func checkUserVault(ctx context.Context, utility *hwsec.CryptohomeClient) error {
	if err := testCheckKeyEx(ctx, utility, util.FirstUsername, util.PasswordLabel, util.FirstPassword, util.IncorrectPassword); err != nil {
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

	return nil
}

// CryptohomeKeysAndMountValidity exercizes and tests the correctness of
// cryptohome's key and vault related APIs when the DUT goes through
// various states (ownership not taken, ownership taken, after reboot).
func CryptohomeKeysAndMountValidity(ctx context.Context, s *testing.State) {
	r := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility := helper.CryptohomeClient()

	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	// Create the user and check it is correctly mounted and can be unmounted.
	func() {
		if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
			s.Fatal("Failed to create user: ", err)
		}
		// Unmount within this closure, because we want to have the thing
		// unmounted for checkUserVault() to work.
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
		// Remove the vault then verify that it's not mountable and that nothing is mounted.
		if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
		if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), false, hwsec.NewVaultConfig()); err == nil {
			s.Fatal("Still can mount vault after removing vault: ")
		}
		if err := checkMountState(ctx, utility, false); err != nil {
			s.Fatal(err, "Vault mounted after removing vault: ")
		}
	}()

	// Take ownership, confirming the vault state before and afterwards.
	if err := checkUserVault(ctx, utility); err != nil {
		s.Fatal("Check user failed: ", err)
	}
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Time out waiting for TPM to be ready: ", err)
	}
	if err := checkUserVault(ctx, utility); err != nil {
		s.Fatal("Check user failed: ", err)
	}

	// Reboot then confirm vault status.
	if err := helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}
	if err := checkUserVault(ctx, utility); err != nil {
		s.Fatal("Check user failed: ", err)
	}
}
