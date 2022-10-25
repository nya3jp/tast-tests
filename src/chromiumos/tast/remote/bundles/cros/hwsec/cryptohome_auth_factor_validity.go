// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	uda "chromiumos/system_api/user_data_auth_proto"
	cryptohomecommon "chromiumos/tast/common/cryptohome"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/storage/files"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CryptohomeAuthFactorValidity,
		Desc: "Checks that the AuthFactor APIs work in various TPM states",
		Contacts: []string{
			"jadmanski@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr:         []string{"group:hwsec_destructive_func"},
		SoftwareDeps: []string{"tpm", "reboot"},
		// Skip "enguarde" due to the reboot issue when removing the key. Please see b/151057300.
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("enguarde")),
		Timeout:      25 * time.Minute,
	})
}

// unmountVaultAndTest is a helper function that unmount the test vault. It
// expects the vault to be mounted when it is called.
func unmountVaultAndTest(ctx context.Context, client *hwsec.CryptohomeClient, hf *files.HomedirFiles) error {
	if _, err := client.Unmount(ctx, util.FirstUsername); err != nil {
		return errors.Wrap(err, "failed to unmount vault")
	}
	if err := testMountState(ctx, client, hf, false /*shouldBeMounted*/); err != nil {
		return errors.Wrap(err, "vault still mounted after unmount")
	}
	return nil
}

// testMountState is a helper function that returns an error if the result
// from IsMounted() is not equal to expected, or if there is a problem calling
// IsMounted().
//
// Also, in the case where the vault is expected mounted, it'll check that
// existing files are still available and correct. It will also add additional
// contents to the homedir so that subsequent checks can confirm that changes
// are not being lost.
func testMountState(ctx context.Context, client *hwsec.CryptohomeClient, hf *files.HomedirFiles, shouldBeMounted bool) error {
	actuallyMounted, err := client.IsMounted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to call IsMounted()")
	}
	if actuallyMounted != shouldBeMounted {
		return errors.Errorf("incorrect IsMounted() state %t, expected %t", actuallyMounted, shouldBeMounted)
	}
	if shouldBeMounted {
		// Mounted? Let's check the files.
		if err = hf.Verify(ctx); err != nil {
			return errors.Wrap(err, "homedir files failed to verify")
		}
		// Add more content to make sure contents are not lost.
		if err = hf.Step(ctx); err != nil {
			return errors.Wrap(err, "homedir files failed to step")
		}
	}
	return nil
}

// checkFactors checks if the list auth factors matches the given expected set
// and returns an error if it doesn't.
func checkFactors(ctx context.Context, client *hwsec.CryptohomeClient, expectedFactors []*uda.AuthFactorWithStatus) error {
	// Get a listing of all factors.
	listFactors, err := client.ListAuthFactors(ctx, util.FirstUsername)
	if err != nil {
		return errors.Wrap(err, "failed to list auth factors")
	}
	// Compare the results against the expected factors.
	if err = cryptohomecommon.ExpectAuthFactorsWithTypeAndLabel(
		listFactors.ConfiguredAuthFactorsWithStatus, expectedFactors); err != nil {
		return errors.Errorf("mismatch in configured auth factors (-got, +want) %s", err)
	}

	return nil
}

// testAuthenticate tests that Authenticating via password works as expected.
func testAuthenticate(ctx context.Context, client *hwsec.CryptohomeClient, username, label, correctPassword, incorrectPassword string) error {
	_, authSessionID, err := client.StartAuthSession(ctx, username, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_VERIFY_ONLY)
	if err != nil {
		return errors.Wrap(err, "failed to start session to check passwords")
	}
	defer client.InvalidateAuthSession(ctx, authSessionID)

	// Authorization should fail with the incorrect password.
	reply, err := client.AuthenticateAuthFactor(ctx, authSessionID, label, incorrectPassword)
	if err == nil {
		return errors.New("incorrect password successfully authenticated")
	}
	if reply == nil {
		return errors.New("empty error on unsuccessful login")
	}
	if reply.ErrorInfo.PrimaryAction != uda.PrimaryAction_PRIMARY_INCORRECT_AUTH {
		return errors.New("incorrect auth does not yield the correct PrimaryAction in reply")
	}

	// Authorization should work with the correct password.
	if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, label, correctPassword); err != nil {
		return errors.Wrap(err, "correct password failed to authenticate")
	}

	return nil
}

