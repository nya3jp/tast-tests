package example

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Input,
		Desc: "Demonstrates injecting keyboard events",
		// TODO(derat): Remove "disabled" if/when there's a way to depend on an internal keyboard.
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func Input(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in: ", err)
	}
	defer cr.Close(ctx)

	keyboardExamples(ctx, s, cr)
	touchscreenExamples(ctx, s, cr)
}

func keyboardExamples(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	const (
		html        = "<!DOCTYPE html><input id='text' type='text' autofocus>"
		elementExpr = "document.getElementById('text')"
		valueExpr   = elementExpr + ".value"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	s.Log("Loading input page")
	conn, err := cr.NewConn(ctx, server.URL)
	if err != nil {
		s.Fatal("Creating renderer failed: ", err)
	}
	defer conn.Close()

	// getText waits for expr to evaluate to a string of the given length and returns the string.
	getText := func(expr string, length int) (string, error) {
		s.Log("Waiting for text from ", expr)
		if err := conn.WaitForExpr(ctx, fmt.Sprintf("%s.length === %d", expr, length)); err != nil {
			return "", errors.Wrapf(err, "waiting for %s failed", expr)
		}
		var actual string
		if err := conn.Eval(ctx, expr, &actual); err != nil {
			return "", errors.Wrapf(err, "evaluating %s failed", expr)
		}
		s.Logf("Got text %q from %s", actual, expr)
		return actual, nil
	}

	s.Log("Waiting for focus")
	if err := conn.WaitForExpr(ctx, elementExpr+" === document.activeElement"); err != nil {
		s.Fatal("Failed waiting for focus: ", err)
	}

	s.Log("Finding and opening keyboard device")
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer ew.Close()

	const inputText = "Hello, world!"
	s.Logf("Injecting keyboard events for %q", inputText)
	if err = ew.Type(ctx, inputText); err != nil {
		s.Fatal("Failed to write events: ", err)
	}
	// TODO(derat): The text typed above seems to sometimes not show up; try to figure out why.
	// Maybe there's a small delay within Blink between document.activeElement being updated and keyboard
	// events actually being directed to the element.
	if actual, err := getText(valueExpr, len(inputText)); err != nil {
		s.Error("Failed to get input text (this can be flaky): ", err)
	} else if actual != inputText {
		s.Errorf("Got input text %q; typed %q (non-QWERTY layout or Caps Lock?)", actual, inputText)
	}

	const (
		pageText = "mittens"
		dataURL  = "data:text/plain," + pageText
		bodyExpr = "document.body.innerText"
	)
	s.Logf("Navigating to %q via omnibox", dataURL)
	ew.Accel(ctx, "Ctrl+L")
	ew.Type(ctx, dataURL+"\n")
	if actual, err := getText(bodyExpr, len(pageText)); err != nil {
		s.Error("Failed to get page text: ", err)
	} else if actual != pageText {
		s.Errorf("Got page text %q; want %q", actual, pageText)
	}
}

func touchscreenExamples(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
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
	// HTML page that accepts drawing should be used. Addtionally, Kleki seems to
	// the 2nd + last events when drawing splines. But for the purpose of showing how
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
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer ew.Close()

	// Touchscreen bounds: The size of the touchscreen might not be the same
	// as the display size. In fact, might be even up to 4x bigger.
	var touchWidth = ew.TouchInfoX.Maximum
	var touchHeight = ew.TouchInfoY.Maximum
	// Display bounds
	var displayWidth = info.Bounds.Width
	var displayHeight = info.Bounds.Height

	pixelToTouchFactorX := float64(touchWidth) / float64(displayWidth)
	pixelToTouchFactorY := float64(touchHeight) / float64(displayHeight)

	// Generates some touch events
	startX := 250 * pixelToTouchFactorX
	startY := 250 * pixelToTouchFactorY

	// Draw dotted a line:
	// TouchEvent is being resued for the 5 dots. The event is "ended" after each touch.
	// Thus generating a "dotted" (instead of continuos) line.
	te, _ := ew.TouchEvent()
	for i := 0; i < 5; i++ {
		// TouchEvent API expects values in "touchscreen coordinates", not pixel coordinates.
		te.MoveTo(int32(startX+float64(i)*100.0),
			int32(startY+float64(i)*100.0))
		te.End()
		sleepMilliseconds(ctx, 100)
	}

	// Draw a circle:
	// Draws a circle with 40 TouchEvents.
	// By reusing the same event without "ending" it, a continous line is generated.
	te, _ = ew.TouchEvent()
	ch := make(chan vec2f)
	go circleGenerator(40, ch)
	centerX := int32(400 * pixelToTouchFactorX)
	centerY := int32(400 * pixelToTouchFactorY)
	const radius = 300 // in pixels
	pressure := int32(60)
	for v := range ch {
		x := int32(v.x * radius * pixelToTouchFactorX)
		y := int32(v.y * radius * pixelToTouchFactorY)

		// It is possible to change the pressure for each event. Although certain applications might ignore it.
		te.AbsPressure = pressure
		pressure++
		te.MoveTo(centerX+x, centerY+y)
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

func circleGenerator(segments int, c chan vec2f) {
	coef := (2.0 * math.Pi) / float64(segments)
	for i := 0; i <= segments; i++ {
		rads := float64(i) * coef
		c <- vec2f{math.Cos(rads), math.Sin(rads)}
	}
	close(c)
}

func sleepMilliseconds(ctx context.Context, ms time.Duration) error {
	select {
	case <-time.After(ms * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}
	return errors.New("timeout")
}
