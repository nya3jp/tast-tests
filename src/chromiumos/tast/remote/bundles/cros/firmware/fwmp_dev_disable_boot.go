// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FwmpDevDisableBoot,
		Desc:         "Verify that firmware management parameters (FWMP) can restrict developer mode",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.DevMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.Vboot2()),
	})
}

func FwmpDevDisableBoot(ctx context.Context, s *testing.State) {
	v := s.FixtValue().(*fixture.Value)
	h := v.Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	setFWMP := func(ctx context.Context, flags string) error {
		s.Log("Taking ownership")
		if err := s.DUT().Conn().CommandContext(ctx, "tpm_manager_client", "take_ownership").Run(ssh.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to take ownership")
		}
		s.Log("Setting firmware management parameters")
		if err := s.DUT().Conn().CommandContext(ctx, "cryptohome", "--action=set_firmware_management_parameters", "--flags=0x"+flags).Run(ssh.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to set firmware management parameters with flags 0x%s", flags)
		}
		return nil
	}

	// Set DUT in "dev mode disable" state by setting TPM flags to "0x1".
	if err := setFWMP(ctx, "1"); err != nil {
		s.Fatal("Failed while taking ownership and setting flags: ", err)
	}
	defer func() {
		s.Log("Reverting the 'dev mode disable' state on DUT at the end of test")
		if err := setFWMP(ctx, "0"); err != nil {
			s.Fatal("Failed while taking ownership and setting flags at the end of test: ")
		}
	}()

	ms, err := firmware.NewModeSwitcher(ctx, h)
	if err != nil {
		s.Fatal("Failed to create mode switcher: ", err)
	}

	ownershipData, err := s.DUT().Conn().CommandContext(ctx, "hwsec-ownership-id", "id").Output(ssh.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get ownership ID: ", err)
	}
	ownershipID := strings.TrimSpace(string(ownershipData))

	// When dev mode is disabled by FWMP, DUT is expected to boot into normal mode.
	var opts []firmware.ModeSwitchOption
	opts = append(opts, firmware.SkipModeCheckAfterReboot, firmware.PressEnterAtToNorm)
	if err := ms.ModeAwareReboot(ctx, firmware.ColdReset, opts...); err != nil {
		s.Fatal("Unexpected error occurred while attempting to boot DUT: ", err)
	}

	// Confirm TPM ownership changed.
	s.Log("Checking that TPM ownership changed at the end of the test")
	if err = s.DUT().Conn().CommandContext(ctx, "hwsec-ownership-id", "diff", "--id="+ownershipID).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("While checking TPM ownership: ", err)
	}
}
