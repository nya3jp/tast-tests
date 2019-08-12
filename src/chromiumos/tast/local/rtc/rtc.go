// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package rtc provides access to a device's Real Time Clock (hardware clock)
package rtc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
)

const (
	// The hwclock command uses the "dd mmm yyyy HH:MM" format for the --date arg,
	// so this is the corresponding format string for time.Format().
	hwclockDateFormat = "02 Jan 2006 15:04"

	// Then, to be difficult, the hwclock commantd uses a different format
	// for the output when reading the time. An example output
	// from reading hwclock is  "2019-08-23 12:48:45.846380-06:00"
	hwclockOutputFormat = "2006-01-02 15:04:05.000000Z07:00"
	hwclockTimeout      = 3 * time.Second
)

// RTC represents a real time clock that is accessed with the "hwclock" command.
type RTC struct {
	// DevName indicates which RTC to use. For example DevName="rtc1" means that /dev/rtc1 will be used.
	DevName string
	// LocalTime indicates whether the "--localtime" flag should be set with the hwclock command.
	LocalTime bool
	// NoAdjfile indicates whether the "--noadjfile" flag should be set with the hwclock command.
	NoAdjfile bool
}

// Write sets the RTC using the "hwclock" command. This only changes
// the external hardware clock, it does not change the OS/system time.
// It uses a 3 second timeout on top of the given context.
func (rtc RTC) Write(ctx context.Context, t time.Time) error {
	args := rtc.hwclockArgs()
	args = append(args, "--set")
	// hwclock likes "JAN", but t.Format give "Jan" for the month.
	dateString := strings.ToUpper(t.Format(hwclockDateFormat))
	args = append(args, "--date="+dateString)

	ctx, cancel := context.WithTimeout(ctx, hwclockTimeout)
	defer cancel()
	return testexec.CommandContext(ctx, "hwclock", args...).Run(testexec.DumpLogOnError)
}

// Read reads the RTC using the "hwclock" command.
// It uses a 3 second timeout on top of the given context.
func (rtc RTC) Read(ctx context.Context) (time.Time, error) {
	args := rtc.hwclockArgs()
	args = append(args, "--get")

	ctx, cancel := context.WithTimeout(ctx, hwclockTimeout)
	defer cancel()
	bytes, err := testexec.CommandContext(ctx, "hwclock", args...).CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(hwclockOutputFormat, strings.TrimSpace(string(bytes)))
}

func (rtc RTC) hwclockArgs() []string {
	args := []string{"--rtc=/dev/" + rtc.DevName}
	if rtc.LocalTime {
		args = append(args, "--localtime")
	}
	if rtc.NoAdjfile {
		args = append(args, "--noadjfile")
	}
	return args
}
