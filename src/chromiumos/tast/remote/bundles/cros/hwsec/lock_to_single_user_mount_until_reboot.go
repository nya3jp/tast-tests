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

type testParam struct {
	pinWeaverSupported bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: LockToSingleUserMountUntilReboot,
		Desc: "Checks that LockToSingleUserMountUntilReboot method works",
		Contacts: []string{
			"zuan@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"informational", "group:mainline"},
		SoftwareDeps: []string{"tpm"},
		Params: []testing.Param{
			{
				Val: testParam{pinWeaverSupported: false},
			},
			{
				Name:              "pin_weaver",
				ExtraSoftwareDeps: []string{"pinweaver"},
				Val:               testParam{pinWeaverSupported: true},
			},
		},
	})
}

// create2VaultsForTesting will create 2 vaults for testing.
func create2VaultsForTesting(ctx context.Context, utility *hwsec.CryptohomeClient, pinSupported bool) (returnedErr error) {
	// Cleanup if we failed.
	defer func() {
		if returnedErr != nil {
			if err := utility.UnmountAll(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to unmount when cleaning up create2VaultsForTesting: ", err)
			}

			if err := cleanupVault(ctx, utility); err != nil {
				testing.ContextLog(ctx, "Failed to cleanup vault when cleanup create2VaultsForTesting: ", err)
			}
		}
	}()

	// Create 2 vaults for testing.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to create first user")
	}
	if err := utility.MountVault(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to create second user")
	}

	// Unmount the vault before further testing.
	if err := utility.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount vault before testing")
	}

	// Add a second vault key.
	if err := utility.AddVaultKey(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, util.FirstPassword2, util.Password2Label, false); err != nil {
		return errors.Wrap(err, "failed to add key to vault for first user")
	}
	if err := utility.AddVaultKey(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, util.SecondPassword2, util.Password2Label, false); err != nil {
		return errors.Wrap(err, "failed to add key to vault for second user")
	}

	// If PinWeaver is supported, add a third vault key, which is a le credential (PIN).
	if pinSupported {
		if err := utility.AddVaultKey(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, util.FirstPin, util.PinLabel, true); err != nil {
			return errors.Wrap(err, "failed to add key to vault for first user")
		}
		if err := utility.AddVaultKey(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, util.SecondPin, util.PinLabel, true); err != nil {
			return errors.Wrap(err, "failed to add key to vault for second user")
		}
	}

	return nil
}

// cleanupVault will delete the first and second user's vault.
func cleanupVault(ctx context.Context, utility *hwsec.CryptohomeClient) (returnedErr error) {
	// Unmount first.
	if err := utility.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount vaults before cleanup")
	}
	// Remove the vault.
	if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
		returnedErr = errors.Wrap(err, "failed to remove first user's vault")
		// Note: We are not returning here because we want to cleanup the
		// second vault as well.
	}
	if _, err := utility.RemoveVault(ctx, util.SecondUsername); err != nil {
		returnedErr = errors.Wrap(err, "failed to remove second user's vault")
	}
	return returnedErr
}

// checkVaultWorks will check that the vault specified by username, both passwords and pin (if supported) works in both mounting and unlock (CheckKeyEx).
func checkVaultWorks(ctx context.Context, utility *hwsec.CryptohomeClient, username, password1, password2, pin string, pinSupported bool) error {
	if err := utility.MountVault(ctx, username, password1, util.PasswordLabel, false, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to mount with password1")
	}
	if err := utility.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount vault")
	}
	if err := utility.MountVault(ctx, username, password2, util.Password2Label, false, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to mount with password2")
	}
	if err := utility.UnmountAll(ctx); err != nil {
		return errors.Wrap(err, "failed to unmount vault")
	}
	if pinSupported {
		if err := utility.MountVault(ctx, username, pin, util.PinLabel, false, hwsec.NewVaultConfig()); err != nil {
			return errors.Wrap(err, "failed to mount with pin")
		}
		if err := utility.UnmountAll(ctx); err != nil {
			return errors.Wrap(err, "failed to unmount vault")
		}
	}

	if result, _ := utility.CheckVault(ctx, username, password1, util.PasswordLabel); !result {
		return errors.New("failed to check key with password1")
	}
	if result, _ := utility.CheckVault(ctx, username, password2, util.Password2Label); !result {
		return errors.New("failed to check key with password2")
	}
	if pinSupported {
		if result, _ := utility.CheckVault(ctx, username, pin, util.PinLabel); !result {
			return errors.New("failed to check key with pin")
		}
	}

	return nil
}

