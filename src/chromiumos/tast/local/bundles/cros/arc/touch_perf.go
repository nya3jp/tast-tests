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
		Func:         TouchPerf,
		Desc:         "Test ARC touchscreen system performance",
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
		Fixture: "arcBooted",
		Timeout: 2 * time.Minute,
	})
}

func TouchPerf(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Could not open Test API connection: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Could not initialize UI Automator: ", err)
	}
	defer d.Close(ctx)

	s.Log("Creating virtual touchscreen")
	ts, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Unable to create virtual touchscreen: ", err)
	}
	defer ts.Close()
	stw, err := ts.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Unable to create SingleTouchWriter: ", err)
	}
	defer stw.Close()

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

	s.Log("Injecting touch move events")
	const numEvents = 50
	const waitMS = 50
	eventTimes := make([]int64, 0, numEvents)
	// Stay in the middle of the screen to avoid activating the titlebar or
	// taskbar.
	var x, y input.TouchCoord = ts.Width() / 2, ts.Height() / 2

	// The first touch ACTION_DOWN is not counted when evaluating touch ACTION_MOVE latency.
	if err := stw.Move(x, y); err != nil {
		s.Fatal("Unable to inject touch event: ", err)
	}
	if err := inputlatency.WaitForClearUI(ctx, d); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}

	for i := 0; i < numEvents; i++ {
		if (i & 1) == 0 {
			x += 20
		} else {
			x -= 20
		}

		if err := inputlatency.WaitForNextEventTime(ctx, a, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}

		if err := stw.Move(x, y); err != nil {
			s.Fatal("Unable to inject touch event: ", err)
		}
	}

	pv := perf.NewValues()

	if err := inputlatency.EvaluateLatency(ctx, s, d, numEvents, eventTimes, "avgTouchscreenMoveLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	// End of touch. The last touch ACTION_UP is not counted when evaluating touch ACTION_MOVE latency.
	if err := stw.End(); err != nil {
		s.Fatal("Unable to release touch: ", err)
	}

	if err := inputlatency.WaitForClearUI(ctx, d); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}

	s.Log("Injecting touch press events")
	eventTimes = make([]int64, 0, numEvents)
	for i := 0; i < numEvents/2; i++ {
		if err := inputlatency.WaitForNextEventTime(ctx, a, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		if err := stw.Move(x, y); err != nil {
			s.Fatal("Unable to touch down: ", err)
		}
		if err := inputlatency.WaitForNextEventTime(ctx, a, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		if err := stw.End(); err != nil {
			s.Fatal("Unable to release touch: ", err)
		}
	}

	if err := inputlatency.EvaluateLatency(ctx, s, d, numEvents, eventTimes, "avgTouchscreenPressLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
