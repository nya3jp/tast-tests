// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewDragWindowPerf,
		Desc:         "Measures the presentation time of window dragging in overview in tablet mode",
		Contacts:     []string{"xiyuan@chromium.org", "mukai@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "tablet_mode"},
		Pre:          chrome.LoggedIn(),
		Timeout:      4 * time.Minute,
	})
}

// Tracks histogram diff between test cases and records metrics.
type perfTracker struct {
	ctx       context.Context
	cr        *chrome.Chrome
	prevHists map[string]*metrics.Histogram
	pv        *perf.Values
}

// newPerfTracker creates a perfTracker.
func newPerfTracker(ctx context.Context, cr *chrome.Chrome) *perfTracker {
	return &perfTracker{
		ctx:       ctx,
		cr:        cr,
		prevHists: make(map[string]*metrics.Histogram),
		pv:        perf.NewValues(),
	}
}

// update updates the cached histogram of given |name| and return the diff since
// last call.
func (p *perfTracker) update(s *testing.State, name string) *metrics.Histogram {
	histogram, err := metrics.GetHistogram(p.ctx, p.cr, name)
	if err != nil {
		s.Fatalf("Failed to get histogram %s: %v", name, err)
	}

	histToReport := histogram
	if prevHist, ok := p.prevHists[name]; ok {
		if histToReport, err = histogram.Diff(prevHist); err != nil {
			s.Fatalf("Failed to compute the histogram diff of %s: %v", name, err)
		}
	}

	p.prevHists[name] = histogram
	return histToReport
}

// recordLatency records the mean of histogram |histName| diffs of a latency
// under |metricName|.
func (p *perfTracker) recordLatency(s *testing.State, histName string, metricName string) {
	histogram := p.update(s, histName)

	p.pv.Set(perf.Metric{
		Name:      metricName,
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, histogram.Mean())
	s.Logf("%s=%f", metricName, histogram.Mean())
}

// Generates touch events for test.
type eventGenerator struct {
	ctx context.Context
	tsw *input.TouchscreenEventWriter
	stw *input.SingleTouchEventWriter

	// Last touch position.
	x input.TouchCoord
	y input.TouchCoord
}

// newEventGenerator creates an eventGenerator.
func newEventGenerator(ctx context.Context, s *testing.State) *eventGenerator {
	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device: ", err)
	}

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}

	return &eventGenerator{
		ctx: ctx,
		tsw: tsw,
		stw: stw,
		x:   input.TouchCoord(0),
		y:   input.TouchCoord(0),
	}
}

func (generator *eventGenerator) close() {
	generator.stw.Close()
	generator.tsw.Close()
}

func (generator *eventGenerator) touchDown(s *testing.State, x, y input.TouchCoord) {
	if err := generator.stw.Move(x, y); err != nil {
		s.Fatal("Failed to execute a move gesture: ", err)
	}
	generator.x = x
	generator.y = y
}

func (generator *eventGenerator) longPressAt(s *testing.State, x, y input.TouchCoord) {
	generator.touchDown(s, x, y)

	if err := testing.Sleep(generator.ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to wait: ", err)
	}
}

func (generator *eventGenerator) moveTo(s *testing.State, x, y input.TouchCoord, t time.Duration) {
	if err := generator.stw.Swipe(generator.ctx, generator.x, generator.y, x, y, t); err != nil {
		s.Fatal("Failed to execute a swipe gesture: ", err)
	}
	generator.x = x
	generator.y = y
}

func (generator *eventGenerator) touchUp(s *testing.State) {
	if err := generator.stw.End(); err != nil {
		s.Fatal("Failed to finish the swipe gesture: ", err)
	}
}

func waitForCPUIdle(ctx context.Context, s *testing.State) {
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}
}

