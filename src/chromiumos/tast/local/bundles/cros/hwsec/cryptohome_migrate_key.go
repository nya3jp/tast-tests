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
		Func: CryptohomeMigrateKey,
		Desc: "Tests Cryptohome's ability to migrate the key",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@google.com",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline"},
	})
}

const (
	user        = util.FirstUsername
	oldPassword = util.FirstPassword
	newPassword = util.SecondPassword
	badPassword = util.ThirdPassword
)

// createAccountAndChangePassword creates an account with oldPassword and changes the password to newPassword.
// It uses testPassword to change the password, and changePasswordShouldFail represents the action of changing password should fail or not.
func createAccountAndChangePassword(ctx context.Context, cryptohome *hwsec.CryptohomeClient, testPassword string, changePasswordShouldFail bool) error {
	// Create the account.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, oldPassword), true, hwsec.NewVaultConfig()); err != nil {
		return errors.Wrap(err, "failed to mount vault")
	}
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		return errors.Wrap(err, "failed to unmount vault")
	}

	err := cryptohome.ChangeVaultPassword(ctx, user, testPassword, util.PasswordLabel, newPassword)
	if !changePasswordShouldFail && err != nil {
		return errors.Wrap(err, "failed to change vault password")
	}
	if changePasswordShouldFail && err == nil {
		return errors.New("changing password unexpectedly succeeded")
	}
	return nil
}

// migrateGoodKeyTest checks that migrating the key with good password works correctly.
func migrateGoodKeyTest(ctx context.Context, s *testing.State, cryptohome *hwsec.CryptohomeClient) {
	// Create account and migrate the key with good password.
	if err := createAccountAndChangePassword(ctx, cryptohome, oldPassword, false); err != nil {
		s.Fatal("Failed to create account and migrate the key: ", err)
	}

	// We expect the mount should fail, because we are using old password.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, oldPassword), true, hwsec.NewVaultConfig()); err == nil {
		s.Fatal("Cryptohome was successfully mounted with the old password; want: should have failed")
	}

	// Try the correct password.
	if err := cryptohome.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(user, newPassword), true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault with correct password: ", err)
	}
}

// migrateBadKeyTest checks that migrating the key with bad password should fail.
func migrateBadKeyTest(ctx context.Context, s *testing.State, cryptohome *hwsec.CryptohomeClient) {
	// Create account and migrate the key with bad password.
	if err := createAccountAndChangePassword(ctx, cryptohome, badPassword, true); err != nil {
		s.Fatal("Migrate bad key test fail: ", err)
	}
}

// migrateNonexistUserTest checks that migrating the key of non-exist user should fail.
func migrateNonexistUserTest(ctx context.Context, s *testing.State, cryptohome *hwsec.CryptohomeClient) {
	// Migrating the key of non-exist user should fail.
	if err := cryptohome.ChangeVaultPassword(ctx, user, oldPassword, util.PasswordLabel, newPassword); err == nil {
		s.Fatal("Password was successfully changed for non-existent user; want: should have failed")
	}
}

// CryptohomeMigrateKey checks that cryptohome could migrate the key and login correctly.
func CryptohomeMigrateKey(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	// Ensure clean cryptohome.
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}
	if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	for _, tc := range []struct {
		name string
		test func(context.Context, *testing.State, *hwsec.CryptohomeClient)
	}{
		{
			// Migrate the key with good password.
			name: "good_password",
			test: migrateGoodKeyTest,
		},
		{
			// Migrate the key with bad password.
			name: "bad_password",
			test: migrateBadKeyTest,
		},
		{
			// Migrate the key of non-exist user.
			name: "nonexistent_user",
			test: migrateNonexistUserTest,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			defer func() {
				// Ensure we remove the user account after each test.
				if _, err := cryptohome.Unmount(ctx, user); err != nil {
					s.Fatal("Failed to unmount: ", err)
				}
				if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
					s.Fatal("Failed to remove vault: ", err)
				}
			}()

			// Run the test.
			tc.test(ctx, s, cryptohome)
		})
	}
}
