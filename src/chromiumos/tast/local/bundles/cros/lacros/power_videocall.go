// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	memorymetrics "chromiumos/tast/local/memory/metrics"
	"chromiumos/tast/testing"
)

const (
	videocallURL        = "https://storage.googleapis.com/chromiumos-test-assets-public/power_VideoCall/power_VideoCall.html"
	docsURL             = "http://crospower.page.link/power_VideoCall_doc"
	measurementDuration = 60 * time.Second
	notes               = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. "
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PowerVideocall,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Runs a video conference and text input window side-by-side with either ash-chrome and lacros-chrome",
		Contacts:     []string{"luken@google.com", "hidehiko@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      20 * time.Minute,
		Params: []testing.Param{{
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name:    "ash",
			Val:     browser.TypeAsh,
			Fixture: "chromeLoggedIn",
		}},
	})
}

func PowerVideocall(ctx context.Context, s *testing.State) {
	bt := s.Param().(browser.Type)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows: ", err)
	}
	for _, w := range ws {
		w.CloseWindow(ctx, tconn)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get all windows from ash"))
		}
		if len(ws) > 0 {
			return errors.New("waiting to close all windows")
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: 2 * time.Minute}); err != nil {
		s.Fatal("Failed to poll to close any excess windows: ", err)
	}

	memBase, err := memorymetrics.NewBaseMemoryStats(ctx, nil)
	if err != nil {
		s.Fatal("Failed to get base zram stats: ", err)
	}

	videoConn, br, cleanup, err := browserfixt.SetUpWithURL(ctx, s.FixtValue(), bt, videocallURL)
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer cleanup(ctx)
	defer func() {
		videoConn.CloseTarget(ctx)
		videoConn.Close()
	}()

	bTconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get browser test API connection: ", err)
	}

	videoWin, err := ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
		if bt == browser.TypeAsh {
			return window.WindowType == ash.WindowTypeBrowser
		}
		return window.WindowType == ash.WindowTypeLacros
	})
	if err != nil {
		s.Fatal("Failed to find video window: ", err)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, videoWin.ID, ash.WindowStateRightSnapped); err != nil {
		s.Error("Failed to snap first blank window to the right: ", err)
	}

	pc := pointer.NewMouse(tconn)

	bubble := nodewith.ClassName("PermissionPromptBubbleView").First()
	allow := nodewith.Name("Allow").Role(role.Button).Ancestor(bubble)
	if err := pc.Click(allow)(ctx); err != nil {
		s.Fatal("Failed to click permission bubble: ", err)
	}

	docsConn, err := br.NewConn(ctx, docsURL, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open docs window: ", err)
	}
	defer func() {
		docsConn.CloseTarget(ctx)
		docsConn.Close()
	}()

	docsWin, err := ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
		if bt == browser.TypeAsh {
			if window.WindowType == ash.WindowTypeBrowser {
				return window != videoWin
			}
		} else {
			if window.WindowType == ash.WindowTypeLacros {
				return window != videoWin
			}
		}
		return false
	})
	if err != nil {
		s.Fatal("Failed to find docs window: ", err)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, docsWin.ID, ash.WindowStateLeftSnapped); err != nil {
		s.Error("Failed to snap second blank window to the left: ", err)
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kw.Close()

	// Select text input field
	if err := pc.Click(nodewith.Name("Edit here").Role(role.TextField))(ctx); err != nil {
		s.Fatal("Failed to select input field on docs page: ", err)
	}

	histograms, err := metrics.RunAndWaitAll(
		ctx,
		bTconn,
		2*measurementDuration,
		func(ctx context.Context) error {
			end := time.Now()
			for time.Now().Sub(end) < measurementDuration {
				if err := uiauto.Combine(
					"sleep and type",
					action.Sleep(5*time.Second),
					kw.TypeAction(notes),
				)(ctx); err != nil {
					return err
				}
			}
			return nil
		},
		"Event.Latency.EndToEnd.KeyPress",
	)
	if err != nil {
		s.Fatal("Failed to collect metric data: ", err)
	}

	pv := perf.NewValues()

	if err := memorymetrics.LogMemoryStats(ctx, memBase, nil, pv, s.OutDir(), ""); err != nil {
		s.Error("Failed to log memory stats: ", err)
	}

	for _, h := range histograms {
		mean, err := h.Mean()
		if err != nil {
			s.Error("Failed to extract mean from histogram: ", err)
		}
		pv.Set(
			perf.Metric{
				Name:      h.Name + ".mean",
				Unit:      "microseconds",
				Direction: perf.SmallerIsBetter,
			},
			mean,
		)
		totalCount := h.TotalCount()
		sampleNum95 := (totalCount * 95) / 100
		var max int64
		var variance float64
		var value95 float64
		var t int64
		for _, b := range h.Buckets {
			midpoint := (float64(b.Min) + float64(b.Max)) / 2.0
			variance = variance + (float64(b.Count) * (mean - midpoint) * (mean - midpoint))
			max = b.Max
			if t < sampleNum95 {
				if t+b.Count >= sampleNum95 {
					value95 = float64(b.Min) + ((float64(b.Max) - float64(b.Min)) * ((float64(sampleNum95) - float64(t)) / float64(b.Count)))
				}
			}
			t = t + b.Count
		}

		variance = math.Sqrt(variance / (float64(totalCount) - 1.0))
		pv.Set(
			perf.Metric{
				Name:      h.Name + ".std_dev",
				Unit:      "microseconds",
				Direction: perf.SmallerIsBetter,
			},
			variance,
		)

		pv.Set(
			perf.Metric{
				Name:      h.Name + ".percent_95",
				Unit:      "microseconds",
				Direction: perf.SmallerIsBetter,
			},
			value95,
		)

		pv.Set(
			perf.Metric{
				Name:      h.Name + ".max",
				Unit:      "microseconds",
				Direction: perf.SmallerIsBetter,
			},
			float64(max),
		)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}
