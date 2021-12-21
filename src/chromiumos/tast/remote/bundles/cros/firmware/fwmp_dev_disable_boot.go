// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strings"
	"time"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	// The duration for takeOwnershipDelay was determined experimentally. The following warning would occur without
	// this delay: "WARNING cryptohome: [bus.cc(638)] Bus::SendWithReplyAndBlock took 11465ms to process message:
	// type=method_call, path=/org/chromium/UserDataAuth, interface=org.chromium.InstallAttributesInterface,
	// member=SetFirmwareManagementParameters".
	takeOwnershipDelay = 20 * time.Second

	// Below are the expected error messages at reboot, after disabling dev mode through TPM flags.
	errExpectedDevGotNormal = "incorrect boot mode after resetting DUT: got normal; want dev"
	errFailedtoReconnect    = "failed to reconnect to DUT"
)

type expectedErr struct {
	*errors.E
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         FwmpDevDisableBoot,
		Desc:         "Verify that firmware management parameters (FWMP) can restrict developer mode",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
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

	// checkTPMNotOwned returns error if TPM is owned.
	checkTPMNotOwned := func(ctx context.Context) error {
		out, err := s.DUT().Conn().CommandContext(ctx, "cryptohome", "--action=tpm_status").Output(ssh.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "unable to read TPM status")
		}
		regex := `TPM Owned:(\s+\w+\s?)`
		expMatch := regexp.MustCompile(regex)
		matches := expMatch.FindStringSubmatch(string(out))
		if len(matches) < 2 {
			return errors.Errorf("failed to match regex %q in %q", expMatch, string(out))
		}
		tpmStatus := strings.TrimSpace(matches[1])
		if tpmStatus == "True" {
			return errors.New("got TPM Owned: true, expected it to be false")
		}
		return nil
	}

	setFWMP := func(ctx context.Context, flags string) error {
		s.Log("Taking ownership")
		if err := s.DUT().Conn().CommandContext(ctx, "cryptohome", "--action=tpm_take_ownership").Run(ssh.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to take ownership")
		}
		s.Logf("Sleeping for %s before taking ownership", takeOwnershipDelay)
		if err := testing.Sleep(ctx, takeOwnershipDelay); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
		if err := s.DUT().Conn().CommandContext(ctx, "cryptohome", "--action=tpm_wait_ownership").Run(ssh.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to wait for ownership")
		}
		s.Log("Setting firmware management parameters")
		if err := s.DUT().Conn().CommandContext(ctx, "cryptohome", "--action=set_firmware_management_parameters", "--flags=0x"+flags).Run(ssh.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to set firmware management parameters with flags 0x%s", flags)
		}
		return nil
	}

	expectBootNormal := func(ctx context.Context, ms *firmware.ModeSwitcher) error {
		if err := ms.ModeAwareReboot(ctx, firmware.ColdReset); err != nil {
			// Errors, as defined errExpectedDebGotNormal and errFailedtoReconnect, are expected
			// because DUT should not be able to boot back into dev mode.
			if strings.Contains(err.Error(), errExpectedDevGotNormal) {
				s.Log("DUT booted into normal mode")
				return &expectedErr{E: errors.New(errExpectedDevGotNormal)}
			}
			if strings.Contains(err.Error(), errFailedtoReconnect) {
				s.Log("DUT might be stuck at the 'to_norm' screen, pressing ENTER to boot")
				if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurTab); err != nil {
					return errors.Wrap(err, "failed to press ENTER to boot")
				}
				s.Log("Waiting for DUT to reconnect")
				waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
				defer cancelWaitConnect()
				if err := h.WaitConnect(waitConnectCtx); err != nil {
					return errors.Wrap(err, "failed to reconnect to DUT")
				}
				s.Log("Checking DUT has successfully booted into normal mode")
				if curr, err := h.Reporter.CurrentBootMode(ctx); err != nil {
					return errors.Wrap(err, "failed to check boot mode")
				} else if curr != fwCommon.BootModeNormal {
					return errors.Errorf("Wrong boot mode: got %q, want %q", curr, fwCommon.BootModeNormal)
				}
				return &expectedErr{E: errors.New(errFailedtoReconnect)}
			}
		}
		return nil
	}

	s.Log("Checking that TPM is not owned")
	if err := checkTPMNotOwned(ctx); err != nil {
		s.Fatal("While checking TPM status: ", err)
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

	s.Log("Expecting DUT to boot into normal mode")
	err = expectBootNormal(ctx, ms)
	if err == nil {
		s.Fatal("DUT booted into developer mode")
	}
	rebootErr, ok := err.(*expectedErr)
	if !ok {
		s.Fatal("Unexpected error occured while attempting to boot DUT into normal mode: ", rebootErr)
	}

	// Confirm TPM is not owned.
	s.Log("Checking that TPM is not owned at the end of the test")
	if err := checkTPMNotOwned(ctx); err != nil {
		s.Fatal("While checking TPM status: ", err)
	}
}
