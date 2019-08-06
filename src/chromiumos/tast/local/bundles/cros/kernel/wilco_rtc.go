// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/rtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WilcoRTC,
		Desc:         "Checks that the EC RTC works on Wilco devices",
		Contacts:     []string{"ncrews@chromium.org"}, // test author
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"wilco"},
		Timeout:      30 * time.Second,
	})
}

// WilcoRTC tests the RTC contained within the EC on Wilco devices. As a
// first check it reads the current time. Then, for a more detail check,
// it sets the time to a dummy time, sleeps for a bit, and reads the
// time again. The RTC better have updated itself. The test attempts to
// reset the RTC back to time.Now() after failure or completion.
func WilcoRTC(fullCtx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()

	const (
		// All of the following are assumed to be in local time.
		timeFormat = "02 Jan 2006 15:04:05"
		startTime  = "01 Jan 2001 1:00:00"
		endTimeMin = "01 Jan 2001 1:00:01"
		endTimeMax = "01 Jan 2001 1:00:05"
	)
	sleepTime := 3 * time.Second
	RTC := rtc.RTC{DevName: "rtc1", LocalTime: true, NoAdjfile: true}

	readTime := func() time.Time {
		t, err := RTC.Read(ctx)
		if err != nil {
			s.Fatal("Failed to read EC RTC: ", err)
		}
		return t
	}

	writeTime := func(t time.Time) {
		if err := RTC.Write(ctx, t); err != nil {
			s.Fatalf("Failed to set EC RTC to %v: %v", t, err)
		}
	}

	// Parse a time string to a local time.
	parseTime := func(str string) time.Time {
		t, err := time.Parse(timeFormat, str)
		if err != nil {
			s.Fatalf("Failed to parse time %v as %v: %v", startTime, timeFormat, err)
		}
		return t.In(time.Now().Location())
	}

	// Ensure (as best we can) that the DUT is back in it's original state after the test.
	defer func() {
		writeTime(time.Now())
	}()

	// Set the RTC, sleep a bit, and the RTC better have updated itself
	writeTime(parseTime(startTime))
	testing.Sleep(ctx, sleepTime)
	t := readTime()
	if t.Before(parseTime(endTimeMin)) || t.After(parseTime(endTimeMax)) {
		s.Fatalf("after waiting, RTC reports time as %v, should be in range [%v, %v]", t, parseTime(endTimeMin), parseTime(endTimeMax))
	}

}
