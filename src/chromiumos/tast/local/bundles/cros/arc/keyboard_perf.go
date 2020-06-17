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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

type inputEvent struct {
	Type       string // Usually either "KeyEvent" or "MotionEvent"
	KernelTime int64  // time (in ms) that the event was sent by the kernel
	RecvTime   int64  // time (in ms) that the event was received by the app
	Latency    int64  // difference between KernelTime and RecvTime
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         KeyboardPerf,
		Desc:         "Test ARC keyboard system performance",
		Contacts:     []string{"arc-performance@google.com", "wvk@google.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          arc.Booted(),
		Timeout:      2 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
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
	const numEvents = 50
	for i := 0; i < numEvents; i++ {
		if err := kbd.Type(ctx, "a"); err != nil {
			s.Fatal("Unable to inject key events: ", err)
		}
		testing.Sleep(ctx, 250*time.Millisecond)
	}

	s.Log("Collecting results from logcat")
	out, err := a.Command(ctx, "logcat", "-d", "-s", "InputLatencyTest:I").Output()
	if err != nil {
		s.Fatal("Unable to collect results from logcat: ", err)
	}

	events := parseLogcatEvents(string(out), s)
	if events == nil || len(events) == 0 {
		s.Fatal("Could not find any events in logcat")
	}

	mean, median, stdDev, max, min := calculateMetrics(events)
	s.Logf("Keyboard latency: mean %f median %f std %f max %f min %f", mean, median, stdDev, max, min)

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
// input events.
func calculateMetrics(events []inputEvent) (mean, median, stdDev, max, min float64) {
	n := len(events)
	sort.Slice(events, func(i, j int) bool { return events[i].Latency < events[j].Latency })
	min = float64(events[0].Latency)
	max = float64(events[n-1].Latency)
	median = float64(events[n/2].Latency)
	sum := float64(0)
	for _, e := range events {
		sum += float64(e.Latency)
	}
	mean = sum / float64(n)
	stdSum := float64(0)
	for _, e := range events {
		stdSum += math.Pow(float64(e.Latency)-mean, 2)
	}
	stdDev = math.Sqrt(stdSum / float64(n-1))
	return
}

// parseLogcatEvents parses logcat output in log for input events logged by
// the helper app.
func parseLogcatEvents(log string, s *testing.State) []inputEvent {
	re := regexp.MustCompile(`((Key|Touch)Event)\:(\d+)\:(\d+)\:(\d+)`)
	lines := strings.Split(log, "\n")
	events := make([]inputEvent, 0, len(lines))
	for _, line := range lines {
		match := re.FindStringSubmatch(line)
		if match == nil || len(match) == 0 {
			continue
		}
		kt, err := strconv.ParseInt(match[3], 10, 64)
		if err != nil {
			s.Logf("%s could not be parsed as an int", match[3])
			continue
		}
		rt, err := strconv.ParseInt(match[4], 10, 64)
		if err != nil {
			s.Logf("%s could not be parsed as an int", match[4])
			continue
		}
		lt, err := strconv.ParseInt(match[5], 10, 64)
		if err != nil {
			s.Logf("%s could not be parsed as an int", match[5])
			continue
		}
		events = append(events, inputEvent{
			Type:       match[1],
			KernelTime: kt,
			RecvTime:   rt,
			Latency:    lt,
		})
	}
	return events
}
