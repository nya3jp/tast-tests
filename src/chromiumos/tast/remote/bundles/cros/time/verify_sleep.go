// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package time

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/testing"
)

const (
	sleepDuration   = 300 * time.Second
	sleepIterations = 10
	timeout         = sleepDuration*sleepIterations + time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     VerifySleep,
		Desc:     "Verifies that the sleeps on DUT are as long as they should be",
		Contacts: []string{"semihalf@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Timeout:  timeout,
	})
}

func measureSleep(ctx context.Context, s *testing.State, d time.Duration) time.Duration {
	dut := s.DUT()
	sleepArg := strconv.FormatInt(int64(d.Seconds()), 10)
	cmd := dut.Conn().Command("sleep", sleepArg)

	start := time.Now()
	err := cmd.Run(ctx)
	elapsed := time.Since(start)

	if err != nil {
		s.Fatal("Running `sleep` on DUT failed: ", err)
	}

	return elapsed
}

func runOnce(ctx context.Context, s *testing.State, d time.Duration) {
	measured := measureSleep(ctx, s, d)

	// measuredMs smaller than requested always implies an error,
	// network delays can't be negative
	if measured < d {
		s.Fatalf("Measured %vms", measured.Milliseconds())
	} else {
		s.Logf("[OK] Measured %vms", measured.Milliseconds())
	}
}

func VerifySleep(ctx context.Context, s *testing.State) {
	s.Logf("Sleeping for %vms %v times", sleepDuration.Milliseconds(), sleepIterations)
	for k := 0; k < sleepIterations; k++ {
		runOnce(ctx, s, sleepDuration)
	}
}
