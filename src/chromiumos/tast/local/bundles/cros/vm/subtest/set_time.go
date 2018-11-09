// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	exec "chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func getTime(ctx context.Context, s *testing.State, cont *vm.Container) (int64, error) {
	cmd := cont.Command(ctx, "date", "+%s")
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx)
		return 0, err
	}
	outStr := strings.Trim(string(out[:]), "\r\n")
	secs, err := strconv.ParseInt(outStr, 10, 64)
	if err != nil {
		s.Errorf("Invalid date output: %s: %v", outStr, err)
		return 0, err
	}
	return secs, nil
}

// SyncTime manually sets the time in the guest to an incorrect value,
// uses "SyncTimes" to correct it, and verifies that it is correct.
func SyncTime(ctx context.Context, s *testing.State, cont *vm.Container) {
	s.Log(ctx, "Executing SyncTime test")
	// Set the time with maitred_client.
	cmd := exec.CommandContext(ctx, "maitred_client", fmt.Sprintf("--cid=%d", cont.VM.Cid), "--port=8888", "--set_time_sec=10000")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Error(err)
		//s.Error(string(out[:]))
		return
	}

	// Verify that the time was set correctly.
	secs, err := getTime(ctx, s, cont)
	if err != nil {
		s.Error(err)
		return
	}
	if secs < 10000 || secs > 10001 {
		s.Errorf("maitred failed to set time; actual %d, should be 10000", secs)
	}

	err = cont.VM.Concierge.SyncTimes(ctx)
	if err != nil {
		s.Error(err)
		return
	}

	secs, err = getTime(ctx, s, cont)
	if err != nil {
		return
	}
	actualTime := time.Now().Unix()
	if actualTime-secs > 1 || secs-actualTime > 1 {
		s.Errorf("Failed to correct time; is %d, should be %d", secs, actualTime)
	}
}
