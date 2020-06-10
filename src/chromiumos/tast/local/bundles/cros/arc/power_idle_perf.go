// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testArgsForPowerIdlePerf struct {
	setupOption setup.BatteryDischargeMode
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerIdlePerf,
		Desc: "Measures the battery drain of an idle system",
		Contacts: []string{
			"cwd@chromium.org",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Params: []testing.Param{{
			Name:              "noarc",
			ExtraSoftwareDeps: []string{"arc"}, // to prevent this from running on non-ARC boards
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.ForceBatteryDischarge,
			},
			Pre: chrome.LoggedIn(),
		}, {
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.ForceBatteryDischarge,
			},
			Pre: arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.ForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.ForceBatteryDischarge,
			},
			Pre: arc.Booted(),
		}, {
			Name:              "noarc_nobatterymetrics",
			ExtraSoftwareDeps: []string{"arc"}, // to prevent this from running on non-ARC boards
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.NoBatteryDischarge,
			},
			Pre: chrome.LoggedIn(),
		}, {
			Name:              "nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_p"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.NoBatteryDischarge,
			},
			Pre: arc.Booted(),
		}, {
			Name:              "vm_nobatterymetrics",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.NoForceDischarge()),
			Val: testArgsForPowerIdlePerf{
				setupOption: setup.NoBatteryDischarge,
			},
			Pre: arc.Booted(),
		}},
		Timeout: 15 * time.Minute,
	})
}

func PowerIdlePerf(ctx context.Context, s *testing.State) {
	const (
		iterationCount    = 30
		iterationDuration = 10 * time.Second
	)
	args := s.Param().(testArgsForPowerIdlePerf)

	// Give cleanup actions a minute to run, even if we fail by exceeding our
	// deadline.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		cr = s.PreValue().(arc.PreData).Chrome
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	sup, cleanup := setup.New("power idle perf")
	defer func() {
		if err := cleanup(cleanupCtx); err != nil {
			s.Fatal("Cleanup failed: ", err)
		}
	}()

	sup.Add(setup.PowerTest(ctx, tconn, args.setupOption))
	if err := sup.Check(ctx); err != nil {
		s.Fatal("Setup failed: ", err)
	}

	metrics, err := perf.NewTimeline(ctx, power.TestMetrics(), perf.Interval(iterationDuration))
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}
	s.Log("Finished setup")

	if err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		s.Fatal("Failed to cool down: ", err)
	}

	if err := metrics.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	if err := metrics.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	if err := testing.Sleep(ctx, iterationCount*iterationDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	p, err := metrics.StopRecording()
	if err != nil {
		s.Fatal("Error while recording power metrics: ", err)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
