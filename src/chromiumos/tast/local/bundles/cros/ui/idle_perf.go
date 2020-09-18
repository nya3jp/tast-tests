// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IdlePerf,
		Desc:         "Measures the CPU usage while the desktop is idle",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "arcvm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func IdlePerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome

	conn, err := cr.NewConn(ctx, ui.PerftestURL)
	if err != nil {
		s.Fatal("Failed to open the new tab page: ", err)
	}
	// No need to control the browser window, so it's safe to close the connection
	// now.
	if err = conn.Close(); err != nil {
		s.Fatal("Failed to close the connection: ", err)
	}
	if _, err = power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Recorder with no additional config; it records and reports memory usage and
	// CPU percents of browser/GPU processes.
	recorder, err := cuj.NewRecorder(ctx)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	if err := recorder.Run(ctx, tconn, func(ctx context.Context) error {
		s.Log("Just wait for 20 seconds to check the load of idle status")
		return testing.Sleep(ctx, 20*time.Second)
	}); err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}

	pv := perf.NewValues()
	if err = recorder.Record(pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
