// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/bundles/cros/hwsec/util"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

// NOTE: This test is largely similar to hwsec.KeysetTiedToTPM1 (a remote test), if change is made to one, it is likely that the other have to be changed as well.
// The referred test is specifically for TPMv1.2, while this test is for TPMv2.0.
// Both versions of TPM is incompatible with each other and they way we handle reboot for the 2 versions are different and thus the need for 2 versions of the same test.

func init() {
	testing.AddTest(&testing.Test{
		Func: KeysetTiedToTPM2,
		Desc: "Verifies that for TPMv2.0 devices, the keyset is tied to TPM regardless of when it's created and if a reboot happens",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"group:mainline"},
	})
}

// loginTakeOwnershipAndCheckKeysetTiedToTPM is the primary workflow in this test.
// It'll reset the TPM, login and logout to create a vault. Then, it'll optionally
// reboot. After that, it'll take ownership, login again, and check that the keyset
// is tied to the TPM.
func loginTakeOwnershipAndCheckKeysetTiedToTPM(ctx context.Context, s *testing.State, utility *hwsec.CryptohomeClient, helper *hwseclocal.CmdHelperLocal, reboot bool) {
	// Reset TPM.
	if err := helper.EnsureTPMAndSystemStateAreReset(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}

	// Login+Logout.
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), true, hwsec.NewVaultConfig()); err != nil {
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
		if err := helper.DaemonController().RestartTPMDaemons(ctx); err != nil {
			s.Fatal("Failed to restart TPM-related daemons to simulate reboot: ", err)
		}
		if err := cryptohome.CheckService(ctx); err != nil {
			s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
		}
	}

	// TakeOwnership.
	if err := helper.EnsureTPMIsReadyAndBackupSecrets(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned when testing without reboot: ", err)
	}

	// Login again.
	if err := utility.MountVault(ctx, util.PasswordLabel, hwsec.NewPassAuthConfig(util.FirstUsername, util.FirstPassword), false, hwsec.NewVaultConfig()); err != nil {
		s.Fatal("Failed to login when testing without reboot: ", err)
	}

	// Check TPM wrapped.
	if err := utility.CheckTPMWrappedUserKeyset(ctx, util.FirstUsername); err != nil {
		s.Fatal("Keyset not TPM bound when testing without reboot: ", err)
	}
}

// KeysetTiedToTPM2 is an integration test that verifies a user's VKK is tied
// to the TPM after the second login.
func KeysetTiedToTPM2(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewCmdRunner()
	helper, err := hwseclocal.NewHelper(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}
	utility := helper.CryptohomeClient()

	// First we test the case without reboot, that is:
	// Reset TPM -> Login+Logout -> TakeOwnership -> Login -> Check TPM Bound.
	loginTakeOwnershipAndCheckKeysetTiedToTPM(ctx, s, utility, helper, false)

	// Next we test the case with reboot, that is:
	// Reset TPM -> Login+Logout -> Reboot -> TakeOwnership -> Login -> Check TPM Bound.
	loginTakeOwnershipAndCheckKeysetTiedToTPM(ctx, s, utility, helper, true)
}
