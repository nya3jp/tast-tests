// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/inputlatency"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MousePerf,
		Desc:         "Test ARC mouse system performance",
		Contacts:     []string{"arc-performance@google.com", "wvk@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         inputlatency.AndroidData(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Pre:     arc.Booted(),
		Timeout: 2 * time.Minute,
	})
}

func MousePerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	a := s.PreValue().(arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Could not initialize UI Automator: ", err)
	}

	s.Log("Creating virtual mouse")
	m, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal("Unable to create virtual mouse: ", err)
	}
	defer m.Close()

	if err := inputlatency.InstallArcHostClockClient(ctx, a, s); err != nil {
		s.Fatal("Could not install arc-host-clock-client: ", err)
	}

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
	const (
		numEvents = 100
		waitMS    = 50
	)
	eventTimes := make([]int64, 0, numEvents)
	var x, y int32 = 10, 0
	if err := m.Press(); err != nil {
		s.Fatal("Unable to inject Press mouse event: ", err)
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
		if err := inputlatency.WaitForNextEventTime(ctx, a, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		if err := m.Move(x, y); err != nil {
			s.Fatal("Unable to inject Move mouse event: ", err)
		}
	}

	pv := perf.NewValues()

	if err := inputlatency.EvaluateLatency(ctx, s, d, numEvents, eventTimes, "avgMouseLeftMoveLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := m.Release(); err != nil {
		s.Fatal("Unable to inject Release mouse event: ", err)
	}

	if err := inputlatency.WaitForClearUI(ctx, d); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}

	// When left-clicking on mouse, it injects ACTION_DOWN, ACTION_BUTTON_PRESS, ACTION_UP and ACTION_BUTTON_RELEASE events.
	// Check latency for these four actions.
	s.Log("Injecting mouse left-click events")
	eventTimes = make([]int64, 0, numEvents)
	for i := 0; i < numEvents; i += 4 {
		if err := inputlatency.WaitForNextEventTime(ctx, a, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// ACTION_DOWN and ACTION_BUTTON_PRESS are generated together.
		eventTimes = append(eventTimes, eventTimes[len(eventTimes)-1])
		if err := m.Press(); err != nil {
			s.Fatal("Unable to inject Press mouse event: ", err)
		}

		if err := inputlatency.WaitForNextEventTime(ctx, a, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// ACTION_UP and ACTION_BUTTON_RELEASE are generated together.
		eventTimes = append(eventTimes, eventTimes[len(eventTimes)-1])
		if err := m.Release(); err != nil {
			s.Fatal("Unable to inject Release mouse event: ", err)
		}
	}

	if err := inputlatency.EvaluateLatency(ctx, s, d, numEvents, eventTimes, "avgMouseLeftClickLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
