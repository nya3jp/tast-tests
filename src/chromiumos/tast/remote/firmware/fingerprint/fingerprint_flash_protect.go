// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const (
	// TODO(b/149590275): remove once fixed
	flashprotectOutputHardwareAndSoftwareWriteProtectEnabledBloonchipper = `Flash protect flags: 0x0000040f wp_gpio_asserted ro_at_boot ro_now rollback_now all_now
Valid flags:         0x0000003f wp_gpio_asserted ro_at_boot ro_now all_now STUCK INCONSISTENT
Writable flags:      0x00000000
`
	flashprotectOutputHardwareAndSoftwareWriteProtectEnabled = `Flash protect flags: 0x0000000b wp_gpio_asserted ro_at_boot ro_now
Valid flags:         0x0000003f wp_gpio_asserted ro_at_boot ro_now all_now STUCK INCONSISTENT
Writable flags:      0x00000004 all_now
`

	// TODO(b/149590275): remove once fixed
	flashprotectOutputHardwareWriteProtectDisabledAndSoftwareWriteProtectEnabledBloonchipper = `Flash protect flags: 0x00000407 ro_at_boot ro_now rollback_now all_now
Valid flags:         0x0000003f wp_gpio_asserted ro_at_boot ro_now all_now STUCK INCONSISTENT
Writable flags:      0x00000000
`

	flashprotectOutputHardwareWriteProtectDisabledAndSoftwareWriteProtectEnabled = `Flash protect flags: 0x00000003 ro_at_boot ro_now
Valid flags:         0x0000003f wp_gpio_asserted ro_at_boot ro_now all_now STUCK INCONSISTENT
Writable flags:      0x00000000
`

	// TODO(b/149590275): remove once fixed
	flashprotectOutputHardwareAndSoftwareWriteProtectEnabledROBloonchipper = `Flash protect flags: 0x0000000b wp_gpio_asserted ro_at_boot ro_now
Valid flags:         0x0000003f wp_gpio_asserted ro_at_boot ro_now all_now STUCK INCONSISTENT
Writable flags:      0x00000004 all_now
`

	flashprotectOutputHardwareAndSoftwareWriteProtectDisabled = `Flash protect flags: 0x00000000
Valid flags:         0x0000003f wp_gpio_asserted ro_at_boot ro_now all_now STUCK INCONSISTENT
Writable flags:      0x00000001 ro_at_boot
`
)

func flashprotectState(ctx context.Context, d *dut.DUT) (string, error) {
	bytes, err := EctoolCommand(ctx, d, "flashprotect").Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get flashprotect state")
	}
	return string(bytes), nil
}

func expectedFlashProtectOutput(fpBoard FPBoardName, curImage FWImageType, softwareWriteProtectEnabled, hardwareWriteProtectEnabled bool) string {
	expectedOutput := ""

	switch {
	case softwareWriteProtectEnabled && hardwareWriteProtectEnabled:
		// TODO(b/149590275): remove once fixed
		if fpBoard == FPBoardNameBloonchipper {
			if curImage == ImageTypeRO {
				expectedOutput = flashprotectOutputHardwareAndSoftwareWriteProtectEnabledROBloonchipper
			} else {
				expectedOutput = flashprotectOutputHardwareAndSoftwareWriteProtectEnabledBloonchipper
			}
		} else {
			expectedOutput = flashprotectOutputHardwareAndSoftwareWriteProtectEnabled
		}
	case softwareWriteProtectEnabled && !hardwareWriteProtectEnabled:
		// TODO(b/149590275): remove once fixed
		if fpBoard == FPBoardNameBloonchipper {
			expectedOutput = flashprotectOutputHardwareWriteProtectDisabledAndSoftwareWriteProtectEnabledBloonchipper
		} else {
			expectedOutput = flashprotectOutputHardwareWriteProtectDisabledAndSoftwareWriteProtectEnabled
		}
	case !softwareWriteProtectEnabled && !hardwareWriteProtectEnabled:
		expectedOutput = flashprotectOutputHardwareAndSoftwareWriteProtectDisabled
	}

	return expectedOutput
}

// SetHardwareWriteProtect sets the FPMCU's hardware write protection to the
// state specified by enable.
func SetHardwareWriteProtect(ctx context.Context, pxy *servo.Proxy, enable bool) error {
	hardwareWriteProtectState := servo.FWWPStateOff

	if enable {
		hardwareWriteProtectState = servo.FWWPStateOn
	}

	if err := pxy.Servo().SetFWWPState(ctx, hardwareWriteProtectState); err != nil {
		return errors.Wrapf(err, "failed to set hardware write protect to %q", enable)
	}

	return nil
}

// SetSoftwareWriteProtect sets the FPMCU's software write protection to the
// state specified by enable.
func SetSoftwareWriteProtect(ctx context.Context, d *dut.DUT, enable bool) error {
	softwareWriteProtect := "disable"
	if enable {
		softwareWriteProtect = "enable"
	}
	// TODO(b/116396469): Add error checking once it's fixed.
	// This command can return error even on success, so ignore error for now.
	_ = EctoolCommand(ctx, d, "flashprotect", softwareWriteProtect).Run(ctx)
	// TODO(b/116396469): "flashprotect enable" command is slow, so wait for
	// it to complete before attempting to reboot.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return EctoolCommand(ctx, d, "version").Run(ctx)
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "failed to poll after running flashprotect")
	}
	if err := RebootFpmcu(ctx, d, ImageTypeRW); err != nil {
		return errors.Wrapf(err, "failed to set software write protect to %q", enable)
	}
	return nil
}

// CheckWriteProtectStateCorrect correct returns an error if the FPMCU's current write
// protection state does not match the expected state.
func CheckWriteProtectStateCorrect(ctx context.Context, d *dut.DUT, fpBoard FPBoardName, curImage FWImageType, softwareWriteProtectEnabled, hardwareWriteProtectEnabled bool) error {
	output, err := flashprotectState(ctx, d)
	if err != nil {
		return err
	}

	expectedOutput := expectedFlashProtectOutput(fpBoard, curImage, softwareWriteProtectEnabled, hardwareWriteProtectEnabled)

	if expectedOutput == "" {
		return errors.Errorf("invalid state, hw wp: %q, sw wp: %q", hardwareWriteProtectEnabled, softwareWriteProtectEnabled)
	}

	if expectedOutput != output {
		return errors.Errorf("incorrect write protect state, expected: %q, actual: %q", expectedOutput, output)
	}

	return nil
}

func sysInfoFlagsCommand(ctx context.Context, d *dut.DUT) *ssh.Cmd {
	return EctoolCommand(ctx, d, "sysinfo", "flags")
}

// CheckSystemIsLocked validates that the FPMCU is locked and returns an error if it is not.
func CheckSystemIsLocked(ctx context.Context, d *dut.DUT) error {
	// SYSTEM_IS_LOCKED
	// SYSTEM_JUMP_ENABLED
	// SYSTEM_JUMPED_TO_CURRENT_IMAGE
	// See https://chromium.googlesource.com/chromiumos/platform/ec/+/10fe09bf9aaf59213d141fc1d479ed259f786049/include/ec_commands.h#1865
	const sysInfoSystemIsLockedFlags = "0x0000000d"

	flagsBytes, err := sysInfoFlagsCommand(ctx, d).Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to get sysinfo flags")
	}

	flags := strings.TrimSpace(string(flagsBytes))
	if flags != sysInfoSystemIsLockedFlags {
		return errors.Errorf("sys info flags do not match. expected: %q, actual %q", sysInfoSystemIsLockedFlags, flags)
	}

	return nil
}
