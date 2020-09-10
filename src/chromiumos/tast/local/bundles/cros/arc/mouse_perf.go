// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/inputlatency"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MousePerf,
		Desc:     "Test ARC mouse system performance",
		Contacts: []string{"arc-performance@google.com", "wvk@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		// TODO(wvk): Once clocks are synced between the host and guest, add
		// support for ARCVM to this test (b/123416853).
		SoftwareDeps: []string{"chrome", "android_p"},
		Pre:          arc.Booted(),
		Timeout:      2 * time.Minute,
	})
}

func MousePerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	a := s.PreValue().(arc.PreData).ARC
	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Could not initialize UI Automator: ", err)
	}

	s.Log("Creating virtual mouse")
	m, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal("Unable to create virtual mouse: ", err)
	}
	defer m.Close()

	const (
		apkName      = "ArcInputLatencyTest.apk"
		appName      = "org.chromium.arc.testapp.inputlatency"
		activityName = ".MainActivity"
	)
	s.Log("Installing " + apkName)
	if err := a.Install(ctx, arc.APKPath(apkName)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	s.Logf("Launching %s/%s", appName, activityName)
	act, err := arc.NewActivity(a, appName, activityName)
	if err != nil {
		s.Fatalf("Unable to create new activity %s/%s: %v", appName, activityName, err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Unable to launch %s/%s: %v", appName, activityName, err)
	}
	defer act.Stop(ctx, tconn)

	if err := act.SetWindowState(ctx, tconn, arc.WindowStateFullscreen); err != nil {
		s.Fatal("Could not maximize test app: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait until CPU idle: ", err)
	}

	// Check latency for mouse ACTION_MOVE events which are generated when moving mouse after left-button pressing down and holding.
	s.Log("Injecting mouse move events")
	const numEvents = 50
	const waitMS = 50
	eventTimes := make([]int64, 0, numEvents)
	var x, y int32 = 10, 0
	if err := m.Press(); err != nil {
		s.Fatal("Unable to inject mouse press event: ", err)
	}
	if err := inputlatency.WaitForClearUI(ctx, d); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}
	for i := 0; i < numEvents; i++ {
		if x == 10 {
			x = -10
		} else {
			x = 10
		}
		if err := inputlatency.WaitForNextEventTime(ctx, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		if err := m.Move(x, y); err != nil {
			s.Fatal("Unable to inject mouse event: ", err)
		}
	}

	pv := perf.NewValues()

	if err := inputlatency.EvaluateLatency(ctx, s, d, numEvents, &eventTimes, "avgMouseLeftMoveLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := m.Release(); err != nil {
		s.Fatal("Unable to inject mouse press event: ", err)
	}

	if err := inputlatency.WaitForClearUI(ctx, d); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}

	// Check latency for these four actions.
	// When left-clicking on mouse, it injects ACTION_DOWN, ACTION_BUTTON_PRESS, ACTION_UP and ACTION_BUTTON_RELEASE events.
	s.Log("Injecting mouse left-click events")
	eventTimes = make([]int64, 0, numEvents*2)
	for i := 0; i < numEvents/2; i++ {
		if err := inputlatency.WaitForNextEventTime(ctx, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// ACTION_DOWN and ACTION_BUTTON_PRESS are generated together.
		eventTimes = append(eventTimes, eventTimes[len(eventTimes)-1])
		if err := m.Press(); err != nil {
			s.Fatal("Unable to left-click down: ", err)
		}

		if err := inputlatency.WaitForNextEventTime(ctx, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// ACTION_UP and ACTION_BUTTON_RELEASE are generated together.
		eventTimes = append(eventTimes, eventTimes[len(eventTimes)-1])
		if err := m.Release(); err != nil {
			s.Fatal("Unable to release left-click: ", err)
		}
	}

	if err := inputlatency.EvaluateLatency(ctx, s, d, numEvents*2, &eventTimes, "avgMouseLeftClickLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
