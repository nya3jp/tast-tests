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
		s.Fatal("Could not get a new TouchEventWriter")

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
		sleepMilliseconds(ctx, 100)
	}

	// Delay to make the test visually more pleasing.
	sleepMilliseconds(ctx, 500)

	// Draw a circle:
	// Draws a circle with 40 touch events. The touch event is moved to
	// 120 different locations generating a continuous circle.
	stw, err = tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Could not create TouchEventWriter")
	}
	defer stw.Close()

	const radius = 200 // in pixels
	for v := range circleIter(120) {
		x := v.x * radius * pixelToTouchFactorX
		y := v.y * radius * pixelToTouchFactorY
		stw.Move(input.TouchCoord(centerX+x), input.TouchCoord(centerY+y))
		sleepMilliseconds(ctx, 15)
	}
	// And finally "end" (lift the finger) the line.
	stw.End()

	// Delay to make the test visually more pleasing.
	sleepMilliseconds(ctx, 500)

	// Multitouch test: Zoom out + zoom in
	// Get a multitouch writer for two touches.
	mtw, err := tsw.NewMultiTouchWriter(2)
	if err != nil {
		s.Fatal("Could not get a new TouchEventWriter", err)

	}
	defer mtw.Close()

	// Get the individual TouchState
	ts0 := mtw.TouchState(0)
	ts1 := mtw.TouchState(1)

	// Zoom out
	for i := 5; i < 100; i++ {
		deltaX := float64(i) * pixelToTouchFactorX
		deltaY := float64(i) * pixelToTouchFactorY

		ts0.SetX(input.TouchCoord(centerX - deltaX))
		ts0.SetY(input.TouchCoord(centerY - deltaY))
		ts1.SetX(input.TouchCoord(centerX + deltaX))
		ts1.SetY(input.TouchCoord(centerY + deltaY))
		mtw.Sync()
		sleepMilliseconds(ctx, 15)
	}

	// Zoom in
	for i := 100; i > 15; i-- {
		deltaX := float64(i) * pixelToTouchFactorX
		deltaY := float64(i) * pixelToTouchFactorY

		ts0.SetX(input.TouchCoord(centerX - deltaX))
		ts0.SetY(input.TouchCoord(centerY - deltaY))
		ts1.SetX(input.TouchCoord(centerX + deltaX))
		ts1.SetY(input.TouchCoord(centerY + deltaY))
		mtw.Sync()
		sleepMilliseconds(ctx, 15)
	}
	mtw.End()
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
			rads := float64(i)*coef + math.Pi
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
	return nil
}
