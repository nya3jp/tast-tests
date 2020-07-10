// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/*
Package inputlatency contains functions and structs used for measuring input latency on ARC.
*/
package inputlatency

import (
	"context"
	"math"
	"sort"
	"strconv"

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

	v := d.Object(ui.ID("org.chromium.arc.testapp.inputlatency:id/event_json"))
	txt, err := v.GetText(ctx)
	if err != nil {
		return "", err
	}
	return txt, nil
}
