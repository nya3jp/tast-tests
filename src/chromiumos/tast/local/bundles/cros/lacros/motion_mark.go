// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/testing"
)

const (
	motionMarkURL = "https://browserbench.org/MotionMark1.2/"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MotionMark,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Runs the MotionMark browser benchmark on either Ash or LaCrOS",
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

func MotionMark(ctx context.Context, s *testing.State) {
	bt := s.Param().(browser.Type)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}

	conn, _, cleanup, err := browserfixt.SetUpWithURL(ctx, s.FixtValue(), bt, motionMarkURL)
	if err != nil {
		s.Fatal("Failed to set up browser: ", err)
	}
	defer cleanup(ctx)
	defer func() {
		conn.CloseTarget(ctx)
		conn.Close()
	}()

	win, err := ash.FindWindow(ctx, tconn, func(window *ash.Window) bool {
		if bt == browser.TypeAsh {
			return window.WindowType == ash.WindowTypeBrowser
		}
		return window.WindowType == ash.WindowTypeLacros
	})
	if err != nil {
		s.Fatal("Failed to find browser window: ", err)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, win.ID, ash.WindowStateMaximized); err != nil {
		s.Error("Failed to fullscreen test window: ", err)
	}

	var score float64
	if err := conn.Eval(ctx, `
    new Promise(resolve => {
      benchmarkRunnerClient.didFinishLastIteration = function() {
        resolve(benchmarkRunnerClient.results.score);
      };
      benchmarkController.startBenchmark();
    })`, &score); err != nil {
		s.Error("Failed to collect MotionMark results score: ", err)
	}

	pv := perf.NewValues()
	pv.Set(
		perf.Metric{
			Name:      "score",
			Unit:      "count",
			Direction: perf.BiggerIsBetter,
		}, score,
	)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}
