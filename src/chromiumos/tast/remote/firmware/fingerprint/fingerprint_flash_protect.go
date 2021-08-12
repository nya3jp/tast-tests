// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
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

// FlashProtectFlags represents a set of EC flash protect flags.
type FlashProtectFlags uint64

// Individual flash protect flags.
const (
	FlashProtectRoAtBoot          FlashProtectFlags = 0x1   // RO flash code protected when the EC boots.
	FlashProtectRoNow             FlashProtectFlags = 0x2   // RO flash code protected now.  If this bit is set, at-boot status cannot be changed.
	FlashProtectAllNow            FlashProtectFlags = 0x4   // Entire flash code protected now, until reboot.
	FlashProtectGpioAsserted      FlashProtectFlags = 0x8   // Flash write protect GPIO is asserted now.
	FlashProtectErrorStuck        FlashProtectFlags = 0x10  // Error - at least one bank of flash is stuck locked, and cannot be unlocked.
	FlashProtectErrorInconsistent FlashProtectFlags = 0x20  // Error - flash protection is in inconsistent state.
	FlashProtectAllAtBoot         FlashProtectFlags = 0x40  // Entire flash code protected when the EC boots.
	FlashProtectRwAtBoot          FlashProtectFlags = 0x80  // RW flash code protected when the EC boots.
	FlashProtectRwNow             FlashProtectFlags = 0x100 // RW flash code protected now.
	FlashProtectRollbackAtBoot    FlashProtectFlags = 0x200 // Rollback information flash region protected when the EC boots.
	FlashProtectRollbackNow       FlashProtectFlags = 0x400 // Rollback information flash region protected now.
)

// IsSet checks if the given flags are set.
func (f FlashProtectFlags) IsSet(flags FlashProtectFlags) bool {
	return (f & flags) == flags
}

// UnmarshalerEctool unmarshals part of the ectool output into a ECFlashProtectFlags.
func (f *FlashProtectFlags) UnmarshalerEctool(data []byte) error {
	flagString := string(data)
	flags, err := strconv.ParseUint(flagString, 0, 32)
	if err != nil {
		return errors.Wrapf(err, "failed to convert flash protect flags (%s) to int", flagString)
	}
	*f = FlashProtectFlags(flags)
	return nil
}

// FlashProtect hold the state of flash protect from an EC.
type FlashProtect struct {
	Active    FlashProtectFlags
	Valid     FlashProtectFlags
	Writeable FlashProtectFlags
}

// IsHardwareWriteProtected is used to obtain actual hardware write protection
// state as reported by the FPMCU.
func (f *FlashProtect) IsHardwareWriteProtected() bool {
	return f.Active.IsSet(FlashProtectGpioAsserted)
}

// IsSoftwareReadOutProtected is used to obtain the RDP status.
//
// Software write protection is a bit ambiguous. We enable RDP on both
// bloonchipper and dartmonkey, which corresponds to the irremovable
// ro_at_boot flag. This flag, among other things, indicates that ro_now
// will be enabled at boot. The ro_at_boot flag does not indicate whether
// the RO is protected from on-chip flash modifications (because it isn't).
func (f *FlashProtect) IsSoftwareReadOutProtected() bool {
	return f.Active.IsSet(FlashProtectRoAtBoot)
}

// IsSoftwareWriteProtected is used to obtain the software write protect status
// of RO.
//
// When ro_now is enabled, the RO flash cannot be written to. This is the
// canonical software write protect, but the behavior of this mechanism differs
// between dartmonkey and bloonchipper.
//
// For Dartmonkey, once this flag is set on boot, it cannot be removed without
// flashing via the bootloader (flash_fp_mcu).
// For bloonchipper, this flag can be removed, but the ro_at_boot cannot be
// removed.
//
// See IsSoftwareReadOutProtected for a bit more detail.
func (f *FlashProtect) IsSoftwareWriteProtected() bool {
	return f.Active.IsSet(FlashProtectRoNow)
}

