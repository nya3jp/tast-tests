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
		Func: CryptohomeMount,
		Desc: "Tests Cryptohome's ability to mount the user folder",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"yich@google.com",
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

// CryptohomeMount checks that cryptohome could mount the user folder correctly.
func CryptohomeMount(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)

	const (
		user     = "this_is_a_local_test_account@chromium.org"
		goodPass = "this_is_a_test_password"
		badPass  = "this_is_an_incorrect_password"
	)

	defer func() {
		// Ensure we remove the user account after the test.
		if _, err := cryptohome.Unmount(ctx, user); err != nil {
			s.Fatal("Failed to unmount: ", err)
		}
		if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
			s.Fatal("Failed to remove vault: ", err)
		}
	}()

	// Ensure clean cryptohome.
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}
	if _, err := cryptohome.RemoveVault(ctx, user); err != nil {
		s.Fatal("Failed to remove vault: ", err)
	}

	// Create the account.
	if err := cryptohome.MountVault(ctx, user, goodPass, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to mount vault: ", err)
	}

	// Unmount the vault.
	if _, err := cryptohome.Unmount(ctx, user); err != nil {
		s.Fatal("Failed to unmount: ", err)
	}

}
