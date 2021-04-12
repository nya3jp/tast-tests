// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
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

// CheckWriteProtectStateCorrect correct returns an error if the FPMCU's current write
// protection state does not match the expected state.
func CheckWriteProtectStateCorrect(ctx context.Context, d *dut.DUT, fpBoard FPBoardName, curImage FWImageType, softwareWriteProtectEnabled, hardwareWriteProtectEnabled bool) error {
	output, err := flashprotectState(ctx, d)
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
