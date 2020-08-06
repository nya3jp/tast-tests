// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/inputlatency"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GamepadPerf,
		Desc:         "Test ARC gamepad system performance",
		Contacts:     []string{"arc-performance@google.com", "ruanc@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "android_p"},
		Pre:          arc.Booted(),
		Timeout:      2 * time.Minute,
	})
}

func GamepadPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(arc.PreData).Chrome
	a := s.PreValue().(arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Could not initialize UI Automator: ", err)
	}

	s.Log("Creating a virtual gamepad device")
	gp, err := input.Gamepad(ctx)
	if err != nil {
		s.Fatal("Failed to create a gamepad: ", err)
	}
	defer func() {
		if gp != nil {
			gp.Close()
		}
	}()

	s.Log("Created a virtual gamepad device ", gp.Device())

	const (
		apk = "ArcInputLatencyTest.apk"
		pkg = "org.chromium.arc.testapp.inputlatency"
		cls = pkg + ".MainActivity"
	)

	s.Log("Installing ", apk)
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install the APK: ", err)
	}

	s.Log("Launching ", cls)
	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait until CPU idle: ", err)
	}

	generateNextEventTime := func(eventTimes *[]int64) error {
		// Wait to generate next event time.
		if err := testing.Sleep(ctx, 50*time.Millisecond); err != nil {
			return errors.Wrap(err, "timeout while waiting to generate next event time")
		}
		// The timestamp passed for gamepad from Chrome side should be in nanosecond.
		*eventTimes = append(*eventTimes, time.Now().UnixNano())
		return nil
	}

	clearUI := func() error {
		v := d.Object(ui.ID("org.chromium.arc.testapp.inputlatency:id/clear_btn"))
		if err := v.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click clear button")
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

	evaluate := func(numEvents int, eventTimes *[]int64, perfName string, pv *perf.Values) error {
		s.Log("Collecting results")
		txt, err := inputlatency.WaitForEvents(ctx, d, numEvents)
		if err != nil {
			return errors.Wrap(err, "unable to wait for events")
		}
		var events []inputlatency.InputEvent
		if err := json.Unmarshal([]byte(txt), &events); err != nil {
			return errors.Wrap(err, "could not ummarshal events from app")
		}

		// Assign event RTC time.
		for i := range events {
			events[i].RTCEventTime = (*eventTimes)[i] / 1000000
		}

		mean, median, stdDev, max, min := inputlatency.CalculateMetrics(events, func(i int) float64 {
			return float64(events[i].Latency)
		})
		s.Logf("Gamepad latency: mean %f median %f std %f max %f min %f", mean, median, stdDev, max, min)

		rmean, rmedian, rstdDev, rmax, rmin := inputlatency.CalculateMetrics(events, func(i int) float64 {
			return float64(events[i].RTCRecvTime - events[i].RTCEventTime)
		})
		s.Logf("Gamepad RTC latency: mean %f median %f std %f max %f min %f", rmean, rmedian, rstdDev, rmax, rmin)

		pv.Set(perf.Metric{
			Name:      perfName,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, mean)
		return nil
	}

	s.Log("Injecting one button key event each time")
	const repeat = 25
	eventTimes := make([]int64, 0, repeat*2)
	for i := 0; i < repeat; i++ {
		if err := generateNextEventTime(&eventTimes); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		if err := gp.PressButton(ctx, input.BTN_EAST); err != nil {
			s.Fatal("Failed to inject key event: ", err)
		}

		if err := generateNextEventTime(&eventTimes); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}

		if err := gp.ReleaseButton(ctx, input.BTN_EAST); err != nil {
			s.Fatal("Failed to inject key event: ", err)
		}
	}

	pv := perf.NewValues()

	if err := evaluate(repeat*2, &eventTimes, "avgGamepadButtonLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := clearUI(); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}

	s.Log("Injecting one joystick event each time")
	eventTimes = make([]int64, 0, repeat*2)
	axis := gp.Axes()[input.ABS_X]
	for i := 0; i < repeat; i++ {
		if err := generateNextEventTime(&eventTimes); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// Move axis x to maximum.
		if err := gp.MoveAxis(ctx, input.ABS_X, axis.Maximum); err != nil {
			s.Fatal("Failed to move axis: ", err)
		}

		if err := generateNextEventTime(&eventTimes); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// Move axis x to minimum.
		if err := gp.MoveAxis(ctx, input.ABS_X, axis.Minimum); err != nil {
			s.Fatal("Failed to move axis: ", err)
		}
	}

	if err := evaluate(repeat*2, &eventTimes, "avgGamepadStickLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := clearUI(); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}

	s.Log("Injecting one button key event and one axis event together each time")
	eventTimes = make([]int64, 0, repeat*4)
	axis = gp.Axes()[input.ABS_X]
	pressEvents := []input.GamepadEvent{
		{Et: input.EV_ABS, Ec: input.ABS_X, Val: axis.Maximum},
		{Et: input.EV_KEY, Ec: input.BTN_EAST, Val: 1}}
	releaseEvents := []input.GamepadEvent{
		{Et: input.EV_ABS, Ec: input.ABS_X, Val: axis.Minimum},
		{Et: input.EV_KEY, Ec: input.BTN_EAST, Val: 0}}
	for i := 0; i < repeat; i++ {
		// Generate event time for pressing button.
		if err := generateNextEventTime(&eventTimes); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// Same event time for pressing button and moving joystick.
		eventTimes = append(eventTimes, eventTimes[len(eventTimes)-1])
		if err := gp.PressButtonsAndAxes(ctx, pressEvents); err != nil {
			s.Fatal("Failed to inject key event: ", err)
		}

		// Generate event time for releasing button.
		if err := generateNextEventTime(&eventTimes); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// Same event time for release button and moving joystick.
		eventTimes = append(eventTimes, eventTimes[len(eventTimes)-1])
		if err := gp.PressButtonsAndAxes(ctx, releaseEvents); err != nil {
			s.Fatal("Failed to release button: ", err)
		}
	}

	if err := evaluate(repeat*4, &eventTimes, "avgGamepadMixLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to saving perf data: ", err)
	}
}
