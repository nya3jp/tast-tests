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

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/testing"
)

// InputEvent represents a single input event received by the helper app.
type InputEvent struct {
	// One of "KeyEvent", "MotionEvent", or "InputEvent".
	Kind string `json:"type"`
	// Time (in ms) that the event was sent by the kernel.
	EventTime int64 `json:"eventTime"`
	// RTC time that the event was sent by the kernel.
	RTCEventTime int64
	// Time (in ms) that the event was received by the app.
	RecvTime int64 `json:"receiveTime"`
	// RTC time that the event was received by the app.
	RTCRecvTime int64 `json:"rtcReceiveTime"`
	// Difference between eventTime and recvTime.
	Latency int64 `json:"latency"`
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
	numEvents int, eventTimes *[]int64, perfName string, pv *perf.Values) error {
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
		events[i].RTCEventTime = (*eventTimes)[i]
	}

	mean, median, stdDev, max, min := CalculateMetrics(events, func(i int) float64 {
		return float64(events[i].Latency)
	})
	s.Logf("Latency: mean %f median %f std %f max %f min %f", mean, median, stdDev, max, min)

	rmean, rmedian, rstdDev, rmax, rmin := CalculateMetrics(events, func(i int) float64 {
		return float64(events[i].RTCRecvTime - events[i].RTCEventTime)
	})
	s.Logf("RTC latency: mean %f median %f std %f max %f min %f", rmean, rmedian, rstdDev, rmax, rmin)

	pv.Set(perf.Metric{
		Name:      perfName,
		Unit:      "milliseconds",
		Direction: perf.SmallerIsBetter,
	}, mean)
	return nil
}

// WaitForNextEventTime generates next event time with specific time interval in millisecond.
func WaitForNextEventTime(ctx context.Context, eventTimes *[]int64, ms time.Duration) error {
	// Wait to generate next event time.
	if err := testing.Sleep(ctx, ms*time.Millisecond); err != nil {
		return errors.Wrap(err, "timeout while waiting to generate next event time")
	}
	*eventTimes = append(*eventTimes, time.Now().UnixNano()/1000000)
	return nil
}
