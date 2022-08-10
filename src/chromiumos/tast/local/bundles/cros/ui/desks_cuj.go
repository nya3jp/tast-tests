// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DesksCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of critical user journey for virtual desks",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		Timeout:      time.Hour,
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "loggedInToCUJUser",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "loggedInToCUJUserLacros",
		}},
	})
}

func DesksCUJ(ctx context.Context, s *testing.State) {
	const browserWindowsPerDesk = 8

	docURL, err := cuj.GetTestDocURL()
	if err != nil {
		s.Fatal("Failed to get test document URL: ", err)
	}

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	var cs ash.ConnSource
	var bTconn *chrome.TestConn
	var l *lacros.Lacros
	switch bt {
	case browser.TypeLacros:
		// Launch lacros.
		var err error
		if l, err = lacros.Launch(ctx, tconn); err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		defer l.Close(cleanupCtx)
		cs = l

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}
	case browser.TypeAsh:
		cs = cr
		bTconn = tconn
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	if err := ash.CreateWindows(ctx, tconn, cs, docURL, browserWindowsPerDesk); err != nil {
		s.Fatal("Failed to create browser windows on first desk: ", err)
	}

	if bt == browser.TypeLacros {
		if err := l.Browser().CloseWithURL(ctx, chrome.NewTabURL); err != nil {
			s.Fatal("Failed to close blank tab: ", err)
		}
	}

	if err := ash.CreateNewDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to create second desk: ", err)
	}
	defer ash.CleanUpDesks(cleanupCtx, tconn)

	if err := ash.ActivateDeskAtIndex(ctx, tconn, 1); err != nil {
		s.Fatal("Failed to switch to second desk: ", err)
	}

	if err := ash.CreateWindows(ctx, tconn, cs, docURL, browserWindowsPerDesk); err != nil {
		s.Fatal("Failed to create browser windows on second desk: ", err)
	}

	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized); err != nil {
			return errors.Wrap(err, "failed to ensure window is maximized")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to ensure all windows are maximized: ", err)
	}

	// The above preparation may take several minutes. Ensure that the
	// display is awake and will stay awake for the performance measurement.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to wake display: ", err)
	}

	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(cleanupCtx)

	configs := []cujrecorder.MetricConfig{
		cujrecorder.NewCustomMetricConfig("Ash.Desks.AnimationLatency.DeskActivation", "ms", perf.SmallerIsBetter),
		cujrecorder.NewSmoothnessMetricConfig("Ash.Desks.AnimationSmoothness.DeskActivation"),
	}
	if err := recorder.AddCollectedMetrics(tconn, browser.TypeAsh, configs...); err != nil {
		s.Fatal("Failed to add recorded metrics: ", err)
	}

	if err := recorder.AddCommonMetrics(tconn, bTconn); err != nil {
		s.Fatal("Failed to add common metrics to recorder: ", err)
	}

	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	if err := recorder.RunFor(ctx, func(ctx context.Context) error {
		if err := ash.ActivateDeskAtIndex(ctx, tconn, 0); err != nil {
			return errors.Wrap(err, "failed to switch to first desk")
		}

		if err := ash.ActivateDeskAtIndex(ctx, tconn, 1); err != nil {
			return errors.Wrap(err, "failed to switch to second desk")
		}

		return nil
	}, 10*time.Minute); err != nil {
		s.Fatal("Failed to conduct the performance measurement: ", err)
	}

	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the performance data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the performance data: ", err)
	}
}
