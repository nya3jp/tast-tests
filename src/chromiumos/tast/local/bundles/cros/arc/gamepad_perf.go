// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
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

	s.Log("Injecting one button key event each time")
	const repeat = 25
	const waitMS = 50
	eventTimes := make([]int64, 0, repeat*2)
	for i := 0; i < repeat; i++ {
		if err := inputlatency.WaitForNextEventTime(ctx, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		if err := gp.PressButton(ctx, input.BTN_EAST); err != nil {
			s.Fatal("Failed to inject key event: ", err)
		}

		if err := inputlatency.WaitForNextEventTime(ctx, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}

		if err := gp.ReleaseButton(ctx, input.BTN_EAST); err != nil {
			s.Fatal("Failed to inject key event: ", err)
		}
	}

	pv := perf.NewValues()

	if err := inputlatency.EvaluateLatency(ctx, s, d, repeat*2, &eventTimes, "avgGamepadButtonLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := inputlatency.WaitForClearUI(ctx, d); err != nil {
		s.Fatal("Failed to clear UI: ", err)
	}

	s.Log("Injecting one joystick event each time")
	eventTimes = make([]int64, 0, repeat*2)
	axis := gp.Axes()[input.ABS_X]
	for i := 0; i < repeat; i++ {
		if err := inputlatency.WaitForNextEventTime(ctx, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// Move axis x to maximum.
		if err := gp.MoveAxis(ctx, input.ABS_X, axis.Maximum); err != nil {
			s.Fatal("Failed to move axis: ", err)
		}

		if err := inputlatency.WaitForNextEventTime(ctx, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// Move axis x to minimum.
		if err := gp.MoveAxis(ctx, input.ABS_X, axis.Minimum); err != nil {
			s.Fatal("Failed to move axis: ", err)
		}
	}

	if err := inputlatency.EvaluateLatency(ctx, s, d, repeat*2, &eventTimes, "avgGamepadStickLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := inputlatency.WaitForClearUI(ctx, d); err != nil {
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
		if err := inputlatency.WaitForNextEventTime(ctx, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// Same event time for pressing button and moving joystick.
		eventTimes = append(eventTimes, eventTimes[len(eventTimes)-1])
		if err := gp.PressButtonsAndAxes(ctx, pressEvents); err != nil {
			s.Fatal("Failed to inject key event: ", err)
		}

		// Generate event time for releasing button.
		if err := inputlatency.WaitForNextEventTime(ctx, &eventTimes, waitMS); err != nil {
			s.Fatal("Failed to generate event time: ", err)
		}
		// Same event time for release button and moving joystick.
		eventTimes = append(eventTimes, eventTimes[len(eventTimes)-1])
		if err := gp.PressButtonsAndAxes(ctx, releaseEvents); err != nil {
			s.Fatal("Failed to release button: ", err)
		}
	}

	if err := inputlatency.EvaluateLatency(ctx, s, d, repeat*4, &eventTimes, "avgGamepadMixLatency", pv); err != nil {
		s.Fatal("Failed to evaluate: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to saving perf data: ", err)
	}
}
