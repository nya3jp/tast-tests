// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Running,
		Desc: "Checks that hpsd is running on devices with hps enabled",
		Contacts: []string{
			"evanbenn@chromium.org", // Test author
			"chromeos-hps-swe@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
		// TODO(b/227525135): re-enable when we have some brya DUTs with HPS
		HardwareDeps: hwdep.D(hwdep.SkipOnModel("brya")),
		SoftwareDeps: []string{"hps"},
	})
}

// Running checks the hpsd job and fails if it isn't running or has a process
// in the zombie state.
func Running(ctx context.Context, s *testing.State) {
	if err := upstart.CheckJob(ctx, "hpsd"); err != nil {
		s.Fatal("Test failed: ", err)
	}
}
