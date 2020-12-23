// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/json"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
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
		Contacts:     []string{"prabirmsp@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
	})
}

// PointerCapture tests the Android Pointer Capture API support on ChromeOS. It uses a test
// application that requests Pointer Capture and verifies the relative movements the app receives
// when injecting events into ChromeOS through a uinput mouse.
// More about Pointer Capture: https://developer.android.com/training/gestures/movement#pointer-capture
func PointerCapture(ctx context.Context, s *testing.State) {
	p := s.FixtValue().(*arc.PreData)
	cr := p.Chrome
	a := p.ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	s.Log("Installing apk ", motioninput.APK)
	if err := a.Install(ctx, arc.APKPath(motioninput.APK)); err != nil {
		s.Fatalf("Failed installing %s: %v", motioninput.APK, err)
	}

	runSubtest := func(ctx context.Context, s *testing.State, subtestFunc pointerCaptureSubtestFunc) {
		test := pointerCaptureSubtestState{}
		test.arc = a
		test.tconn = tconn
		test.d = d

		act, err := arc.NewActivity(a, motioninput.Package, motioninput.AutoPointerCaptureActivity)
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

		if err := expectPointerCaptureState(ctx, d, true); err != nil {
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
			Name: "Pointer Capture sends relative movements",
			Func: verifyPointerCaptureRelativeMovement,
		}, {
			Name: "Pointer Capture is not restricted by display bounds",
			Func: verifyPointerCaptureBounds,
		}, {
			Name: "Pointer Capture buttons",
			Func: verifyPointerCaptureButtons,
		}, {
			Name: "Pointer Capture is disabled when Chrome is focused",
			Func: verifyPointerCaptureDisabledWhenChromeFocused,
		}, {
			Name: "Pointer Capture is re-enabled after switching focus with the keyboard",
			Func: verifyPointerCaptureWithKeyboardFocusChange,
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

// expectPointerCaptureState polls readPointerCaptureState repeatedly until Pointer Capture is
// equal to the expected value.
func expectPointerCaptureState(ctx context.Context, d *ui.Device, enabled bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		state, err := readPointerCaptureState(ctx, d)
		if err != nil {
			return err
		}
		if state.Enabled != enabled {
			return errors.Errorf("unexpected Pointer Capture state: want: %t, got: %t", enabled, state.Enabled)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// pointerCaptureSubtestState holds values that are initialized to be used by the subtests.
type pointerCaptureSubtestState struct {
	tester *motioninput.Tester
	mew    *input.MouseEventWriter
	arc    *arc.ARC
	tconn  *chrome.TestConn
	d      *ui.Device
}

// pointerCaptureSubtestFunc represents a subtest function.
type pointerCaptureSubtestFunc func(ctx context.Context, s *testing.State, t pointerCaptureSubtestState)

// ensureRelativeMovement is a helper function that injects a relative mouse movements through a uinput
// mouse device and checks that a relative movement was sent to the application.
func ensureRelativeMovement(ctx context.Context, t pointerCaptureSubtestState, delta coords.Point) error {
	if err := t.mew.Move(int32(delta.X), int32(delta.Y)); err != nil {
		return errors.Wrapf(err, "failed to move mouse by (%d, %d)", delta.X, delta.Y)
	}
	// We only verify the action and source of each event and not the magnitude of the movements
	// because ChromeOS applies mouse acceleration which changes the magnitude.
	matcher := motioninput.ActionSourceMatcher(motioninput.ActionMove, motioninput.SourceMouseRelative)

	if err := t.tester.ExpectEventsAndClear(ctx, matcher); err != nil {
		return errors.Wrap(err, "failed to verify motion event and clear")
	}
	if err := t.tester.ClearMotionEvents(ctx); err != nil {
		return errors.Wrap(err, "failed to clear events")
	}
	return nil
}

// verifyPointerCaptureRelativeMovement is a subtest that verifies that mouse movements injected when Pointer
// Capture is enabled are sent to the app as relative movements.
func verifyPointerCaptureRelativeMovement(ctx context.Context, s *testing.State, t pointerCaptureSubtestState) {
	if err := ensureRelativeMovement(ctx, t, coords.NewPoint(10, 10)); err != nil {
		s.Fatal("Failed to verify relative movement: ", err)
	}
}

// verifyPointerCaptureBounds is a subtest that verifies mouse movement is not restricted by the
// bounds of the display, since only relative movements are reported when Pointer Capture is
// enabled. This is tested by injecting a large number of relative mouse movements in a single
// direction.
func verifyPointerCaptureBounds(ctx context.Context, s *testing.State, t pointerCaptureSubtestState) {
	delta := coords.NewPoint(-100, -100)

	for i := 0; i < 20; i++ {
		if err := ensureRelativeMovement(ctx, t, delta); err != nil {
			s.Fatal("Failed to verify relative movement: ", err)
		}
	}
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

// verifyPointerCaptureDisabledWhenChromeFocused is a subtest that ensures Pointer Capture is disabled when
// a Chrome window comes into focus.
func verifyPointerCaptureDisabledWhenChromeFocused(ctx context.Context, s *testing.State, t pointerCaptureSubtestState) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Press the search key to bring the launcher into focus.
	if err := kb.Accel(ctx, "Search"); err != nil {
		s.Fatal("Failed to press Search: ", err)
	}

	// Pointer Capture should be disabled when window loses focus.
	if err := expectPointerCaptureState(ctx, t.d, false); err != nil {
		s.Fatal("Failed to verify that pointer capture is disabled: ", err)
	}

	// Press the search key again to hide the launcher.
	if err := kb.Accel(ctx, "Search"); err != nil {
		s.Fatal("Failed to press Search: ", err)
	}

	// Pointer Capture should be enabled when window gains focus.
	if err := expectPointerCaptureState(ctx, t.d, true); err != nil {
		s.Fatal("Failed to verify that pointer capture is enabled: ", err)
	}
	// Clear events, since hover events could have been generated before Pointer Capture was re-enabled.
	if err := t.tester.ClearMotionEvents(ctx); err != nil {
		s.Fatal("Failed to clear events: ", err)
	}
	if err := ensureRelativeMovement(ctx, t, coords.NewPoint(10, 10)); err != nil {
		s.Fatal("Failed to verify relative movement: ", err)
	}
}

// verifyPointerCaptureWithKeyboardFocusChange is a subtest that ensures Pointer Capture is disabled when
// the activity loses focus and re-gains Pointer Capture when it is focused again using the keyboard.
func verifyPointerCaptureWithKeyboardFocusChange(ctx context.Context, s *testing.State, t pointerCaptureSubtestState) {
	// Launch the settings activity to make the Pointer Capture Activity lose focus.
	const (
		settingsPackage  = "com.android.settings"
		settingsActivity = ".Settings"
	)
	act, err := arc.NewActivity(t.arc, settingsPackage, settingsActivity)
	if err != nil {
		s.Fatal("Failed to create an activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, t.tconn); err != nil {
		s.Fatal("Failed to start an activity: ", err)
	}
	defer act.Stop(ctx, t.tconn)

	if err := ash.WaitForVisible(ctx, t.tconn, settingsPackage); err != nil {
		s.Fatal("Failed to wait for activity to be visible: ", err)
	}

	// Pointer Capture should be disabled when window loses focus.
	if err := expectPointerCaptureState(ctx, t.d, false); err != nil {
		s.Fatal("Failed to verify that pointer capture is disabled: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	if err := kb.Accel(ctx, "Alt+Tab"); err != nil {
		s.Fatal("Failed to press Alt+Tab to switch windows: ", err)
	}

	// The activity will automatically request pointer capture when it gains focus.
	if err := expectPointerCaptureState(ctx, t.d, true); err != nil {
		s.Fatal("Failed to verify that pointer capture is enabled: ", err)
	}
	// Clear events, since hover events could have been generated before Pointer Capture was re-enabled.
	if err := t.tester.ClearMotionEvents(ctx); err != nil {
		s.Fatal("Failed to clear events: ", err)
	}
	if err := ensureRelativeMovement(ctx, t, coords.NewPoint(10, 10)); err != nil {
		s.Fatal("Failed to verify relative movement: ", err)
	}
}
