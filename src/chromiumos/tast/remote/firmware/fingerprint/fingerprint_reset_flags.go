// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// ResetFlags represents FPMCU reset flags.
type ResetFlags uint32

// Individual reset flags. Definitions of these flags come from
// ec_commands.h file.
// https://source.chromium.org/chromiumos/chromiumos/codesearch/+/main:src/platform/ec/include/ec_commands.h
const (
	// ResetFlagNone No reset flags.
	ResetFlagNone ResetFlags = 0
	// ResetFlagOther Other known reason.
	ResetFlagOther ResetFlags = 0x1
	// ResetFlagResetPin Reset pin asserted.
	ResetFlagResetPin ResetFlags = 0x2
	// ResetFlagBrownout Reset caused by brownout.
	ResetFlagBrownout ResetFlags = 0x4
	// ResetFlagPowerOn Power-on reset.
	ResetFlagPowerOn ResetFlags = 0x8
	// ResetFlagWatchdog Watchdog timer reset.
	ResetFlagWatchdog ResetFlags = 0x10
	// ResetFlagSoft Soft reset triggered by core.
	ResetFlagSoft ResetFlags = 0x20
	// ResetFlagHibernate Wake from hibernate.
	ResetFlagHibernate ResetFlags = 0x40
	// ResetFlagRtcAlarm RTC alarm wake.
	ResetFlagRtcAlarm ResetFlags = 0x80
	// ResetFlagWakePin Wake pin triggered wake.
	ResetFlagWakePin ResetFlags = 0x100
	// ResetFlagLowBattery Low battery triggered wake.
	ResetFlagLowBattery ResetFlags = 0x200
	// ResetFlagSysjump Jumped directly to this image.
	ResetFlagSysjump ResetFlags = 0x400
	// ResetFlagHard Hard reset from software.
	ResetFlagHard ResetFlags = 0x800
	// ResetFlagApOff Do not power on AP.
	ResetFlagApOff ResetFlags = 0x1000
	// ResetFlagPreserved Some reset flags preserved from previous boot.
	ResetFlagPreserved ResetFlags = 0x2000
	// ResetFlagUsbResume USB resume triggered wake.
	ResetFlagUsbResume ResetFlags = 0x4000
	// ResetFlagRdd USB Type-C debug cable.
	ResetFlagRdd ResetFlags = 0x8000
	// ResetFlagRbox Fixed reset functionality.
	ResetFlagRbox ResetFlags = 0x10000
	// ResetFlagSecurity Security threat.
	ResetFlagSecurity ResetFlags = 0x20000
	// ResetFlagApWatchdog AP experienced a watchdog reset.
	ResetFlagApWatchdog ResetFlags = 0x40000
	// ResetFlagStayInRo Do not select RW in EFS.
	ResetFlagStayInRo ResetFlags = 0x80000
	// ResetFlagEfs Jumped to this image by EFS.
	ResetFlagEfs ResetFlags = 0x100000
	// ResetFlagApIdle Leave alone AP.
	ResetFlagApIdle ResetFlags = 0x200000
	// ResetFlagInitialPwr EC had power, then was reset.
	ResetFlagInitialPwr ResetFlags = 0x400000
)

// IsSet checks if the given flags are set.
func (f ResetFlags) IsSet(flags ResetFlags) bool {
	return (f & flags) == flags
}

// unmarshalEctoolResetFlags is used to prepare output from ectool before
// extracting reset flags from it.
func unmarshalEctoolResetFlags(str string) (uint32, error) {
	return UnmarshalEctoolFlags(strings.TrimSpace(str))
}

// GetResetFlags is used to obtain actual reset cause as reported by the FPMCU
// using the 'ectool --name=cros_fp sysinfo reset_flags' command.
func GetResetFlags(ctx context.Context, d *dut.DUT) (ResetFlags, error) {
	cmd := EctoolCommand(ctx, d, "sysinfo", "reset_flags")
	bytes, err := cmd.Output()
	if err != nil {
		return ResetFlags(ResetFlagNone), errors.Wrap(err, "failed to get FPMCU reset flags")
	}
	output := string(bytes)
	flags, err := unmarshalEctoolResetFlags(output)

	return ResetFlags(flags), err
}
