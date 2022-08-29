// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
)

// ResetFlags represents FPMCU reset flags.
type ResetFlags uint64

// Individual reset flags. Definitions of these flags come from
// ec_commands.h file.
const (
	// Other known reason.
	ResetFlagOther ResetFlags = 0x1
	// Reset pin asserted.
	ResetFlagResetPin ResetFlags = 0x2
	// Brownout.
	ResetFlagBrownout ResetFlags = 0x3
	// Power-on reset.
	ResetFlagPowerOn ResetFlags = 0x8
	// Watchodg timer reset.
	ResetFlagWatchdog ResetFlags = 0x10
	// Soft reset triggered by core.
	ResetFlagSoft ResetFlags = 0x20
	// Wake from hibernate.
	ResetFlagHibernate ResetFlags = 0x40
	// RTC alarm wake.
	ResetFlagRtcAlarm ResetFlags = 0x80
	// Wake pin triggered wake.
	ResetFlagWakePin ResetFlags = 0x100
	// Low battery triggered wake.
	ResetFlagLowBattery ResetFlags = 0x200
	// Jumped directly to this image.
	ResetFlagSysjump ResetFlags = 0x400
	// Hard reset from software.
	ResetFlagHard ResetFlags = 0x800
	// Do not power on AP.
	ResetFlagApOff ResetFlags = 0x1000
	// Some reset flags preserved from previous boot.
	ResetFlagPreserved ResetFlags = 0x2000
	// USB resume triggered wake.
	ResetFlagUsbResume ResetFlags = 0x4000
	// USB Type-C debug cable.
	ResetFlagRdd ResetFlags = 0x8000
	// Fixed reset functionality.
	ResetFlagRbox ResetFlags = 0x10000
	// Security threat.
	ResetFlagSecurity ResetFlags = 0x20000
	// AP experienced a watchdog reset.
	ResetFlagApWatchdog ResetFlags = 0x40000
	// Do not select RW in EFS.
	ResetFlagStayInRo ResetFlags = 0x80000
	// Jumped to this image by EFS.
	ResetFlagEfs ResetFlags = 0x100000
	// Leave alone AP.
	ResetFlagApIdle ResetFlags = 0x200000
	// EC had power, then was reset.
	ResetFlagInitialPwr ResetFlags = 0x400000
)

// IsSet checks if the given flags are set.
func (f ResetFlags) IsSet(flags ResetFlags) bool {
	return (f & flags) == flags
}

// unmarshalEctoolFlags unmarshals part of the ectool output into a ResetFlags.
func unmarshalEctoolFlags(data string) (ResetFlags, error) {
	flags, err := strconv.ParseUint(data, 0, 32)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to convert encryption status flags (%s) to int", data)
	}
	return ResetFlags(flags), nil
}

// GetResetFlags is used to obtain actual reset cause as reported by the FPMCU
// using the 'ectool --name=cros_fp sysinfo reset_flags' command.
func GetResetFlags(ctx context.Context, d *dut.DUT) (ResetFlags, error) {
	cmd := firmware.NewECTool(d, firmware.ECToolNameFingerprint).Command(ctx, "sysinfo", "reset_flags")
	bytes, err := cmd.Output()
	if err != nil {
		return ResetFlags(0), errors.Wrap(err, "failed to get FPMCU reset flags")
	}
	output := string(bytes)
	return unmarshalEctoolFlags(output)
}