// testAddRemoveFactor tests that adding and removing a factor work as expected.
func testAddRemoveFactor(ctx context.Context, client *hwsec.CryptohomeClient, hf *files.HomedirFiles) error {
	// Add the new factor.
	err := func() error {
		_, authSessionID, err := client.StartAuthSession(ctx, util.FirstUsername, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		defer client.InvalidateAuthSession(ctx, authSessionID)
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, util.Password1Label, util.FirstPassword1); err != nil {
			return errors.Wrap(err, "password failed to authenticate")
		}
		if err := client.AddAuthFactor(ctx, authSessionID, util.Password2Label, util.FirstPassword2); err != nil {
			return errors.Wrap(err, "failed to add password auth factor")
		}
		return nil
	}()
	if err != nil {
		return errors.Wrap(err, "failed to add new factor")
	}

	// Now that the factor is added, check the expected factors.
	if err := checkFactors(ctx, client,
		[]*uda.AuthFactorWithStatus{
			{AuthFactor: &uda.AuthFactor{
				Type:  uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD,
				Label: util.Password1Label,
			}},
			{AuthFactor: &uda.AuthFactor{
				Type:  uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD,
				Label: util.Password2Label,
			}},
		}); err != nil {
		return errors.Wrap(err, "list of factors is incorrect after adding factor")
	}

	// Mount using the new factor.
	err = func() error {
		_, authSessionID, err := client.StartAuthSession(ctx, util.FirstUsername, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		defer client.InvalidateAuthSession(ctx, authSessionID)
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, util.Password2Label, util.FirstPassword2); err != nil {
			return errors.Wrap(err, "password failed to authenticate")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			return errors.Wrap(err, "failed to prepare persistent vault")
		}
		return nil
	}()
	if err != nil {
		return errors.Wrap(err, "failed to mount user with the new factor")
	}
	if err := testMountState(ctx, client, hf, true /*shouldBeMounted*/); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with added factor")
	}
	// Authentication should work correctly with both the new and old factor.
	if err := testAuthenticate(ctx, client, util.FirstUsername, util.Password1Label, util.FirstPassword1, util.IncorrectPassword); err != nil {
		return errors.Wrap(err, "old factor malfunctions while mounted with added factor")
	}
	if err := testAuthenticate(ctx, client, util.FirstUsername, util.Password2Label, util.FirstPassword2, util.IncorrectPassword); err != nil {
		return errors.Wrap(err, "new factor malfunctions while mounted with added factor")
	}
	// Now unmount it.
	if err := unmountVaultAndTest(ctx, client, hf); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}

	// Mount using the old factor.
	err = func() error {
		_, authSessionID, err := client.StartAuthSession(ctx, util.FirstUsername, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		defer client.InvalidateAuthSession(ctx, authSessionID)
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, util.Password1Label, util.FirstPassword1); err != nil {
			return errors.Wrap(err, "password failed to authenticate")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			return errors.Wrap(err, "failed to prepare persistent vault")
		}
		return nil
	}()
	if err != nil {
		return errors.Wrap(err, "failed to mount user with the old factor")
	}
	if err := testMountState(ctx, client, hf, true /*shouldBeMounted*/); err != nil {
		return errors.Wrap(err, "vault not mounted after mounting with old factor")
	}
	// Authentication should work correctly with both the new and old factor after remount.
	if err := testAuthenticate(ctx, client, util.FirstUsername, util.Password1Label, util.FirstPassword1, util.IncorrectPassword); err != nil {
		return errors.Wrap(err, "old factor malfunctions while mounted with original factor")
	}
	if err := testAuthenticate(ctx, client, util.FirstUsername, util.Password2Label, util.FirstPassword2, util.IncorrectPassword); err != nil {
		return errors.Wrap(err, "new factor malfunctions while mounted with original factor")
	}
	// Now unmount it.
	if err := unmountVaultAndTest(ctx, client, hf); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}

	// Remove the new factor.
	err = func() error {
		_, authSessionID, err := client.StartAuthSession(ctx, util.FirstUsername, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		defer client.InvalidateAuthSession(ctx, authSessionID)
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, util.Password1Label, util.FirstPassword1); err != nil {
			return errors.Wrap(err, "password failed to authenticate")
		}
		if err := client.RemoveAuthFactor(ctx, authSessionID, util.Password2Label); err != nil {
			return errors.Wrap(err, "failed to remove password auth factor")
		}
		return nil
	}()
	if err != nil {
		return errors.Wrap(err, "failed to remove new password")
	}

	// After the new factor is removed it should no longer be able to mount.
	err = func() error {
		_, authSessionID, err := client.StartAuthSession(ctx, util.FirstUsername, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		defer client.InvalidateAuthSession(ctx, authSessionID)
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, util.Password2Label, util.FirstPassword2); err == nil {
			return errors.New("factor was still able to authenticate")
		}
		return nil
	}()
	if err != nil {
		return errors.Wrap(err, "testing mount failure of removed factor failed")
	}
	if err := testMountState(ctx, client, hf, false /*shouldBeMounted*/); err != nil {
		return errors.Wrap(err, "vault mounted after unmounting and failed mount")
	}

	return nil
}

