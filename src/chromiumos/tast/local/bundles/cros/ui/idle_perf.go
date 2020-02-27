// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
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
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Val:               []string{},
		}, {
			Name:              "arcvm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               []string{"--enable-arcvm"},
		}},
	})
}

func IdlePerf(ctx context.Context, s *testing.State) {
	args := s.Param().([]string)
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	conn, err := cr.NewConn(ctx, ui.PerftestURL)
	if err != nil {
		s.Fatal("Failed to open the new tab page: ", err)
	}
	// No need to control the browser window, so it's safe to close the connection
	// now.
	if err = conn.Close(); err != nil {
		s.Fatal("Failed to close the connection: ", err)
	}
	s.Log("Waiting 1 min for stability")
	if err = testing.Sleep(ctx, time.Minute); err != nil {
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
	if err := recorder.Run(ctx, tconn, func() error {
		s.Log("Just wait for 20 seconds to check the load of idle status")
		return testing.Sleep(ctx, 20*time.Second)
	}); err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}

	if err = recorder.Stop(); err != nil {
		s.Fatal("Failed to stop the recorder: ", err)
	}

	pv := perf.NewValues()
	if err = recorder.Record(pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
