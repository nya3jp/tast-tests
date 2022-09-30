// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/inputlatency"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MousePerf,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test ARC mouse system performance",
		Contacts:     []string{"arc-performance@google.com"},
		// Disabled due to <1% pass rate over 30 days. See b/241943132
		//Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         inputlatency.AndroidData(),
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "lacrosWithArcBooted",
		},
		/*		{
					Name:              "vm",
					ExtraSoftwareDeps: []string{"android_vm"},
					Fixture: "arcBooted",
				}, {
					Name:              "vm_lacros",
					ExtraSoftwareDeps: []string{"android_vm", "lacros"},
					Fixture: "lacrosWithArcBooted",
				}*/
		},
		Timeout: 2 * time.Minute,
	})
}

func MousePerf(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	d := s.FixtValue().(*arc.PreData).UIDevice

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

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
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
	s.Log("Injecting mouse press-down move events")
	const (
		numEvents = 100
		waitMS    = 50
		y         = 0
	)
	eventTimes := make([]int64, 0, numEvents)
	if err := m.Press(); err != nil {
		s.Fatal("Unable to inject Press mouse event: ", err)
	}
	if err := inputlatency.WaitForClearUI(ctx, d); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}
	var x int32 = 10
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

	s.Log("Injecting mouse left-click events")
	eventTimes = make([]int64, 0, numEvents)
	ver, err := arc.SDKVersion()
	if err != nil {
		s.Fatal("Failed to get SDK version: ", err)
	}
	// When left-clicking on mouse, it injects ACTION_DOWN, ACTION_BUTTON_PRESS, ACTION_UP, and ACTION_BUTTON_RELEASE.
	// On R, the framework also injects ACTION_HOVER_MOVE.
	// Check latency for these actions.
	var leftClickNumEvents int
	if ver >= arc.SDKR {
		leftClickNumEvents = 5
	} else {
		leftClickNumEvents = 4
	}
	for i := 0; i < numEvents; i += leftClickNumEvents {
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
		// ACTION_UP, ACTION_BUTTON_RELEASE, and ACTION_HOVER_MOVE are generated together.
		eventTimes = append(eventTimes, eventTimes[len(eventTimes)-1])
		if leftClickNumEvents == 5 {
			eventTimes = append(eventTimes, eventTimes[len(eventTimes)-1])
		}
		if err := m.Release(); err != nil {
			s.Fatal("Unable to inject Release mouse event: ", err)
		}
	}

	if err := inputlatency.EvaluateLatency(ctx, s, d, numEvents, eventTimes, "avgMouseLeftClickLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	// Clear data to start next test.
	if err := inputlatency.WaitForClearUI(ctx, d); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}

	s.Log("Injecting the mouse hover-move events")
	// Additional ACTION_HOVER_ENTER event is generated for the first mouse hover-move event.
	if err := m.Move(x, y); err != nil {
		s.Fatal("Unable to inject mouse hover-move event: ", err)
	}
	if err := inputlatency.WaitForClearUI(ctx, d); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}

	eventTimes = make([]int64, 0, numEvents)
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
			s.Fatal("Unable to inject mouse hover-move event: ", err)
		}
	}
	if err := inputlatency.EvaluateLatency(ctx, s, d, numEvents, eventTimes, "avgMouseHoverMoveLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