// testUpdateFactor tests that updating a factor works as expected.
func testUpdateFactor(ctx context.Context, client *hwsec.CryptohomeClient, hf *files.HomedirFiles) error {
	// Update the existing factor.
	err := func() error {
		_, authSessionID, err := client.StartAuthSession(ctx, util.FirstUsername, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		defer client.InvalidateAuthSession(ctx, authSessionID)
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, util.Password1Label, util.FirstPassword1); err != nil {
			return errors.Wrap(err, "password failed to authenticate")
		}
		if err := client.UpdatePasswordAuthFactor(
			ctx, authSessionID,
			/*label=*/ util.Password1Label,
			/*newKeyLabel=*/ util.Password1Label,
			/*password=*/ util.FirstChangedPassword); err != nil {
			return errors.Wrap(err, "failed to update auth factor")
		}
		return nil
	}()
	if err != nil {
		return errors.Wrap(err, "failed to update the existing auth factor")
	}

	// Check that the new and old password both behave as expected.
	if err := testAuthenticate(ctx, client, util.FirstUsername, util.Password1Label, util.FirstChangedPassword, util.FirstPassword1); err != nil {
		return errors.Wrap(err, "authentication behaviour right after changing password")
	}

	// The old factor should no longer be able to authenticate but the new one
	// should be able to authenticate, mount and then change the factor back.
	err = func() error {
		_, authSessionID, err := client.StartAuthSession(ctx, util.FirstUsername, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			return errors.Wrap(err, "failed to start auth session")
		}
		defer client.InvalidateAuthSession(ctx, authSessionID)
		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, util.Password2Label, util.FirstPassword2); err == nil {
			return errors.New("old factor was still able to authenticate")
		}

		if err := testMountState(ctx, client, hf, false /*shouldBeMounted*/); err != nil {
			return errors.Wrap(err, "vault mounted after failed mount")
		}

		if _, err := client.AuthenticateAuthFactor(ctx, authSessionID, util.Password1Label, util.FirstChangedPassword); err != nil {
			return errors.Wrap(err, "new password failed to authenticate")
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			return errors.Wrap(err, "failed to prepare persistent vault")
		}
		if err := testMountState(ctx, client, hf, true /*shouldBeMounted*/); err != nil {
			return errors.Wrap(err, "vault not mounted after mounting with updated factor")
		}
		if err := client.UpdatePasswordAuthFactor(
			ctx, authSessionID,
			/*label=*/ util.Password1Label,
			/*newKeyLabel=*/ util.Password1Label,
			/*password=*/ util.FirstPassword1); err != nil {
			return errors.Wrap(err, "failed to update auth factor")
		}

		return nil
	}()
	if err != nil {
		return errors.Wrap(err, "failed to restore the original auth factor")
	}

	// Authentiating with the restored secret should be effective immediately without a remount.
	if err := testAuthenticate(ctx, client, util.FirstUsername, util.Password1Label, util.FirstPassword1, util.FirstChangedPassword); err != nil {
		return errors.Wrap(err, "authentication failed right after password is changed back")
	}

	// The restored factor should continue to work even after we unmount.
	if err := unmountVaultAndTest(ctx, client, hf); err != nil {
		return errors.Wrap(err, "failed to unmount")
	}
	if err := testMountState(ctx, client, hf, false /*shouldBeMounted*/); err != nil {
		return errors.Wrap(err, "vault mounted after unmounting while testing update")
	}
	if err := testAuthenticate(ctx, client, util.FirstUsername, util.Password1Label, util.FirstPassword1, util.FirstChangedPassword); err != nil {
		return errors.Wrap(err, "authentication failed after the password is changed back")
	}

	return nil
}

