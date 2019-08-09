// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"time"

	"chromiumos/tast/local/wilco"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WilcoECRTC,
		Desc:         "Checks that the EC RTC works on Wilco devices",
		Contacts:     []string{"ncrews@chromium.org"}, // test author
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"wilco"},
		Timeout:      15 * time.Second,
	})
}

// WilcoECRTC tests the RTC contained within the EC on Wilco devices. As a
// first check it reads the current time. Then, for a more detailed check,
// it sets the time to a dummy time, sleeps for a bit, and reads the
// time again. The RTC better have updated itself. The test attempts to
// reset the RTC back to time.Now() after failure or completion.
func WilcoECRTC(ctx context.Context, s *testing.State) {
	const (
		// All of the following are assumed to be in local time.
		timeFormat = "02 Jan 2006 15:04:05"
		startTime  = "01 Jan 2001 1:00:00"
		endTimeMin = "01 Jan 2001 1:00:01"
		endTimeMax = "01 Jan 2001 1:00:05"
		sleepTime  = 3 * time.Second
	)

	// Parse a time string to a local time.
	parseTime := func(str string) time.Time {
		t, err := time.Parse(timeFormat, str)
		if err != nil {
			s.Fatalf("Failed to parse time %v as %v: %v", startTime, timeFormat, err)
		}
		return t.In(time.Now().Location())
	}

	wilco.ReadECRTC(ctx, s)

	// Ensure (as best we can) that the DUT is back in it's original state after the test.
	defer func() {
		wilco.WriteECRTC(ctx, s, time.Now())
	}()

	// Stop the upstart job that keeps the EC RTC in sync with local time.
	wilco.StopSyncRTCJob(ctx, s)
	defer func() {
		wilco.StartSyncRTCJob(ctx, s)
	}()

	// Set the RTC, sleep a bit, and the RTC better have updated itself.
	wilco.WriteECRTC(ctx, s, parseTime(startTime))
	testing.Sleep(ctx, sleepTime)
	t := wilco.ReadECRTC(ctx, s)
	if t.Before(parseTime(endTimeMin)) || t.After(parseTime(endTimeMax)) {
		s.Fatalf("RTC did no update properly: RTC reports time as %v, should be in range [%v, %v]", t, parseTime(endTimeMin), parseTime(endTimeMax))
	}
}
