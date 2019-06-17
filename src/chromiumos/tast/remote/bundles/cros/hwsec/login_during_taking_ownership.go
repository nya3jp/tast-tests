// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	libhwsec "chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	libhwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LoginDuringTakingOwnership,
		Desc:         "Verifies that the TPM ownership can be cleared and taken",
		Contacts:     []string{"cylai@google.com"},
		SoftwareDeps: []string{"reboot"},
		Attr:         []string{"informational"},
	})
}

func LoginDuringTakingOwnership(ctx context.Context, s *testing.State) {

	s.Log("Start test with creating a helper and a utility")
	helper, err := libhwsecremote.NewHelperRemote(s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}
	utility, err := libhwsec.NewUtility(ctx, helper, libhwsec.CryptohomeBinaryType)
	if err != nil {
		s.Fatal("Utilty creation error: ", err)
	}

	loginFunc := func(username, passwd string) error {
		s.Log("Start creating vault")
		result, err := utility.CreateVault(username, passwd)
		if err != nil {
			return errors.Wrap(err, "error during create vault w/o tpm ownership")
		} else if !result {
			return errors.New("failed to create vault w/o tpm ownership")
		}
		s.Log("Checking if the vault is TPM-backed")
		result, err = utility.IsTpmWrappedKeySet(username)
		if err != nil {
			return errors.Wrap(err, "error checking if vault key set is tpm wrapped")
		}
		if result {
			return errors.New("vault key set is tpm wrapped what~~~~~")
		}
		s.Log("Finished creating non-TPM-backed vault")
		return nil
	}
	loginRoutine := func(username, passwd string, err chan error) {
		err <- loginFunc(username, passwd)
	}
	takeOwnershipFunc := func() error {
		s.Log("Start taking ownership")
		if err := libhwsec.EnsureTpmIsReady(ctx, utility, 40*1000); err != nil {
			return errors.Wrap(err, "failed to ensure ownership: ")
		}
		s.Log("Ownership is taken")
		return nil
	}
	takeOwnershipRoutine := func(err chan error) {
		err <- takeOwnershipFunc()
	}

	s.Log("Start resetting TPM if needed")
	if err := libhwsec.EnsureTpmIsReset(ctx, helper, utility); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")
	loginErr := make(chan error)
	ownershipErr := make(chan error)
	username := "unowned@gmail.com"
	go loginRoutine(username, "passwd", loginErr)
	go takeOwnershipRoutine(ownershipErr)
	err1, err2 := <-loginErr, <-ownershipErr
	if err1 != nil || err2 != nil {
		s.Fatal("concurrenty error: ", err1, err2)
	}
	result, err := utility.Unmount(username)
	if err != nil {
		s.Fatal("Error unmounting user: ", err)
	}
	if !result {
		s.Fatal("Failed to unmount user: ", err)
	}
	result, err = utility.RemoveVault(username)
	if err != nil {
		s.Fatal("Error removing vault: ", err)
	}
	if !result {
		s.Fatal("Failed to remove vault: ", err)
	}

}