// testFactorAPI tests that the factor-related APIs works correctly.
func testFactorAPI(ctx context.Context, utility *hwsec.CryptohomeClient, hf *files.HomedirFiles) error {
	if err := testAuthenticate(ctx, utility, util.FirstUsername, util.Password1Label, util.FirstPassword1, util.IncorrectPassword); err != nil {
		return errors.Wrap(err, "test authentication failed")
	}

	if err := checkFactors(ctx, utility,
		[]*uda.AuthFactorWithStatus{
			{AuthFactor: &uda.AuthFactor{
				Type:  uda.AuthFactorType_AUTH_FACTOR_TYPE_PASSWORD,
				Label: util.Password1Label,
			}},
		}); err != nil {
		return errors.Wrap(err, "list of factors is incorrect")
	}

	if err := testAddRemoveFactor(ctx, utility, hf); err != nil {
		return errors.Wrap(err, "test of adding and removing factor failed")
	}

	if err := testUpdateFactor(ctx, utility, hf); err != nil {
		return errors.Wrap(err, "test of updating factor failed")
	}

	return nil
}

// CryptohomeAuthFactorValidity exercises and tests the correctness of adding
// and removing auth factors when the DUT goes through various states (ownership
// not taken, ownership taken, after reboot).
func CryptohomeAuthFactorValidity(ctx context.Context, s *testing.State) {
	cmdRunner := hwsecremote.NewCmdRunner(s.DUT())
	helper, err := hwsecremote.NewHelper(cmdRunner, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	client := helper.CryptohomeClient()

	s.Log("Start resetting TPM if needed")
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	hf, err := files.NewHomedirFiles(ctx, client, cmdRunner, util.FirstUsername)
	if err != nil {
		s.Fatal("Failed to create HomedirFiles for testing files in user's home directory: ", err)
	}

	// Create the user and check it is correctly mounted and can be unmounted.
	func() {
		// Start a session and set up a new user with a password factor.
		_, authSessionID, err := client.StartAuthSession(ctx, util.FirstUsername, false /*ephemeral*/, uda.AuthIntent_AUTH_INTENT_DECRYPT)
		if err != nil {
			s.Fatal("Failed to start auth session: ", err)
		}
		defer client.InvalidateAuthSession(ctx, authSessionID)
		if err := client.CreatePersistentUser(ctx, authSessionID); err != nil {
			s.Fatal("Failed to create persistent user: ", err)
		}
		if err := client.AddAuthFactor(ctx, authSessionID, util.Password1Label, util.FirstPassword1); err != nil {
			s.Fatal("Failed to add password auth factor: ", err)
		}
		if err := client.PreparePersistentVault(ctx, authSessionID, false /*ecryptfs*/); err != nil {
			s.Fatal("Failed to prepare persistent vault: ", err)
		}

		// Reset the state of the test files and Step() it once so we've something to start with.
		if err := hf.Clear(ctx); err != nil {
			s.Fatal("Failed to clear test files in the user's home directory: ", err)
		}
		if err := hf.Step(ctx); err != nil {
			s.Fatal("Failed to initialize the test files in the user's home directory: ", err)
		}

		// Unmount within this closure, because we want to have the thing
		// unmounted for checkUserVault() to work.
		defer func() {
			if err := unmountVaultAndTest(ctx, client, hf); err != nil {
				s.Fatal("Failed to unmount: ", err)
			}
		}()

		if err := testMountState(ctx, client, hf, true /*shouldBeMounted*/); err != nil {
			s.Fatal("Vault is not mounted: ", err)
		}
	}()

	// Cleanup the created vault.
	defer func() {
		// Remove the vault then verify that nothing is mounted.
		if _, err := client.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
		if err := testMountState(ctx, client, hf, false /*shouldBeMounted*/); err != nil {
			s.Fatal("Vault mounted after removing vault: ", err)
		}
	}()

	// Take ownership, confirming the vault state before and afterwards.
	if err = testFactorAPI(ctx, client, hf); err != nil {
		s.Fatal("Check user failed before taking ownership: ", err)
	}
	if err = helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Time out waiting for TPM to be ready: ", err)
	}
	if err = testFactorAPI(ctx, client, hf); err != nil {
		s.Fatal("Check user failed after taking ownership: ", err)
	}

	// Reboot then confirm vault status.
	if err = helper.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}
	if err = testFactorAPI(ctx, client, hf); err != nil {
		s.Fatal("Check user failed after reboot: ", err)
	}
}
