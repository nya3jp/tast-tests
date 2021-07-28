// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/rtc"
	"chromiumos/tast/local/upstart"
)

// RTC (wilco) is a thin wrapper around the rtc.RTC and implements the RTC features specific to the wilco devices.
type RTC struct {
	rtc.RTC
}

// Job "wilco_sync_ec_rtc" continuously keeps the EC RTC in sync with local time (in wilco devices).
const upstartJobName = "wilco_sync_ec_rtc"

// MockECRTC mocks RTC time to the specified time by stopping necessary wilco specific job and returns a callback
// to revert the state back to original.
func (wrtc RTC) MockECRTC(ctx context.Context, t time.Time) (func(context.Context) error, error) {
	// Stop the upstart job that keeps the wilco EC RTC in sync with local time.
	if err := upstart.StopJob(ctx, upstartJobName); err != nil {
		return nil, err
	}
	if err := wrtc.Write(ctx, t); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		if err := upstart.EnsureJobRunning(ctx, upstartJobName); err != nil {
			return err
		}
		args := wrtc.HwclockArgs()
		args = append([]string{"--systohc"}, args...)
		ctx, cancel := context.WithTimeout(ctx, rtc.HwclockTimeout)
		defer cancel()
		return testexec.CommandContext(ctx, "hwclock", args...).Run(testexec.DumpLogOnError)
	}, nil
}
