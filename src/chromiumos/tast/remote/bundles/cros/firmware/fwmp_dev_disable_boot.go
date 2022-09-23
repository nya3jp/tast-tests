// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     FwmpDevDisableBoot,
		Desc:     "Verify that firmware management parameters (FWMP) can restrict developer mode",
		Contacts: []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		// TODO(b/235742217): This test might be leaving broken DUTS that can't be auto-repaired. Add attr firmware_unstable when fixed.
		Attr:         []string{"group:firmware"},
		Timeout:      15 * time.Minute,
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

		/*
			Allow some delay to ensure that this command has fully propagated.
			Without this delay, the following error would likely appear when
			setting firmware management parameters:
			'Failed to call SetFirmwareManagementParameters: (dbus, org.freedesktop.DBus.Error.ServiceUnknown,
			Error calling D-Bus method: org.chromium.InstallAttributesInterface.SetFirmwareManagementParameters:
			The name org.chromium.UserDataAuth was not provided by any .service files)'
		*/
		s.Logf("Sleeping for %s", 5*time.Second)
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}

		// Set FWMP flags in a poll to increase the chances of success.
		s.Log("Setting firmware management parameters")
		reOwnerPassword := regexp.MustCompile(`flags:\s*(0|1)`)
		var currentFlagVal [][]byte
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := s.DUT().Conn().CommandContext(ctx, "cryptohome", "--action=set_firmware_management_parameters", "--flags=0x"+flags).Run(ssh.DumpLogOnError); err != nil {
				return errors.Wrapf(err, "failed to set firmware management parameters with flags 0x%s", flags)
			}

			// Verify the flags have been set as expected.
			out, err := s.DUT().Conn().CommandContext(ctx, "cryptohome", "--action=get_firmware_management_parameters").Output(ssh.DumpLogOnError)
			if err != nil {
				return errors.Wrap(err, "failed to get firmware management parameter flags")
			}

			currentFlagVal = reOwnerPassword.FindSubmatch(out)
			if string(currentFlagVal[1]) != flags {
				return errors.Wrapf(err, "flags haven't been set correctly: expected flags to be %q but got the following output: %s", flags, string(out))
			}

			return nil
		}, &testing.PollOptions{Timeout: 25 * time.Second, Interval: 3 * time.Second}); err != nil {
			if string(currentFlagVal[1]) == "1" {
				return errors.Wrap(err, "dev mode is disabled by FWMP flags=0x1, please run 'cryptohome --action=set_firmware_management_parameters --flags=0x0' to recover DUTs")
			}
			return err
		}

		return nil
	}

	// Verify that TPM owner password is present.
	out, err := s.DUT().Conn().CommandContext(ctx, "tpm_manager_client", "status", "--nonsensitive").Output(ssh.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get TMP status: ", err)
	}
	reOwnerPassword := regexp.MustCompile(`is_owner_password_present: (\S*)`)
	ownerPasswordState := reOwnerPassword.FindSubmatch(out)
	if string(ownerPasswordState[1]) != "true" {
		s.Fatal("TPM owner password is not present, and received output: ", string(out))
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	// Set DUT in "dev mode enable" state by setting TPM flags to "0x0" at the end of the test.
	defer func(cleanupCtx context.Context) {
		s.Log("Reverting the 'dev mode disable' state on DUT at the end of test")
		if err := setFWMP(cleanupCtx, "0"); err != nil {
			s.Fatal("Failed while taking ownership and setting flags at the end of test: ", err)
		}
	}(cleanupCtx)

	// Set DUT in "dev mode disable" state by setting TPM flags to "0x1".
	if err := setFWMP(ctx, "1"); err != nil {
		s.Fatal("Failed while taking ownership and setting flags: ", err)
	}

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
	rebootCtx, cancelRebootCtx := context.WithTimeout(ctx, 10*time.Minute)
	defer cancelRebootCtx()

	var opts []firmware.ModeSwitchOption
	opts = append(opts, firmware.SkipModeCheckAfterReboot, firmware.UseFwScreenToDevMode)
	if err := ms.ModeAwareReboot(rebootCtx, firmware.ColdReset, opts...); err != nil {
		s.Fatal("Unexpected error occurred while attempting to boot DUT: ", err)
	}

	// For debugging purposes, log current boot mode after the reboot.
	mode, err := h.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Fatal("Failed to check boot mode: ", err)
	}
	s.Logf("Boot mode after reboot: %s", mode)

	// Confirm TPM ownership changed.
	s.Log("Checking that TPM ownership changed at the end of the test")
	if err = s.DUT().Conn().CommandContext(ctx, "hwsec-ownership-id", "diff", "--id="+ownershipID).Run(ssh.DumpLogOnError); err != nil {
		s.Fatal("While checking TPM ownership: ", err)
	}
}
