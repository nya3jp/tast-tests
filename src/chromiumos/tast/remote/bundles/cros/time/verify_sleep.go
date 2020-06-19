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

const timeout time.Duration = time.Hour

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifySleep,
		Desc:         "Verifies that the sleeps on DUT are as long as they should be",
		Contacts:     []string{"kek@semihalf.corp-partner.google.com"},
		SoftwareDeps: []string{},
		Attr:         []string{},
		Timeout:      timeout,
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func measureSleepMs(ctx context.Context, s *testing.State, seconds int) int {
	dut := s.DUT()
	sleepArg := strconv.FormatInt(int64(seconds), 10)
	cmd := dut.Conn().Command("sleep", sleepArg)

	start := time.Now()
	err := cmd.Run(ctx)
	elapsed := time.Since(start)

	if err != nil {
		s.Fatal("running `sleep` on DUT failed: ", err)
	}

	return int(elapsed / time.Millisecond)
}

func runOnce(ctx context.Context, s *testing.State, seconds int) {
	measuredMs := measureSleepMs(ctx, s, seconds)

	// measuredMs smaller than requested always implies an error,
	// network delays can't be negative
	if measuredMs < seconds*1000 {
		s.Fatalf("measured %vms", measuredMs)
	} else {
		s.Logf("[OK] measured %vms", measuredMs)
	}
}

func VerifySleep(ctx context.Context, s *testing.State) {
	secs := 300
	times := 10

	if time.Duration(secs*times)*time.Second > timeout {
		s.Fatalf("requested sleep (%v*%vs) is longer than the timeout (%v)", times, secs, timeout)
	}

	s.Logf("sleeping for %vms %v times", secs*1000, times)
	for k := 0; k < times; k++ {
		runOnce(ctx, s, secs)
	}
}
