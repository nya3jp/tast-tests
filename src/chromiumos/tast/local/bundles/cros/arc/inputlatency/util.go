// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package inputlatency contains functions and structs used for measuring input latency on ARC.
package inputlatency

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"time"
	_ "unsafe" // required to use //go:linkname

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// InputEvent represents a single input event received by the helper app.
type InputEvent struct {
	// Time (in ns) that the event was sent by the kernel (filled by host).
	EventTimeNS int64
	// Time (in ns) that the event was received by the app.
	RecvTimeNS int64 `json:"receiveTimeNs"`
}

// CalculateMetrics calculates mean, median, std dev, max and min for the given
// input events. The function getValue should return the value of the element
// corresponding to the given index.
func CalculateMetrics(events []InputEvent, getValue func(int) float64) (mean, median, stdDev, max, min float64) {
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

// WaitForEvents polls until the counter in the app UI is equal to count, then
// returns the input events from the helper app.
func WaitForEvents(ctx context.Context, d *ui.Device, count int) (string, error) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
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
			return errors.Errorf("unexpected event count; got %d, want %d", num, count)
		}
		return nil
	}, nil); err != nil {
		return "", err
	}

	// Press ESC key to finish event trace and generate JSON data.
	if err := d.PressKeyCode(ctx, ui.KEYCODE_ESCAPE, 0x0); err != nil {
		return "", err
	}

	var txt string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		v := d.Object(ui.ID("org.chromium.arc.testapp.inputlatency:id/event_json"))
		var err error
		txt, err = v.GetText(ctx)
		if err != nil {
			return err
		}
		if txt == "" {
			return errors.New("waiting for generate JSON data")
		}
		return nil
	}, nil); err != nil {
		return "", err
	}
	return txt, nil
}

// WaitForClearUI clears the event data in ArcInputLatencyTest.apk to get ready for next event tracing.
func WaitForClearUI(ctx context.Context, d *ui.Device) error {
	if err := d.PressKeyCode(ctx, ui.KEYCODE_DEL, 0x0); err != nil {
		return err
	}

	// Check whether events are cleared.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		v := d.Object(ui.ID("org.chromium.arc.testapp.inputlatency:id/event_count"))
		txt, err := v.GetText(ctx)
		if err != nil {
			return err
		}
		num, err := strconv.ParseInt(txt, 10, 64)
		if err != nil {
			return err
		}
		if num != 0 {
			return errors.Errorf("failed to clean events; got %d, want 0", num)
		}
		return nil
	}, nil); err != nil {
		return err
	}
	return nil
}

// EvaluateLatency gets event data, calculates the latency, and adds the result to performance metrics.
func EvaluateLatency(ctx context.Context, s *testing.State, d *ui.Device,
	numEvents int, eventTimes []int64, perfName string, pv *perf.Values) error {
	s.Log("Collecting results")
	txt, err := WaitForEvents(ctx, d, numEvents)
	if err != nil {
		return errors.Wrap(err, "unable to wait for events")
	}
	var events []InputEvent
	if err := json.Unmarshal([]byte(txt), &events); err != nil {
		return errors.Wrap(err, "could not ummarshal events from app")
	}

	// Assign event RTC time.
	for i := range events {
		events[i].EventTimeNS = eventTimes[i]
	}

	mean, median, stdDev, max, min := CalculateMetrics(events, func(i int) float64 {
		return float64(events[i].RecvTimeNS-events[i].EventTimeNS) / 1000000.
	})
	s.Logf("Latency (ms): mean %f median %f std %f max %f min %f", mean, median, stdDev, max, min)

	pv.Set(perf.Metric{
		Name:      perfName,
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, mean)
	return nil
}

//go:noescape
//go:linkname nanotime runtime.nanotime
func nanotime() int64

// Now returns the current time in nanoseconds from a monotonic clock. This uses
// runtime.nanotime() which uses CLOCK_MONOTONIC on Linux. We must use this since time.Now()
// does not let us access the raw monotonic time value, which we need to compare to the
// monotonic time value taken on Android.
func Now() time.Duration {
	return time.Duration(nanotime())
}

// WaitForNextEventTime generates next event time with specific time interval in millisecond.
func WaitForNextEventTime(ctx context.Context, a *arc.ARC, eventTimes *[]int64, ms time.Duration) error {
	// Wait to generate next event time.
	if err := testing.Sleep(ctx, ms*time.Millisecond); err != nil {
		return errors.Wrap(err, "timeout while waiting to generate next event time")
	}

	diff := int64(0)
	if vmEnabled, err := arc.VMEnabled(); err != nil {
		return errors.Wrap(err, "unable to check install type of ARC")
	} else if vmEnabled {
		// Get current boottime diff between guest and host. Since the diff changes
		// value as the guest clock drifts, we must get the current diff before each
		// event.
		d, err := vmTimeDiff(ctx, a)
		if err != nil {
			return errors.Wrap(err, "unable to get VM time diff")
		}
		diff = d
	}

	*eventTimes = append(*eventTimes, Now().Nanoseconds()-diff)
	return nil
}

// vmTimeDiff runs arc-host-clock-client in the VM to determine the time difference
// between ARCVM and the host.
func vmTimeDiff(ctx context.Context, a *arc.ARC) (int64, error) {
	out, err := a.Command(ctx, "arc-host-clock-client").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to run arc-host-clock-client")
	}
	i, err := strconv.ParseInt(string(out), 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "unable to convert arc-host-clock-client output")
	}
	return i, nil
}
