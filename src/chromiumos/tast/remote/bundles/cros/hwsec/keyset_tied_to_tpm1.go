// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/remote/bundles/cros/hwsec/util"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

// NOTE: This test is largely similar to hwsec.KeysetTiedToTPM2 (a local test), if change is made to one, it is likely that the other have to be changed as well.
// The referred test is specifically for TPMv2.0, while this test is for TPMv1.2.
// Both versions of TPM is incompatible with each other and they way we handle reboot for the 2 versions are different and thus the need for 2 versions of the same test.

func init() {
	testing.AddTest(&testing.Test{
		Func: KeysetTiedToTPM1,
		Desc: "Verifies that, for TPMv1.2 devices, the keyset is tied to TPM regardless of when it's created and if a reboot happens",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm1"},
		Attr:         []string{"informational", "group:mainline"},
	})
}

// loginTakeOwnershipAndCheckKeysetTiedToTPM is the primary workflow in this test.
// It'll reset the TPM, login and logout to create a vault. Then, it'll optionally
// reboot. After that, it'll take ownership, login again, and check that the keyset
// is tied to the TPM.
func loginTakeOwnershipAndCheckKeysetTiedToTPM(ctx context.Context, s *testing.State, utility *hwsec.UtilityCryptohomeBinary, helper *hwsecremote.HelperRemote, reboot bool) {
	// Reset TPM.
	if err := helper.EnsureTPMIsReset(ctx); err != nil {
		s.Fatal("Failed to ensure resetting TPM: ", err)
	}
	s.Log("TPM is confirmed to be reset")

	// Login+Logout.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to create user vault when testing without reboot: ", err)
	}
	defer func() {
		// Remember to logout and delete vault.
		if err := utility.UnmountAll(ctx); err != nil {
			s.Fatal("Failed to logout during first login and testing with reboot: ", err)
		}
		if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
			s.Error("Failed to cleanup after the test: ", err)
		}
	}()
	if err := utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to logout during first login and testing without reboot: ", err)
	}

	if reboot {
		// Restarts all TPM daemons to simulate a reboot.
		if err := helper.Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot: ", err)
		}
	}

	// TakeOwnership.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned when testing without reboot: ", err)
	}

	// Login again.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to login when testing without reboot: ", err)
	}

	// Check TPM wrapped.
	if err := utility.CheckTPMWrappedUserKeyset(ctx, util.FirstUsername); err != nil {
		s.Fatal("Keyset not TPM bound when testing without reboot: ", err)
	}
}

// KeysetTiedToTPM1 is an integration test that verifies a user's VKK is tied
// to the TPM after the second login.
func KeysetTiedToTPM1(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwsecremote.NewCmdRunner(s.DUT())
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}
	helper, err := hwsecremote.NewHelper(utility, cmdRunner, s.DUT())
	if err != nil {
		s.Fatal("Helper creation error: ", err)
	}

	// First we test the case without reboot, that is:
	// Reset TPM -> Login+Logout -> TakeOwnership -> Login -> Check TPM Bound.
	loginTakeOwnershipAndCheckKeysetTiedToTPM(ctx, s, utility, helper, false)

	// Next we test the case with reboot, that is:
	// Reset TPM -> Login+Logout -> Reboot -> TakeOwnership -> Login -> Check TPM Bound.
	loginTakeOwnershipAndCheckKeysetTiedToTPM(ctx, s, utility, helper, true)
}