func normalDrag(s *testing.State, generator *eventGenerator) {
	x := input.TouchCoord(generator.tsw.Width() / 3)
	y := input.TouchCoord(generator.tsw.Height() / 3)

	// Long press to pick up the overview item at (x, y).
	generator.longPressAt(s, x, y)

	// TODO(crbug.com/1007060): Add API to verify overview drag status.

	// Drag to move it around.
	t := 1000 * time.Millisecond
	generator.moveTo(s, x+generator.tsw.Width()/2, y, t)
	generator.moveTo(s, x+generator.tsw.Width()/2, y+generator.tsw.Height()/2, t)
	generator.moveTo(s, x, y+generator.tsw.Height()/2, t)
	generator.moveTo(s, x, y, t)

	generator.touchUp(s)
}

func dragToSnap(s *testing.State, generator *eventGenerator) {
	x := input.TouchCoord(generator.tsw.Width() / 3)
	y := input.TouchCoord(generator.tsw.Height() / 3)

	// Long press to pick up the overview item at (x, y).
	generator.longPressAt(s, x, y)

	// TODO(crbug.com/1007060): Add API to verify overview drag status.

	// Drag to the left edge to snap it.
	generator.moveTo(s, input.TouchCoord(0), y, 500*time.Millisecond)

	generator.touchUp(s)

	// TODO(crbug.com/1007060): Add API to verify an item is left snapped.
}

func clearSnap(s *testing.State, generator *eventGenerator) {
	// Clears snapped window by touch down screen center and move all the way to
	// left.
	x := input.TouchCoord(generator.tsw.Width() / 2)
	y := input.TouchCoord(generator.tsw.Height() / 2)

	generator.touchDown(s, x, y)
	generator.moveTo(s, input.TouchCoord(0), y, 500*time.Millisecond)
	generator.touchUp(s)

	// TODO(crbug.com/1007060): Add API to verify no snapped window.
}

func dragToClose(s *testing.State, generator *eventGenerator) {
	x := input.TouchCoord(generator.tsw.Width() / 3)
	y := input.TouchCoord(generator.tsw.Height() / 3)

	// No need to long press to pick it up just drag out of screen to close.
	generator.touchDown(s, x, y)
	generator.moveTo(s, x, generator.tsw.Height()-1, 1000*time.Millisecond)
	generator.touchUp(s)

	// Wait for close animation to finish and close the window.
	if err := testing.Sleep(generator.ctx, 500*time.Millisecond); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	// TODO(crbug.com/1007060): Add API to verify window is really closed.
}

func OverviewDragWindowPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer tconn.Close()

	originalTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)

	if err = ash.SetTabletModeEnabled(ctx, tconn, true); err != nil {
		s.Fatal("Failed to enable tablet mode: ", err)
	}

	generator := newEventGenerator(ctx, s)
	defer generator.close()

	const histName = "Ash.Overview.WindowDrag.PresentationTime.TabletMode"

	p := newPerfTracker(ctx, cr)
	currentWindows := 0
	// Run the test cases in different number of browser windows.
	for _, windows := range []int{2, 8} {
		for ; currentWindows < windows; currentWindows++ {
			conn, err := cr.NewConn(ctx, ui.PerftestURL, cdputil.WithNewWindow())
			if err != nil {
				s.Fatal("Failed to open a new connection: ", err)
			}
			defer conn.Close()
		}

		if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter into the overview mode: ", err)
		}

		waitForCPUIdle(ctx, s)
		normalDrag(s, generator)
		p.recordLatency(s, histName, fmt.Sprintf("%s.NormalDrag.%dwindows", histName, currentWindows))

		waitForCPUIdle(ctx, s)
		dragToSnap(s, generator)
		p.recordLatency(s, histName, fmt.Sprintf("%s.DragToSnap.%dwindows", histName, currentWindows))
		clearSnap(s, generator)

		waitForCPUIdle(ctx, s)
		dragToClose(s, generator)
		p.recordLatency(s, histName, fmt.Sprintf("%s.DragToClose.%dwindows", histName, currentWindows))
		currentWindows--
	}

	if err := p.pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
