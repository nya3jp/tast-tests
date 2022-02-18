// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Touch,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Demonstrates injecting touch events",
		Contacts:     []string{"ricardoq@chromium.org", "tast-owners@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.TouchScreen()),
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacrosPrimary",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func Touch(ctx context.Context, s *testing.State) {
	sleep := func(t time.Duration) {
		if err := testing.Sleep(ctx, t); err != nil {
			s.Fatal("Timeout reached: ", err)
		}
	}

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to open connection: ", err)
	}

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("No display: ", err)
	}

	// Setup a browser before opening a tab.
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	// TODO(ricardoq): This page might change/break in the future. If so, a built-in
	// HTML page that accepts drawing should be used. Additionally, Kleki seems to ignore
	// the 2nd & last events when drawing splines. But for the purpose of showing how
	// to use the API is good enough.
	conn, err := br.NewConn(ctx, "http://kleki.com")
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
		sleep(100 * time.Millisecond)
	}

	// Draw a circle:
	// Draws a circle with 120 touch events. The touch event is moved to
	// 120 different locations generating a continuous circle.
	stw, err = tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Could not create TouchEventWriter: ", err)
	}
	defer stw.Close()

	const (
		radius   = 200 // circle radius in pixels
		segments = 120 // segments used for the circle
	)
	for i := 0; i < segments; i++ {
		rads := 2.0*math.Pi*(float64(i)/segments) + math.Pi
		x := radius * pixelToTouchFactorX * math.Cos(rads)
		y := radius * pixelToTouchFactorY * math.Sin(rads)
		if err := stw.Move(input.TouchCoord(centerX+x), input.TouchCoord(centerY+y)); err != nil {
			s.Fatal("Failed to move the touch event: ", err)
		}
		sleep(15 * time.Millisecond)
	}
	// And finally "end" (lift the finger) the line.
	if err := stw.End(); err != nil {
		s.Fatal("Failed to finish the touch event: ", err)
	}

	// Swipe test:
	// Draw a box around the circle using 4 swipes.
	const boxSize = radius * 2 // box size in pixels
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
		x0 := input.TouchCoord(centerX + boxSize/2*d.x0*pixelToTouchFactorX)
		y0 := input.TouchCoord(centerY + boxSize/2*d.y0*pixelToTouchFactorY)
		x1 := input.TouchCoord(centerX + boxSize/2*d.x1*pixelToTouchFactorX)
		y1 := input.TouchCoord(centerY + boxSize/2*d.y1*pixelToTouchFactorY)

		if err := stw.Swipe(ctx, x0, y0, x1, y1, 500*time.Millisecond); err != nil {
			s.Error("Failed to run Swipe: ", err)
		}
	}
	if err := stw.End(); err != nil {
		s.Error("Failed to finish the swipe gesture: ", err)
	}

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
		sleep(15 * time.Millisecond)
	}

	// Zoom in
	for i := 100; i > 15; i-- {
		deltaX := float64(i) * pixelToTouchFactorX
		deltaY := float64(i) * pixelToTouchFactorY

		ts0.SetPos(input.TouchCoord(centerX-deltaX), input.TouchCoord(centerY-deltaY))
		ts1.SetPos(input.TouchCoord(centerX+deltaX), input.TouchCoord(centerY+deltaY))
		mtw.Send()
		sleep(15 * time.Millisecond)
	}
	mtw.End()
}
