// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/rtc"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ECRTC,
		Desc: "Checks that the EC RTC on Wilco devices is readable, writable, and updates itself",
		Contacts: []string{
			"campello@chromium.org",     // Test maintainer.
			"chromeos-wilco@google.com", // Possesses some more domain-specific knowledge.
			"chromeos-kernel-test@google.com",
		},
		SoftwareDeps: []string{"wilco"},
		Timeout:      30 * time.Second,
		Attr:         []string{"group:mainline"},
	})
}

// ECRTC tests the RTC contained within the EC on Wilco devices. As a
// first check it reads the current time. Then, for a more detailed check,
// it sets the time to a dummy time, sleeps for a bit, and reads the
// time again. The RTC better have updated itself. The test attempts to
// reset the RTC back to time.Now() after failure or completion.
func ECRTC(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a few
	// seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	mainCtx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	const (
		numRTCRetries = 5
		sleepTime     = 3 * time.Second
		// There is an upstart job that continually keeps the EC RTC in sync with
		// local time. We need to disable it during the test.
		upstartJobName = "wilco_sync_ec_rtc"
	)
	// Set the RTC to a dummy time for consistency.
	startTime := time.Date(2001, time.January, 1, 12, 0, 0, 0, time.Now().Location())
	endTimeMin := startTime.Add(sleepTime).Add(-2 * time.Second)
	endTimeMax := startTime.Add(sleepTime).Add(2 * time.Second)

	wilcoECRTC := rtc.RTC{DevName: "rtc1", LocalTime: true, NoAdjfile: true}

	readECRTC := func() time.Time {
		for i := 1; ; i++ {
			t, err := wilcoECRTC.Read(mainCtx)
			if err == nil {
				return t
			}
			s.Logf("Failed to read EC RTC (trial %d/%d): %v", i, numRTCRetries, err)
			if i >= numRTCRetries {
				s.Fatal("Failed to read EC RTC: ", err)
				return time.Time{}
			}
		}
	}

	writeECRTC := func(ctx context.Context, t time.Time) {
		for i := 1; ; i++ {
			err := wilcoECRTC.Write(ctx, t)
			if err == nil {
				return
			}
			s.Logf("Failed to write EC RTC (trial %d/%d): %v", i, numRTCRetries, err)
			if i >= numRTCRetries {
				s.Fatal("Failed to write EC RTC: ", err)
				return
			}
		}
	}

	// Sanity check before we do more complicated testing.
	readECRTC()

	// Stop the upstart job that keeps the EC RTC in sync with local time.
	if err := upstart.StopJob(mainCtx, upstartJobName); err != nil {
		s.Fatal("Failed to stop sync RTC upstart job: ", err)
	}
	defer func() {
		if err := upstart.EnsureJobRunning(cleanupCtx, upstartJobName); err != nil {
			s.Fatal("Failed to restart sync RTC upstart job: ", err)
		}
	}()

	// Ensure (as best we can) that the DUT is back in it's original state after the test.
	defer func() {
		writeECRTC(cleanupCtx, time.Now())
	}()
	// Set the RTC, sleep a bit, and the RTC better have updated itself.
	writeECRTC(mainCtx, startTime)
	testing.Sleep(mainCtx, sleepTime)
	t := readECRTC()
	if t.Before(endTimeMin) || t.After(endTimeMax) {
		s.Fatalf("RTC did not update properly: got %v; want in [%v, %v]", t, endTimeMin, endTimeMax)
	}
}