// UnmarshalerEctool unmarshals part of the ectool output into a FlashProtect.
func (f *FlashProtect) UnmarshalerEctool(data []byte) error {
	dataStr := string(data)
	rActive := regexp.MustCompile(`Flash protect flags\:\s+(0x[[:xdigit:]]+)`)
	rValid := regexp.MustCompile(`Valid flags\:\s+(0x[[:xdigit:]]+)`)
	rWriteable := regexp.MustCompile(`Writable flags\:\s+(0x[[:xdigit:]]+)`)

	var fp FlashProtect
	var flags FlashProtectFlags

	result := rActive.FindStringSubmatch(dataStr)
	if result == nil || len(result) != 2 {
		return errors.Errorf("can't find active flash protect flags in %q", dataStr)
	}
	if err := flags.UnmarshalerEctool([]byte(result[1])); err != nil {
		return errors.Wrap(err, "failed to unmarshal active flags")
	}
	fp.Active = flags

	result = rValid.FindStringSubmatch(dataStr)
	if result == nil || len(result) != 2 {
		return errors.Errorf("can't find valid flash protect flags in %q", dataStr)
	}
	if err := flags.UnmarshalerEctool([]byte(result[1])); err != nil {
		return errors.Wrap(err, "failed to unmarshal valid flags")
	}
	fp.Valid = flags

	result = rWriteable.FindStringSubmatch(dataStr)
	if result == nil || len(result) != 2 {
		return errors.Errorf("can't find writeable flash protect flags in %q", dataStr)
	}
	if err := flags.UnmarshalerEctool([]byte(result[1])); err != nil {
		return errors.Wrap(err, "failed to unmarshal writeable flags")
	}
	fp.Writeable = flags

	*f = fp
	return nil
}

// GetFlashProtect is used to obtain actual flash protection state as
// reported by the FPMCU using the 'ectool --name=cros_fp flashprotect'
// command.
func GetFlashProtect(ctx context.Context, d *dut.DUT) (FlashProtect, error) {
	var fp FlashProtect
	cmd := firmware.NewECTool(d, firmware.ECToolNameFingerprint).Command(ctx, "flashprotect")
	bytes, err := cmd.Output()
	if err != nil {
		return fp, errors.Wrap(err, "failed to get flashprotect state")
	}
	return fp, fp.UnmarshalerEctool(bytes)
}

func flashProtectState(ctx context.Context, d *dut.DUT) (string, error) {
	bytes, err := EctoolCommand(ctx, d, "flashprotect").Output()
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
		return errors.Wrapf(err, "failed to set hardware write protect to %t", enable)
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
	_ = EctoolCommand(ctx, d, "flashprotect", softwareWriteProtect).Run()
	// TODO(b/116396469): "flashprotect enable" command is slow, so wait for
	// it to complete before attempting to reboot.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return EctoolCommand(ctx, d, "version").Run()
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "failed to poll after running flashprotect")
	}
	if err := RebootFpmcu(ctx, d, ImageTypeRW); err != nil {
		return errors.Wrapf(err, "failed to set software write protect to %t", enable)
	}
	return nil
}

// CheckWriteProtectStateCorrect correct returns an error if the FPMCU's current write
// protection state does not match the expected state.
func CheckWriteProtectStateCorrect(ctx context.Context, d *dut.DUT, fpBoard FPBoardName, curImage FWImageType, softwareWriteProtectEnabled, hardwareWriteProtectEnabled bool) error {
	output, err := flashProtectState(ctx, d)
	if err != nil {
		return err
	}

	expectedOutput := expectedFlashProtectOutput(fpBoard, curImage, softwareWriteProtectEnabled, hardwareWriteProtectEnabled)

	if expectedOutput == "" {
		return errors.Errorf("invalid state, hw wp: %t, sw wp: %t", hardwareWriteProtectEnabled, softwareWriteProtectEnabled)
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

	flagsBytes, err := sysInfoFlagsCommand(ctx, d).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to get sysinfo flags")
	}

	flags := strings.TrimSpace(string(flagsBytes))
	if flags != sysInfoSystemIsLockedFlags {
		return errors.Errorf("sys info flags do not match. expected: %q, actual %q", sysInfoSystemIsLockedFlags, flags)
	}

	return nil
}
