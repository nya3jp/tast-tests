// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
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
		Attr:         []string{"group:mainline", "informational"},
	})
}

const (
	user    = "foo@example.com"
	oldPass = "old-password"
	newPass = "new-password"
	badPass = "bad-password"
)

func migrateGoodKeyTest(ctx context.Context, s *testing.State, cryptohome *hwsec.CryptohomeClient) {
	// Create the account.
	if err := cryptohome.MountVault(ctx, user, oldPass, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault: ", err)
	}
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}

	// Migrate the key with good password.
	if err := cryptohome.ChangeVaultPassword(ctx, user, oldPass, util.PasswordLabel, newPass); err != nil {
		s.Fatal("Failed to chang vault password: ", err)
	}

	// We expect the mount should fail, because we are using old password.
	if err := cryptohome.MountVault(ctx, user, oldPass, util.PasswordLabel, true, hwsec.NewVaultConfig()); err == nil {
		s.Fatal("Old password still works")
	}

	// Try the correct password.
	if err := cryptohome.MountVault(ctx, user, newPass, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault with correct password: ", err)
	}
}

func migrateBadKeyTest(ctx context.Context, s *testing.State, cryptohome *hwsec.CryptohomeClient) {
	// Create the account.
	if err := cryptohome.MountVault(ctx, user, oldPass, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault: ", err)
	}
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}

	// Migrating the key with bad password should failed.
	if err := cryptohome.ChangeVaultPassword(ctx, user, badPass, util.PasswordLabel, newPass); err == nil {
		s.Fatal("Migrated with bad password")
	}
}

func migrateNonexistUserTest(ctx context.Context, s *testing.State, cryptohome *hwsec.CryptohomeClient) {
	// Migrating the key of non-exist user should failed.
	if err := cryptohome.ChangeVaultPassword(ctx, user, oldPass, util.PasswordLabel, newPass); err == nil {
		s.Fatal("Migrated a nonexistent user")
	}
}

// CryptohomeMigrateKey checks that cryptohome could migrate the key and login correctly.
func CryptohomeMigrateKey(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
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

			// Run the test
			tc.test(ctx, s, cryptohome)
		})
	}
}
