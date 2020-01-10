// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LoginDuringTakingOwnership,
		Desc:         "Verifies that login is workin during TPM ownership is being taken",
		Contacts:     []string{"cylai@chromium.org", "cros-hwsec@google.com"},
		SoftwareDeps: []string{"reboot", "tpm"},
		Attr:         []string{"group:mainline", "disabled"},
	})
}

func LoginDuringTakingOwnership(ctx context.Context, s *testing.State) {
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

	loginErr := make(chan error)
	loginRoutine := func(username, passwd string) {
		loginErr <- func() error {
			s.Log("Start creating vault")
			if err := utility.MountVault(ctx, username, passwd, "dontcare", true); err != nil {
				return errors.Wrap(err, "error during create vault")
			}
			return nil
		}()
	}

	ownershipErr := make(chan error)
	takeOwnershipRoutine := func() {
		ownershipErr <- func() error {
			s.Log("Start taking ownership")
			if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
				return errors.Wrap(err, "failed to ensure ownership")
			}
			s.Log("Ownership is taken")
			return nil
		}()
	}

	const username = "unowned@gmail.com"
	const passwd = "passwd"
	go loginRoutine(username, passwd)
	go takeOwnershipRoutine()

	for i := 0; i < 2; i++ {
		select {
		case err := <-loginErr:
			if err != nil {
				s.Error("login error: ", err)
			}
		case err := <-ownershipErr:
			if err != nil {
				s.Error("ownership error: ", err)
			}
		}
	}

	result, err := utility.Unmount(ctx, username)
	if err != nil {
		s.Fatal("Error unmounting user: ", err)
	}
	if !result {
		s.Fatal("Failed to unmount user: ", err)
	}
	result, err = utility.RemoveVault(ctx, username)
	if err != nil {
		s.Fatal("Error removing vault: ", err)
	}
	if !result {
		s.Fatal("Failed to remove vault: ", err)
	}
}
