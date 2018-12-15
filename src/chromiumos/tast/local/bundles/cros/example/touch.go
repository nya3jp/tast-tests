package example

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Touch,
		Desc: "Demonstrates injecting touch events",
		// TODO(derat): Remove "disabled" if/when there's a way to depend on an internal keyboard.
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func Touch(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer tconn.Close()

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("No display: ", err)
	}

	// TODO(ricardoq): This page might change/break in the future. If so, a built-in
	// HTML page that accepts drawing should be used. Addtionally, Kleki seems to ignore
	// the 2nd & last events when drawing splines. But for the purpose of showing how
	// to use the API is good enough.
	conn, err := cr.NewConn(ctx, "http://kleki.com")
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		s.Fatal("Timed out waiting for page load: ", err)
	}

	s.Log("Finding and opening touchscreen device")
	// It is possible to send raw events to the Touchscreen type. But it recommended to
	// use the Touchscreen.TouchEvent() struct since it already has functions to manipulate
	// Touch events.
	ew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device: ", err)
	}
	defer ew.Close()

	// Touchscreen bounds: The size of the touchscreen might not be the same
	// as the display size. In fact, might be even up to 4x bigger.
	infoX, infoY := ew.GetEventAbsInfo()
	touchWidth := infoX.GetMaximum() - infoX.GetMinimum()
	touchHeight := infoY.GetMaximum() - infoY.GetMinimum()

	// Display bounds
	displayWidth := info.Bounds.Width
	displayHeight := info.Bounds.Height

	pixelToTouchFactorX := float64(touchWidth) / float64(displayWidth)
	pixelToTouchFactorY := float64(touchHeight) / float64(displayHeight)

	// Generates some touch events
	startX := 250 * pixelToTouchFactorX
	startY := 250 * pixelToTouchFactorY

	// Draw a dotted line:
	// TouchEvent is being reused for the 5 dots. The event is "ended" after each touch.
	// Thus generating a "dotted" (instead of continuos) line.
	te, err := ew.NewWriter()
	if err != nil {
		s.Fatal("Could not get a new TouchEventWriter")
	}
	for i := 0; i < 5; i++ {
		// TouchEvent API expects values in "touchscreen coordinates", not pixel coordinates.
		te.TouchAt(int32(startX+float64(i)*100.0), int32(startY+float64(i)*100.0))
		te.End()
		sleepMilliseconds(ctx, 100)
	}

	// Draw a circle:
	// Draws a circle with 40 TouchEvents.
	// By reusing the same event without "ending" it, a continous line is generated.
	te, err = ew.NewWriter()
	if err != nil {
		s.Fatal("Could not create TouchEventWriter")
	}
	centerX := int32(400 * pixelToTouchFactorX)
	centerY := int32(400 * pixelToTouchFactorY)
	const radius = 300 // in pixels
	pressure := int32(60)
	for v := range circleIter(40) {
		x := int32(v.x * radius * pixelToTouchFactorX)
		y := int32(v.y * radius * pixelToTouchFactorY)

		// It is possible to change the pressure for each event. Although certain applications might ignore it.
		te.SetAbsPressure(pressure)
		pressure++
		te.TouchAt(centerX+x, centerY+y)
		sleepMilliseconds(ctx, 100)
	}
	// And finally "end" the continue line
	te.End()

	// TODO(ricardoq): Add MultiTouch example.
	// By creating two (or more) TouchEvents it is possible to generate
	// multi-touch events. This is possible since each event has its own multitouch id.
}

type vec2f struct {
	x float64
	y float64
}

func circleIter(segments int) <-chan vec2f {
	ch := make(chan vec2f)

	go func() {
		coef := (2.0 * math.Pi) / float64(segments)
		for i := 0; i <= segments; i++ {
			rads := float64(i) * coef
			ch <- vec2f{math.Cos(rads), math.Sin(rads)}
		}
		close(ch)
	}()
	return ch
}

func sleepMilliseconds(ctx context.Context, ms time.Duration) error {
	select {
	case <-time.After(ms * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}
	return errors.New("timeout")
}
