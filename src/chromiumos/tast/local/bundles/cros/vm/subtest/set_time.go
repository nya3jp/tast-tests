// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"chromiumos/tast/errors"
	exec "chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// Returns the current wall clock time (since epoch) as reported by `date` in the container.
func getTime(ctx context.Context, s *testing.State, cont *vm.Container) (time.Duration, error) {
	cmd := cont.Command(ctx, "date", "+%s")
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx)
		return 0, err
	}
	outStr := strings.TrimSpace(string(out))
	dur, err := time.ParseDuration(outStr + "s")
	if err != nil {
		return 0, errors.Wrapf(err, "bad seconds: %q", outStr)
	}
	return dur, nil
}

// SyncTime manually sets the time in the guest to an incorrect value,
// uses "SyncTimes" to correct it, and verifies that it is correct.
func SyncTime(ctx context.Context, s *testing.State, cont *vm.Container) {
	s.Log(ctx, "Executing SyncTime test")
	// Set the time with maitred_client.
	cmd := exec.CommandContext(ctx, "maitred_client", fmt.Sprintf("--cid=%d", cont.VM.Cid), "--port=8888", "--set_time_sec=10000")
	if err := cmd.Run(); err != nil {
		s.Error("failed to set past time: ", err)
		cmd.DumpLog(ctx)
		return
	}

	// Verify that the time was set correctly.
	vmTime, err := getTime(ctx, s, cont)
	if err != nil {
		s.Error("failed to get time: ", err)
		return
	}
	secs := vmTime.Seconds()
	if diff := 10000 - secs; math.Abs(diff) > 60 {
		s.Errorf("maitred failed to set time; actual %d, should be 10000", secs)
	}

	if err = cont.VM.Concierge.SyncTimes(ctx); err != nil {
		s.Error("calling syncTimes failed: ", err)
		return
	}

	vmTime, err = getTime(ctx, s, cont)
	if err != nil {
		s.Error("failed to get time: ", err)
		return
	}
	secs = vmTime.Seconds()
	actualTime := float64(time.Now().Unix())
	if diff := actualTime - secs; math.Abs(diff) > 60 {
		s.Errorf("failed to correct time; is %d, should be %d", secs, actualTime)
	}
}
