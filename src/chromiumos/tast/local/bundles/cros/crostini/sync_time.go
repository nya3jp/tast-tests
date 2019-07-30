// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SyncTime,
		Desc:         "Manually sets the time in the guest to an incorrect value, uses 'SyncTimes' to correct it, and verifies that it is correct",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

// getTime returns the current wall clock time as reported by `date` in the container.
func getTime(ctx context.Context, s *testing.State, cont *vm.Container) (time.Time, error) {
	cmd := cont.Command(ctx, "date", "+%s")
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx)
		return time.Time{}, err
	}
	outStr := strings.TrimSpace(string(out))
	secs, err := strconv.ParseInt(outStr, 10, 0)
	if err != nil {
		return time.Time{}, errors.Wrapf(err, "bad seconds: %q", out)
	}
	dur := time.Unix(secs, 0)
	return dur, nil
}

func SyncTime(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container

	// Set the time back 15 minutes, don't make a huge clock change as that can
	// cause other odd behaviors with timers.
	pastTime := time.Now().Add(-15 * time.Minute)
	// Set the time with maitred_client.
	cmd := testexec.CommandContext(ctx, "maitred_client", fmt.Sprintf("--cid=%d", cont.VM.ContextID), "--port=8888", fmt.Sprintf("--set_time_sec=%d", pastTime.Unix()))
	if err := cmd.Run(); err != nil {
		s.Error("Failed to set past time: ", err)
		cmd.DumpLog(ctx)
		return
	}

	// Verify that the time was set correctly.
	vmTime, err := getTime(ctx, s, cont)
	if err != nil {
		s.Error("Failed to get time: ", err)
		return
	}
	if diff := pastTime.Sub(vmTime); diff < -time.Minute || diff > time.Minute {
		s.Errorf("Maitred failed to set time: got %v, want %v", vmTime, pastTime)
	}

	if err = cont.VM.Concierge.SyncTimes(ctx); err != nil {
		s.Error("Calling syncTimes failed: ", err)
		return
	}

	vmTime, err = getTime(ctx, s, cont)
	if err != nil {
		s.Error("Failed to get time: ", err)
		return
	}
	actualTime := time.Now()
	if diff := actualTime.Sub(vmTime); diff < -time.Minute || diff > time.Minute {
		s.Errorf("Failed to correct time: got %v, want %v", vmTime, actualTime)
	}
}
