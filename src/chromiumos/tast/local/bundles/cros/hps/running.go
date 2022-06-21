// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"

	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/media/vm"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Running,
		Desc: "Checks that hpsd is running on devices with hps enabled",
		Contacts: []string{
			"evanbenn@chromium.org", // Test author
			"chromeos-hps-swe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"hps"},
	})
}

// Running checks the hpsd job and fails if it isn't running or has a process
// in the zombie state.
func Running(ctx context.Context, s *testing.State) {
	hasHps, err := crosconfig.Get(ctx, "/hps", "has-hps")
	if err != nil && !crosconfig.IsNotFound(err) {
		s.Fatal("Failed to get has-hps property: ", err)
	}
	// hpsd is only expected to be running if the HPS hardware is present,
	// or if it's configured to use a fake device in a VM.
	expectRunning := hasHps == "true" || vm.IsRunningOnVM()

	if expectRunning {
		if err := upstart.CheckJob(ctx, "hpsd"); err != nil {
			s.Fatal("Test failed: ", err)
		}
	} else {
		_, state, _, err := upstart.JobStatus(ctx, "hpsd")
		if err != nil {
			s.Fatal("Failed to get hpsd Upstart job status: ", err)
		}
		if state != upstartcommon.WaitingState {
			s.Fatalf("hpsd unexpectedly in state %v, expected waiting", state)
		}
	}
}
