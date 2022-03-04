package utils

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/motioninput"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// r99 code changed, need to pay attention
// refer to mouse_input.go
//	notice: need to add this {Fixture: "arcBooted"}
func VerifyMouse(ctx context.Context, s *testing.State) error {
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

	if err := a.Install(ctx, arc.APKPath(motioninput.APK)); err != nil {
		s.Fatal("Failed installing ", motioninput.APK, ": ", err)
	}

	for _, params := range []motioninput.WMTestParams{
		{
			Name:          "Clamshell fullscreen",
			TabletMode:    false,
			WmEventToSend: ash.WMEventFullscreen,
		},
	} {
		s.Run(ctx, params.Name+": Verify Mouse", func(ctx context.Context, s *testing.State) {
			motioninput.RunTestWithWMParams(ctx, s, tconn, d, a, &params, verifyMouse)
		})
	}

	return nil
}

// mouseMatcher returns a motionEventMatcher that matches events from a Mouse device.
func mouseMatcher(a motioninput.Action, p coords.Point) motioninput.Matcher {
	pressure := 0.
	if a == motioninput.ActionMove || a == motioninput.ActionDown || a == motioninput.ActionButtonPress || a == motioninput.ActionHoverExit {
		pressure = 1.
	}
	return motioninput.SinglePointerMatcher(a, motioninput.SourceMouse, p, pressure)
}

// verifyMouse tests the behavior of mouse events injected into Ash on Android apps. It tests hover,
// button, and drag events. It does not use the uinput mouse to inject events because the scale
// relation between the relative movements injected by a relative mouse device and the display
// pixels is determined by ChromeOS and could vary between devices.
func verifyMouse(ctx context.Context, s *testing.State, tconn *chrome.TestConn, t *motioninput.WMTestState, tester *motioninput.Tester) {

	s.Log("Verifying Mouse")

	p := t.CenterOfWindow()
	e := t.ExpectedPoint(p)

	s.Log("Injected initial move, waiting... ")
	if err := mouse.Move(tconn, p, 0)(ctx); err != nil {
		s.Fatalf("Failed to inject move at %v: %v", e, err)
	}

	if err := tester.ClearMotionEvents(ctx); err != nil {
		s.Fatal("Failed to clear events: ", err)
	}

	p = coords.NewPoint(0, 0)
	e = t.ExpectedPoint(p)

	s.Log("Verifying mouse move event at ", e)
	if err := mouse.Move(tconn, p, 0)(ctx); err != nil {
		s.Fatalf("Failed to inject move at %v: %v", e, err)
	}

	if err := tester.ExpectEventsAndClear(ctx, mouseMatcher(motioninput.ActionHoverMove, e)); err != nil {
		s.Fatal("Failed to expect events and clear: ", err)
	}

	return

}
