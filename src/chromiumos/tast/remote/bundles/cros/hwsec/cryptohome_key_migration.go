// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CryptohomeKeyMigration,
		Desc:         "Verifies that the TPM ownership can be cleared and taken",
		Contacts:     []string{"cylai@google.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Attr:         []string{"informational"},
	})
}

func CryptohomeKeyMigration(ctx context.Context, s *testing.State) {
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

	s.Log("Creating a new mount for test user")
	username := "unowned-then-owned@gmail.com"
	passwd := "testpass"

	result, err := utility.CreateVault(ctx, username, passwd)
	if err != nil {
		s.Fatal("Error during create vault w/o tpm ownership: ", err)
	} else if !result {
		s.Fatal("Failed to create vault w/o tpm ownership")
	}

	result, err = utility.IsTPMWrappedKeySet(ctx, username)
	if err != nil {
		s.Fatal("Error checking if vault key set is tpm wrapped: ", err)
	}
	if result {
		s.Fatal("Vault key set is tpm wrapped what~~~~~")
	}

	helper.Reboot(ctx)
	s.Log("Start taking ownership")
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to ensure ownership: ", err)
	}
	s.Log("Ownership is taken")
	s.Log("Creating a new mount for the same test user")
	result, err = utility.CreateVault(ctx, username, passwd)
	if err != nil {
		s.Fatal("Error during create vault with tpm ownership: ", err)
	} else if !result {
		s.Fatal("Failed to create vault with tpm ownership")
	}
	result, err = utility.IsTPMWrappedKeySet(ctx, username)
	if err != nil {
		s.Fatal("Error checking if vault key set is tpm wrapped: ", err)
	}
	if !result {
		s.Fatal("Vault key set is not tpm wrapped")
	}
	result, err = utility.CheckVault(ctx, username, passwd)
	if err != nil {
		s.Fatal("Error checking user vault: ", err)
	} else if !result {
		s.Fatal("Failed to check vault")
	}
	result, err = utility.Unmount(ctx, username)
	if err != nil {
		s.Fatal("Error unmounting user: ", err)
	}
	result, err = utility.RemoveVault(ctx, username)
	if err != nil {
		s.Fatal("Error removing vault: ", err)
	}

	s.Log("Clearing tpm_manager's local data")
	dCtrl := hwsec.NewDaemonController(r)
	dCtrl.StopTpmManager(ctx)
	w := hwsec.NewFileWiper(r)
	if err := w.Wipe(ctx, hwsec.TpmManagerLocalDataPath); err != nil {
		s.Fatal("Failed to wipe tpm_manager local data")
	}
	dCtrl.StartTpmManager(ctx)
	defer func() {
		dCtrl.StopTpmManager(ctx)
		if err := w.Restore(ctx, hwsec.TpmManagerLocalDataPath); err != nil {
			s.Fatal("Failed to restore tpm_manager local data")
		}
		dCtrl.StartTpmManager(ctx)
	}()

	username = "owned-no-db@gmail.com"
	result, err = utility.CreateVault(ctx, username, passwd)
	if err != nil {
		s.Fatal("Error during create vault with tpm ownership: ", err)
	} else if !result {
		s.Fatal("Failed to create vault with tpm ownership")
	}
	result, err = utility.IsTPMWrappedKeySet(ctx, username)
	if err != nil {
		s.Fatal("Error checking if vault key set is tpm wrapped: ", err)
	}
	if !result {
		s.Fatal("Vault key set is not tpm wrapped")
	}
}
