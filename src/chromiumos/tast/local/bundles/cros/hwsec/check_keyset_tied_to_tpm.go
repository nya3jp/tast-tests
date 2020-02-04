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

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckKeysetTiedToTPM,
		Desc: "Verifies that the keyset is tied to TPM regardless of when it's created and if a reboot happens",
		Contacts: []string{
			"cros-hwsec@chromium.org",
			"zuan@chromium.org",
		},
		SoftwareDeps: []string{"tpm2"},
		Attr:         []string{"informational"},
	})
}

func CheckKeysetTiedToTPM(ctx context.Context, s *testing.State) {
	cmdRunner, err := hwseclocal.NewCmdRunner()
	if err != nil {
		s.Fatal("Failed to create CmdRunner: ", err)
	}
	utility, err := hwsec.NewUtilityCryptohomeBinary(cmdRunner)
	if err != nil {
		s.Fatal("Failed to create UtilityCryptohomeBinary: ", err)
	}
	helper, err := hwseclocal.NewHelper(utility)
	if err != nil {
		s.Fatal("Failed to create hwsec local helper: ", err)
	}

	// First we test the case without reboot, that is:
	// Reset TPM -> Login+Logout -> TakeOwnership -> Login -> Check TPM Bound.

	// Reset TPM.
	if err := hwseclocal.ResetTPMAndSystemStates(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}

	// Login+Logout.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true); err != nil {
		s.Fatal("Failed to create user vault when testing without reboot: ", err)
	}
	if err := utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to logout during first login and testing without reboot: ", err)
	}

	// TakeOwnership.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned when testing without reboot: ", err)
	}

	// Login again.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, false); err != nil {
		s.Fatal("Failed to login when testing without reboot: ", err)
	}

	// Check TPM wrapped.
	if err := utility.CheckTPMWrappedUserKeyset(ctx, util.FirstUsername); err != nil {
		s.Fatal("Keyset not TPM bound when testing without reboot: ", err)
	}

	// Remember to logout.
	if err := utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to logout during first login and testing without reboot: ", err)
	}

	// Next we test the case with reboot, that is:
	// Reset TPM -> Login+Logout -> Reboot -> TakeOwnership -> Login -> Check TPM Bound.

	// Reset TPM.
	if err := hwseclocal.ResetTPMAndSystemStates(ctx); err != nil {
		s.Fatal("Failed to reset TPM or system states: ", err)
	}
	if err = cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}

	// Login+Logout.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, true); err != nil {
		s.Fatal("Failed to create user vault when testing with reboot: ", err)
	}
	if err := utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to logout during first login and testing with reboot: ", err)
	}

	// Restarts all TPM daemons to simulate a reboot.
	if err := hwseclocal.RestartTPMDaemons(ctx); err != nil {
		s.Fatal("Failed to restart TPM-related daemons to simulate reboot: ", err)
	}
	if err := cryptohome.CheckService(ctx); err != nil {
		s.Fatal("Cryptohome D-Bus service didn't come back: ", err)
	}

	// TakeOwnership.
	if err := helper.EnsureTPMIsReady(ctx, hwsec.DefaultTakingOwnershipTimeout); err != nil {
		s.Fatal("Failed to wait for TPM to be owned when testing with reboot: ", err)
	}

	// Login again.
	if err := utility.MountVault(ctx, util.FirstUsername, util.FirstPassword, util.PasswordLabel, false); err != nil {
		s.Fatal("Failed to login when testing with reboot: ", err)
	}

	// Check TPM wrapped.
	if err := utility.CheckTPMWrappedUserKeyset(ctx, util.FirstUsername); err != nil {
		s.Fatal("Keyset not TPM bound when testing with reboot: ", err)
	}

	// Remember to logout.
	if err := utility.UnmountAll(ctx); err != nil {
		s.Fatal("Failed to logout during first login and testing with reboot: ", err)
	}

	// Cleanup.
	if _, err := utility.RemoveVault(ctx, util.FirstUsername); err != nil {
		s.Error("Failed to cleanup after the test: ", err)
	}
}
