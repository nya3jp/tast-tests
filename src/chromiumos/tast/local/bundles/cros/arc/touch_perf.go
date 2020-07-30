// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
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
		Func:     TouchPerf,
		Desc:     "Test ARC touchscreen system performance",
		Contacts: []string{"arc-performance@google.com", "wvk@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_perbuild"},
		// TODO(wvk): Once clocks are synced between the host and guest, add
		// support for ARCVM to this test (b/123416853).
		SoftwareDeps: []string{"chrome", "android_p"},
		Pre:          arc.Booted(),
		Timeout:      2 * time.Minute,
	})
}

func TouchPerf(ctx context.Context, s *testing.State) {
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

	s.Log("Injecting touch events")
	const numEvents = 50
	eventTimes := make([]int64, 0, numEvents)
	// Stay in the middle of the screen to avoid activating the titlebar or
	// taskbar.
	var x, y input.TouchCoord = ts.Width() / 2, ts.Height() / 2
	for i := 0; i < numEvents; i++ {
		eventTimes = append(eventTimes, time.Now().UnixNano()/1000000)
		if err := stw.Move(x, y); err != nil {
			s.Fatal("Unable to inject touch event: ", err)
		}

		if (i & 1) == 0 {
			x += 20
		} else {
			x -= 20
		}

		if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			s.Fatal("Failed to sleep between events: ", err)
		}
	}

	s.Log("Collecting results")
	txt, err := inputlatency.WaitForEvents(ctx, d, numEvents)
	if err != nil {
		s.Fatal("Unable to wait for events: ", err)
	}
	var events []inputlatency.InputEvent
	if err := json.Unmarshal([]byte(txt), &events); err != nil {
		s.Fatal("Could not unmarshal events from app: ", err)
	}

	// Add RTCEventTime to inputEvents. We assume the order and number of events in the log
	// is the same as eventTimes.
	if len(events) != len(eventTimes) {
		s.Fatal("There are events missing from the log")
	}
	for i := range events {
		events[i].RTCEventTime = eventTimes[i]
	}

	mean, median, stdDev, max, min := inputlatency.CalculateMetrics(events, func(i int) float64 {
		return float64(events[i].Latency)
	})
	s.Logf("Touchscreen latency: mean %f median %f std %f max %f min %f", mean, median, stdDev, max, min)

	rmean, rmedian, rstdDev, rmax, rmin := inputlatency.CalculateMetrics(events, func(i int) float64 {
		return float64(events[i].RTCRecvTime - events[i].RTCEventTime)
	})
	s.Logf("Touchscreen RTC latency: mean %f median %f std %f max %f min %f", rmean, rmedian, rstdDev, rmax, rmin)

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "avgTouchscreenLatency",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, mean)
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}
