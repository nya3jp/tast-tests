// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

type inputEvent struct {
	kind         string // One of "KeyEvent", "MotionEvent", or "InputEvent".
	eventTime    int64  // Time (in ms) that the event was sent by the kernel.
	rtcEventTime int64  // RTC time that the event was sent by the kernel.
	recvTime     int64  // Time (in ms) that the event was received by the app.
	rtcRecvTime  int64  // RTC time that the event was received by the app.
	latency      int64  // Difference between eventTime and recvTime.
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
	const numEvents = 25
	eventTimes := make([]int64, 0, numEvents*2)
	for i := 0; i < numEvents; i++ {
		eventTimes = append(eventTimes, time.Now().UnixNano()/1000000)
		if err := kbd.AccelPress(ctx, "a"); err != nil {
			s.Fatal("Unable to inject key events: ", err)
		}

		eventTimes = append(eventTimes, time.Now().UnixNano()/1000000)
		if err := kbd.AccelRelease(ctx, "a"); err != nil {
			s.Fatal("Unable to inject key events: ", err)
		}
	}

	s.Log("Closing test app")
	if err := kbd.Type(ctx, "\x1b"); err != nil {
		s.Fatal("Cannot send Esc key: ", err)
	}
	var timeout time.Duration
	if dl, ok := ctx.Deadline(); ok {
		timeout = time.Until(dl)
	} else {
		timeout = 30 * time.Second
	}
	if err := act.WaitForFinished(ctx, timeout); err != nil {
		s.Fatal("Unable to wait for test app to finish: ", err)
	}

	s.Log("Collecting results")
	const path = "/storage/emulated/0/Android/data/org.chromium.arc.testapp.inputlatency/files/latency_test_results.txt"
	out, err := a.ReadFile(ctx, path)
	if err != nil {
		s.Fatal("Unable to collect results: ", err)
	}

	events, err := parseEvents(eventTimes, string(out))
	if err != nil {
		s.Fatal("Could not parse events from log: ", err)
	}

	mean, median, stdDev, max, min := calculateMetrics(events, func(i int) float64 {
		return float64(events[i].latency)
	})
	s.Logf("Keyboard latency: mean %f median %f std %f max %f min %f", mean, median, stdDev, max, min)

	rmean, rmedian, rstdDev, rmax, rmin := calculateMetrics(events, func(i int) float64 {
		return float64(events[i].rtcRecvTime - events[i].rtcEventTime)
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
	stdDev = math.Sqrt(stdSum / float64(n-1))
	return
}

// parseEvents parses output in log for input events logged by the helper app.
func parseEvents(eventTimes []int64, log string) ([]inputEvent, error) {
	re := regexp.MustCompile(`((Key|Motion|Input)Event)\:(\d+)\:(\d+)\:(\d+)\:(\d+)`)
	lines := strings.Split(log, "\n")
	events := make([]inputEvent, 0, len(lines))
	for _, line := range lines {
		match := re.FindStringSubmatch(line)
		if match == nil || len(match) == 0 {
			continue
		}
		et, err := strconv.ParseInt(match[3], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse line %q", line)
		}
		rt, err := strconv.ParseInt(match[4], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse line %q", line)
		}
		rtc, err := strconv.ParseInt(match[5], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse line %q", line)
		}
		lt, err := strconv.ParseInt(match[6], 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "could not parse line %q", line)
		}
		events = append(events, inputEvent{
			kind:        match[1],
			eventTime:   et,
			recvTime:    rt,
			rtcRecvTime: rtc,
			latency:     lt,
		})
	}
	// Add rtcEventTime to inputEvents. We assume the order and number of events in the log
	// is the same as eventTimes.
	if len(events) != len(eventTimes) {
		return nil, errors.New("there are events missing from the log")
	}
	for i := range events {
		events[i].rtcEventTime = eventTimes[i]
	}
	return events, nil
}
