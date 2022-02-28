// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type tracingMode bool

const (
	tracingOn    tracingMode = true
	tracingOff   tracingMode = false
	idleDuration             = 30 * time.Second
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IdlePerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures the CPU usage while the desktop is idle",
		Contacts:     []string{"xiyuan@chromium.org", "yichenz@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      cuj.CPUStablizationTimeout + idleDuration,
		Pre:          arc.Booted(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               tracingOff,
		}, {
			Name:              "trace",
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               tracingOn,
		}, {
			Name:              "arcvm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               tracingOff,
		}, {
			Name:              "arcvm_trace",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               tracingOn,
		}},
	})
}

func IdlePerf(ctx context.Context, s *testing.State) {
	tracing := s.Param().(tracingMode)

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	// Wait for cpu to stabilize before test.
	if err := cpu.WaitUntilStabilized(ctx, cuj.CPUCoolDownConfig()); err != nil {
		// Log the cpu stabilizing wait failure instead of make it fatal.
		// TODO(b/213238698): Include the error as part of test data.
		s.Log("Failed to wait for CPU to become idle: ", err)
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	// Recorder with no additional config; it records and reports memory usage and
	// CPU percents of browser/GPU processes.
	recorder, err := cuj.NewRecorder(ctx, cr, a)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer func() {
		if err := recorder.Close(closeCtx); err != nil {
			s.Error("Failed to stop recorder: ", err)
		}
	}()
	if tracing {
		recorder.EnableTracing(s.OutDir())
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		s.Log("Just wait for ", idleDuration, " to check the load of idle status")
		return testing.Sleep(ctx, idleDuration)
	}); err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}

	pv := perf.NewValues()
	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
