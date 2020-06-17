// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

type inputEvent struct {
	// One of "KeyEvent", "MotionEvent", or "InputEvent".
	Kind string `json:"type"`
	// Time (in ms) that the event was sent by the kernel.
	EventTime int64 `json:"eventTime"`
	// RTC time that the event was sent by the kernel.
	rtcEventTime int64
	// Time (in ms) that the event was received by the app.
	RecvTime int64 `json:"receiveTime"`
	// RTC time that the event was received by the app.
	RTCRecvTime int64 `json:"rtcReceiveTime"`
	// Difference between eventTime and recvTime.
	Latency int64 `json:"latency"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     KeyboardPerf,
		Desc:     "Test ARC keyboard system performance",
		Contacts: []string{"arc-performance@google.com", "wvk@google.com"},
		// TODO(wvk): Once clocks are synced between the host and guest, add
		// support for ARCVM to this test (b/123416853).
		SoftwareDeps: []string{"chrome", "android_p"},
		Pre:          arc.Booted(),
		Timeout:      2 * time.Minute,
	})
}

func KeyboardPerf(ctx context.Context, s *testing.State) {
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

	s.Log("Creating virtual keyboard")
	kbd, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Unable to create virtual keyboard: ", err)
	}
	defer kbd.Close()

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

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait until CPU idle: ", err)
	}

	s.Log("Injecting key events")
	const numEvents = 50
	eventTimes := make([]int64, 0, numEvents)
	for i := 0; i < numEvents; i += 2 {
		eventTimes = append(eventTimes, time.Now().UnixNano()/1000000)
		if err := kbd.AccelPress(ctx, "a"); err != nil {
			s.Fatal("Unable to inject key events: ", err)
		}

		eventTimes = append(eventTimes, time.Now().UnixNano()/1000000)
		if err := kbd.AccelRelease(ctx, "a"); err != nil {
			s.Fatal("Unable to inject key events: ", err)
		}
	}

	s.Log("Collecting results")
	if err := waitForEventCount(ctx, d, numEvents); err != nil {
		s.Fatal("Unable to wait for events to reach expected count: ", err)
	}

	events, err := getEvents(ctx, d)
	if err != nil {
		s.Fatal("Could not retrieve events from app: ", err)
	}
	// Add rtcEventTime to inputEvents. We assume the order and number of events in the log
	// is the same as eventTimes.
	if len(events) != len(eventTimes) {
		s.Fatal("There are events missing from the log")
	}
	for i := range events {
		events[i].rtcEventTime = eventTimes[i]
	}

	mean, median, stdDev, max, min := calculateMetrics(events, func(i int) float64 {
		return float64(events[i].Latency)
	})
	s.Logf("Keyboard latency: mean %f median %f std %f max %f min %f", mean, median, stdDev, max, min)

	rmean, rmedian, rstdDev, rmax, rmin := calculateMetrics(events, func(i int) float64 {
		return float64(events[i].RTCRecvTime - events[i].rtcEventTime)
	})
	s.Logf("Keyboard RTC latency: mean %f median %f std %f max %f min %f", rmean, rmedian, rstdDev, rmax, rmin)

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "avgKeyboardLatency",
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, mean)
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// calculateMetrics calculates mean, median, std dev, max and min for the given
// input events. The function getValue should return the value of the element
// corresponding to the given index.
func calculateMetrics(events []inputEvent, getValue func(int) float64) (mean, median, stdDev, max, min float64) {
	n := len(events)
	sort.Slice(events, func(i, j int) bool { return getValue(i) < getValue(j) })
	min = getValue(0)
	max = getValue(n - 1)
	median = getValue(n / 2)
	sum := 0.
	for i := range events {
		sum += getValue(i)
	}
	mean = sum / float64(n)
	stdSum := 0.
	for i := range events {
		stdSum += math.Pow(getValue(i)-mean, 2)
	}
	stdDev = math.Sqrt(stdSum / float64(n))
	return
}

// waitForEventCount polls until the counter in the app UI is equal to count.
func waitForEventCount(ctx context.Context, d *ui.Device, count int) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		v := d.Object(ui.ID("org.chromium.arc.testapp.inputlatency:id/event_count"))
		txt, err := v.GetText(ctx)
		if err != nil {
			return err
		}
		num, err := strconv.ParseInt(txt, 10, 64)
		if err != nil {
			return err
		}
		if num != int64(count) {
			return errors.Errorf("Event count in app (%d) != expected event count (%d)", num, count)
		}
		return nil
	}, nil)
}

// getEvents retrieves and unmarshals input events from the helper app.
func getEvents(ctx context.Context, d *ui.Device) ([]inputEvent, error) {
	v := d.Object(ui.ID("org.chromium.arc.testapp.inputlatency:id/event_json"))
	txt, err := v.GetText(ctx)
	if err != nil {
		return nil, err
	}

	var events []inputEvent
	if err := json.Unmarshal([]byte(txt), &events); err != nil {
		return nil, err
	}
	return events, nil
}
