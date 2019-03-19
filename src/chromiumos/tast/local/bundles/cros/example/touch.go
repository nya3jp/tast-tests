// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE

package example

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Touch,
		Desc:     "Demonstrates injecting touch events",
		Contacts: []string{"ricardoq@chromium.org", "tast-users@chromium.org"},
		// TODO(derat): Remove "disabled" if/when there's a way to depend on an internal keyboard.
		Attr:         []string{"disabled", "informational"},
		SoftwareDeps: []string{"chrome_login"},
		Pre:          chrome.LoggedIn(),
	})
}

func Touch(ctx context.Context, s *testing.State) {
	const (
		radius   = 200 // circle radius in pixels
		segments = 120 // segments used for the circle
		steps    = 100 // steps used in swipe
	)

	must := func(err error) {
		if err != nil {
			s.Fatal("Failed: ", err)
		}
	}

	cr := s.PreValue().(*chrome.Chrome)
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
	// HTML page that accepts drawing should be used. Additionally, Kleki seems to ignore
	// the 2nd & last events when drawing splines. But for the purpose of showing how
	// to use the API is good enough.
	conn, err := cr.NewConn(ctx, "http://kleki.com")
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}
	defer conn.Close()

	if err := conn.WaitForExpr(ctx, "document.getElementsByTagName('canvas').length > 0"); err != nil {
		s.Fatal("Timed out waiting for page load: ", err)
	}

	s.Log("Finding and opening touchscreen device")
	// It is possible to send raw events to the Touchscreen type. But it is recommended to
	// use the Touchscreen.TouchEventWriter struct since it already has functions to manipulate
	// Touch events.
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device: ", err)
	}
	defer tsw.Close()

	// Touchscreen bounds: The size of the touchscreen might not be the same
	// as the display size. In fact, might be even up to 4x bigger.
	touchWidth := tsw.Width()
	touchHeight := tsw.Height()

	// Display bounds
	displayWidth := float64(info.Bounds.Width)
	displayHeight := float64(info.Bounds.Height)

	pixelToTouchFactorX := float64(touchWidth) / displayWidth
	pixelToTouchFactorY := float64(touchHeight) / displayHeight

	centerX := displayWidth * pixelToTouchFactorX / 2
	centerY := displayHeight * pixelToTouchFactorY / 2

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Could not get a new TouchEventWriter: ", err)
	}
	defer stw.Close()

	// Draw a dotted line:
	// SingleTouchEventWriter is being reused for the 15 dots. The event is "ended" after each touch.
	// "End" is equivalent as lifting the finger from the touchscreen.
	// Thus generating a "dotted" line, instead of continuos one.
	for i := 0; i < 15; i++ {
		// Values must be in "touchscreen coordinates", not pixel coordinates.
		stw.Move(input.TouchCoord(centerX+float64(i)*50.0), input.TouchCoord(centerY+float64(i)*50.0))
		stw.End()
		sleep(ctx, 100*time.Millisecond)
	}

	// Delay to make the test visually more pleasing.
	must(sleep(ctx, 500*time.Millisecond))

	// Draw a circle:
	// Draws a circle with 120 touch events. The touch event is moved to
	// 120 different locations generating a continuous circle.
	stw, err = tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Could not create TouchEventWriter: ", err)
	}
	defer stw.Close()

	for i := 0; i < segments; i++ {
		rads := 2.0*math.Pi*(float64(i)/segments) + math.Pi
		x := radius * pixelToTouchFactorX * math.Cos(rads)
		y := radius * pixelToTouchFactorY * math.Sin(rads)
		stw.Move(input.TouchCoord(centerX+x), input.TouchCoord(centerY+y))
		must(sleep(ctx, 15*time.Millisecond))
	}
	// And finally "end" (lift the finger) the line.
	stw.End()

	// Delay to make the test visually more pleasing.
	must(sleep(ctx, 500*time.Millisecond))

	// Swipe test:
	// Draw a square using 4 swipes.
	stw, err = tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Could not create TouchEventWriter: ", err)
	}
	defer stw.Close()
	for _, d := range []struct {
		x0, y0, x1, y1 float64
	}{
		{-1, 1, -1, -1}, // swipe up form bottom-left
		{-1, -1, 1, -1}, // swipe right from top-left
		{1, -1, 1, 1},   // swipe down from top-right
		{1, 1, -1, 1},   // swipe left from bottom-right
	} {
		x0 := input.TouchCoord(centerX + radius*d.x0*pixelToTouchFactorX)
		y0 := input.TouchCoord(centerY + radius*d.y0*pixelToTouchFactorY)
		x1 := input.TouchCoord(centerX + radius*d.x1*pixelToTouchFactorX)
		y1 := input.TouchCoord(centerY + radius*d.y1*pixelToTouchFactorY)

		if err := stw.Swipe(ctx, x0, y0, x1, y1, steps); err != nil {
			s.Error("Failed to run Swipe: ", err)
		}
	}
	if err := stw.End(); err != nil {
		s.Error("Failed to finish the swipe gesture: ", err)
	}

	// Delay to make the test visually more pleasing.
	must(sleep(ctx, 500*time.Millisecond))

	// Multitouch test: Zoom out + zoom in
	// Get a multitouch writer for two touches.
	mtw, err := tsw.NewMultiTouchWriter(2)
	if err != nil {
		s.Fatal("Could not get a new TouchEventWriter: ", err)
	}
	defer mtw.Close()

	// Get the individual TouchState
	ts0 := mtw.TouchState(0)
	ts1 := mtw.TouchState(1)

	// Zoom out
	for i := 5; i < 100; i++ {
		deltaX := float64(i) * pixelToTouchFactorX
		deltaY := float64(i) * pixelToTouchFactorY

		ts0.SetPos(input.TouchCoord(centerX-deltaX), input.TouchCoord(centerY-deltaY))
		ts1.SetPos(input.TouchCoord(centerX+deltaX), input.TouchCoord(centerY+deltaY))
		mtw.Send()
		must(sleep(ctx, 15*time.Millisecond))
	}

	// Zoom in
	for i := 100; i > 15; i-- {
		deltaX := float64(i) * pixelToTouchFactorX
		deltaY := float64(i) * pixelToTouchFactorY

		ts0.SetPos(input.TouchCoord(centerX-deltaX), input.TouchCoord(centerY-deltaY))
		ts1.SetPos(input.TouchCoord(centerX+deltaX), input.TouchCoord(centerY+deltaY))
		mtw.Send()
		must(sleep(ctx, 15*time.Millisecond))
	}
	mtw.End()
}

func sleep(ctx context.Context, t time.Duration) error {
	select {
	case <-time.After(t):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