// checkBothVaultsAreOperational will check that both first user's vault and second user's vault are operational.
func checkBothVaultsAreOperational(ctx context.Context, utility *hwsec.CryptohomeClient, pinSupported bool) error {
	// Now test that logging in on both user works as expected.
	if err := checkVaultWorks(ctx, utility, util.FirstUsername, util.FirstPassword, util.FirstPassword2, util.FirstPin, pinSupported); err != nil {
		return errors.Wrap(err, "first user's vault doesn't work")
	}
	if err := checkVaultWorks(ctx, utility, util.SecondUsername, util.SecondPassword, util.SecondPassword2, util.SecondPin, pinSupported); err != nil {
		return errors.Wrap(err, "second user's vault doesn't work")
	}

	return nil
}

// checkOthersAreBlocked will check that users other than util.FirstUsername are blocked from mounting and CheckKeyEx (unlock).
func checkOthersAreBlocked(ctx context.Context, utility *hwsec.CryptohomeClient, pinSupported bool) error {
	// Mount should fail.
	if err := utility.MountVault(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel, false, hwsec.NewVaultConfig()); err == nil {
		return errors.Wrap(err, "second user is mountable with password1 after locking to single user")
	}
	if err := utility.MountVault(ctx, util.SecondUsername, util.SecondPassword2, util.Password2Label, false, hwsec.NewVaultConfig()); err == nil {
		return errors.Wrap(err, "second user is mountable with password2 after locking to single user")
	}
	if pinSupported {
		if err := utility.MountVault(ctx, util.SecondUsername, util.SecondPin, util.PinLabel, false, hwsec.NewVaultConfig()); err == nil {
			return errors.Wrap(err, "second user is mountable with pin after locking to single user")
		}
	}

	// CheckKeyEx should fail too.
	if result, _ := utility.CheckVault(ctx, util.SecondUsername, util.SecondPassword, util.PasswordLabel); result {
		return errors.New("check key succeeded with password1 after locking to single user")
	}
	if result, _ := utility.CheckVault(ctx, util.SecondUsername, util.SecondPassword2, util.Password2Label); result {
		return errors.New("check key succeeded with password2 after locking to single user")
	}
	if pinSupported {
		if result, _ := utility.CheckVault(ctx, util.SecondUsername, util.SecondPin, util.PinLabel); result {
			return errors.New("check key succeeded with pin after locking to single user")
		}
	}

	// Shouldn't be able to create new users.
	if err := utility.MountVault(ctx, util.ThirdUsername, util.ThirdPassword, util.PasswordLabel, true, hwsec.NewVaultConfig()); err == nil {
		return errors.Wrap(err, "create user succeeded after locking to single user")
	}
	return nil
}

func LockToSingleUserMountUntilReboot(ctx context.Context, s *testing.State) {
	// Standard initializations.
	r := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(r, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility := helper.CryptohomeClient()

	pinSupported := s.Param().(testParam).pinWeaverSupported

	// LockToSingleUserMountUntilReboot would only available when the TPM is ready.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned: ", err)
	}

	// Create the vaults for testing.
	if err := create2VaultsForTesting(ctx, utility, pinSupported); err != nil {
		s.Fatal("Failed to initialize vaults for testing: ", err)
	}
	defer func() {
		if err := cleanupVault(ctx, utility); err != nil {
			s.Error("Failed to cleanup vault: ", err)
		}
	}()

	// Before starting the actual test, check that everything is alright.
	if err := checkBothVaultsAreOperational(ctx, utility, pinSupported); err != nil {
		s.Fatal("Initially created vaults doesn't work: ", err)
	}

	// Now mount the first user's vault and lock to single user mount.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword2, util.Password2Label, false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount the user for lock to single user mount: ", err)
	}
	if err := utility.LockToSingleUserMountUntilReboot(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to lock to single user mount: ", err)
	}

	// Check that the other user is blocked.
	if err := checkOthersAreBlocked(ctx, utility, pinSupported); err != nil {
		s.Fatal("Other users are not blocked: ", err)
	}

	// Unmount the first user's vault.
	if _, err := utility.Unmount(ctx, util.FirstUsername); err != nil {
		s.Fatal("Failed to unmount first user's vault after locking: ", err)
	}

	// The first user's vault should still work (CheckKeyEx and MountEx).
	if err := checkVaultWorks(ctx, utility, util.FirstUsername, util.FirstPassword, util.FirstPassword2, util.FirstPin, pinSupported); err != nil {
		s.Fatal("The first user's vault doesn't work after locking: ", err)
	}

	// Now reboot and check that the effects wear off.
	if err := helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}

	if err := checkBothVaultsAreOperational(ctx, utility, pinSupported); err != nil {
		s.Fatal("Vaults doesn't work after reboot: ", err)
	}
}
