// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arc/motioninput"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PointerCapture,
		Desc:         "Checks that Pointer Capture works in Android",
		Contacts:     []string{"prabirmsp@chromium.org", "arc-framework@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Pre:          arc.Booted(),
	})
}

// PointerCapture tests the Android Pointer Capture API support on ChromeOS. It uses a test
// application that requests Pointer Capture and verifies the relative movements the app receives
// when injecting events into ChromeOS through a uinput mouse.
// More about Pointer Capture: https://developer.android.com/training/gestures/movement#pointer-capture
func PointerCapture(ctx context.Context, s *testing.State) {
	p := s.PreValue().(arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close()

	s.Log("Installing apk ", motioninput.APK)
	if err := a.Install(ctx, arc.APKPath(motioninput.APK)); err != nil {
		s.Fatalf("Failed installing %s: %v", motioninput.APK, err)
	}

	runSubtest := func(ctx context.Context, s *testing.State, subtestFunc pointerCaptureSubtestFunc) {
		test := pointerCaptureSubtestState{}

		act, err := arc.NewActivity(a, motioninput.Package, motioninput.PointerCaptureActivity)
		if err != nil {
			s.Fatal("Failed to create an activity: ", err)
		}
		defer act.Close()

		if err := act.Start(ctx, tconn); err != nil {
			s.Fatal("Failed to start an activity: ", err)
		}
		defer act.Stop(ctx, tconn)

		if err := ash.WaitForVisible(ctx, tconn, motioninput.Package); err != nil {
			s.Fatal("Failed to wait for activity to be visible: ", err)
		}

		test.mew, err = input.Mouse(ctx)
		if err != nil {
			s.Fatal("Failed to create mouse device: ", err)
		}
		defer test.mew.Close()

		s.Log("Enabling pointer capture")
		if err := enablePointerCapture(ctx, tconn); err != nil {
			s.Fatal("Failed to enable pointer capture: ", err)
		}

		if err := expectPointerCaptureEnabled(ctx, d); err != nil {
			s.Fatal("Failed to verify that pointer capture is enabled: ", err)
		}

		test.tester = motioninput.NewTester(tconn, d, act)
		if err := test.tester.ClearMotionEvents(ctx); err != nil {
			s.Fatal("Failed to clear events: ", err)
		}

		subtestFunc(ctx, s, test)
	}

	for _, subtest := range []struct {
		Name string
		Func pointerCaptureSubtestFunc
	}{
		{
			Name: "Pointer Capture is not restricted by display bounds",
			Func: verifyPointerCaptureBounds,
		}, {
			Name: "Pointer Capture buttons",
			Func: verifyPointerCaptureButtons,
		},
	} {
		s.Run(ctx, subtest.Name, func(ctx context.Context, s *testing.State) {
			runSubtest(ctx, s, subtest.Func)
		})
	}
}

type pointerCaptureState struct {
	Enabled bool `json:"pointer_capture_enabled"`
}

// enablePointerCapture clicks at the center of the test application, which will make the test app
// trigger Pointer Capture.
func enablePointerCapture(ctx context.Context, tconn *chrome.TestConn) error {
	// Click on the capture_view using the ui mouse. This ensures that the Ash window is in focus.
	// We cannot use UI Automator to click on the capture_view because that does not guarantee the
	// window is in focus in Ash as there could be something like a pop-up notification that
	// actually has focus.
	w, err := ash.GetARCAppWindowInfo(ctx, tconn, motioninput.Package)
	if err != nil {
		return errors.Wrap(err, "failed to get ARC app window info")
	}

	center := w.BoundsInRoot.CenterPoint()
	if err := mouse.Click(ctx, tconn, center, mouse.LeftButton); err != nil {
		return errors.Wrap(err, "failed to click in the app window to enable pointer capture")
	}
	return nil
}

// readPointerCaptureState unmarshalls the JSON string in the TextView representing the
// Pointer Capture state in the test application.
func readPointerCaptureState(ctx context.Context, d *ui.Device) (*pointerCaptureState, error) {
	view := d.Object(ui.ID(motioninput.Package + ":id/pointer_capture_state"))
	if err := view.WaitForExists(ctx, 5*time.Second); err != nil {
		return nil, err
	}
	text, err := view.GetText(ctx)
	if err != nil {
		return nil, err
	}

	var state pointerCaptureState
	if err := json.Unmarshal([]byte(text), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// expectPointerCaptureEnabled polls readPointerCaptureState repeatedly until Pointer Capture is
// enabled.
func expectPointerCaptureEnabled(ctx context.Context, d *ui.Device) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		state, err := readPointerCaptureState(ctx, d)
		if err != nil {
			return err
		}
		if !state.Enabled {
			return errors.New("expected Pointer Capture to be enabled, but was disabled")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// pointerCaptureSubtestState holds values that are initialized to be used by the subtests.
type pointerCaptureSubtestState struct {
	tester *motioninput.Tester
	mew    *input.MouseEventWriter
}

// pointerCaptureSubtestFunc represents a subtest function.
type pointerCaptureSubtestFunc func(ctx context.Context, s *testing.State, t pointerCaptureSubtestState)

// verifyPointerCaptureBounds is a subtest that verifies mouse movement is not restricted by the
// bounds of the display, since only relative movements are reported when Pointer Capture is
// enabled. This is tested by injecting a large number of relative mouse movements in a single
// direction.
func verifyPointerCaptureBounds(ctx context.Context, s *testing.State, t pointerCaptureSubtestState) {
	delta := coords.NewPoint(-50, -50)
	var pendingMatchers []motioninput.Matcher

	verifyAndClearInjectedEvents := func() {
		if err := t.tester.ExpectEventsAndClear(ctx, pendingMatchers...); err != nil {
			s.Fatal("Failed to verify motion event and clear: ", err)
		}
		if err := t.tester.ClearMotionEvents(ctx); err != nil {
			s.Fatal("Failed to clear events: ", err)
		}
		pendingMatchers = pendingMatchers[:0]
	}

	for i := 0; i < 100; i++ {
		if err := t.mew.Move(int32(delta.X), int32(delta.Y)); err != nil {
			s.Fatalf("Failed to move mouse by (%d, %d): %v", delta.X, delta.Y, err)
		}
		// We only verify the action and source of each event and not the magnitude of the movements
		// because ChromeOS applies mouse acceleration to such large and frequent movements.
		pendingMatchers = append(pendingMatchers, motioninput.ActionSourceMatcher(motioninput.ActionMove, motioninput.SourceMouseRelative))

		// TODO(b/156655077): Remove input event injection throttling after the bug is resolved.
		//  eventFrequencyHz is the experimentally determined frequency for input injection so that
		//  multiple ACTION_MOVE events are not batched together by Android. We skip processing of
		//  batched events so that we do not count extraneous events generated in the input pipeline.
		//  See the bug for more details.
		const eventFrequencyHz = 30
		if err := testing.Sleep(ctx, time.Second/eventFrequencyHz); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		if i%10 == 0 {
			verifyAndClearInjectedEvents()
		}
	}
	verifyAndClearInjectedEvents()
}

// verifyPointerCaptureButtons is a subtest that ensures mouse button functionality when Pointer
// Capture  is enabled.
func verifyPointerCaptureButtons(ctx context.Context, s *testing.State, t pointerCaptureSubtestState) {
	if err := t.mew.Click(); err != nil {
		s.Fatal("Failed to click mouse button: ", err)
	}
	matcher := func(a motioninput.Action, pressure float64) motioninput.Matcher {
		return motioninput.SinglePointerMatcher(a, motioninput.SourceMouseRelative, coords.NewPoint(0, 0), pressure)
	}
	if err := t.tester.ExpectEventsAndClear(ctx, matcher(motioninput.ActionDown, 1), matcher(motioninput.ActionButtonPress, 1), matcher(motioninput.ActionButtonRelease, 0), matcher(motioninput.ActionUp, 0)); err != nil {
		s.Fatal("Failed to clear motion events and clear: ", err)
	}
}
