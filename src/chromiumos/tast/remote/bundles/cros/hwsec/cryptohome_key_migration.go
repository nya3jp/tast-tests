// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	libhwsecremote "chromiumos/tast/remote/hwsec"
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

	s.Log("Start test with creating a helper and a utility")
	helper, err := libhwsecremote.NewHelperRemote(s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility, err := libhwsec.NewUtility(ctx, helper, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}
	s.Log("Start resetting TPM if needed")
	if err := libhwsec.EnsureTpmIsReset(ctx, helper, utility); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	s.Log("Creating a new mount for test user")
	username := "unowned-then-owned@gmail.com"
	passwd := "testpass"

	result, err := utility.CreateVault(username, passwd)
	if err != nil {
		s.Fatal("Error during create vault w/o tpm ownership: ", err)
	} else if !result {
		s.Fatal("Failed to create vault w/o tpm ownership")
	}

	result, err = utility.IsTpmWrappedKeySet(username)
	if err != nil {
		s.Fatal("Error checking if vault key set is tpm wrapped: ", err)
	}
	if result {
		s.Fatal("vault key set is tpm wrapped what~~~~~")
	}

	helper.Reboot(ctx)
	s.Log("Start taking ownership")
	if err := libhwsec.EnsureTpmIsReady(ctx, utility, 40*1000); err != nil {
		s.Fatal("Failed to ensure ownership: ", err)
	}
	s.Log("Onwership is taken")
	s.Log("Creating a new mount for the same test user")
	result, err = utility.CreateVault(username, passwd)
	if err != nil {
		s.Fatal("Error during create vault with tpm ownership: ", err)
	} else if !result {
		s.Fatal("Failed to create vault with tpm ownership")
	}
	result, err = utility.IsTpmWrappedKeySet(username)
	if err != nil {
		s.Fatal("Error checking if vault key set is tpm wrapped: ", err)
	}
	if !result {
		s.Fatal("Vault key set is not tpm wrapped")
	}
	result, err = utility.Unmount(username)
	if err != nil {
		s.Fatal("Error unmounting user: ", err)
	}
	result, err = utility.RemoveVault(username)
	if err != nil {
		s.Fatal("Error removing vault: ", err)
	}
}
