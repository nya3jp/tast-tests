// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests lacros-chrome running on ChromeOS.
package lacros

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros/lacrosperf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Octane,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Lacros Octane test",
		Contacts:     []string{"edcourtney@chromium.org", "erikchen@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Timeout:      60 * time.Minute,
		Fixture:      "lacros",
		Params: []testing.Param{{
			Val: []browser.Type{browser.TypeLacros, browser.TypeAsh},
		}, {
			Name: "reverse",
			Val:  []browser.Type{browser.TypeAsh, browser.TypeLacros},
		}},
	})
}

const (
	octaneURL = "https://chromium.github.io/octane/"
)

func runOctaneTest(ctx context.Context, ctconn *chrome.TestConn, conn *chrome.Conn) (float64, error) {
	w, err := ash.WaitForAnyWindowWithoutTitle(ctx, ctconn, "about:blank")
	if err != nil {
		return 0.0, err
	}

	if err := ash.SetWindowStateAndWait(ctx, ctconn, w.ID, ash.WindowStateMaximized); err != nil {
		return 0.0, errors.Wrap(err, "failed to maximize window")
	}

	var score float64
	if err := conn.Eval(ctx, `
		new Promise(resolve => BenchmarkSuite.RunSuites({
			NotifyResult(name, result) {
				// Ignore sub-suite scores.
			},
			NotifyScore(score) {
				resolve(parseFloat(score));
			}
		}));`, &score); err != nil {
		return 0.0, errors.Wrap(err, "octane tests did not run")
	}

	return score, nil
}

func runLacrosOctaneTest(ctx context.Context, cr *chrome.Chrome) (float64, error) {
	conn, _, _, cleanup, err := lacrosperf.SetupLacrosTestWithPage(ctx, cr, octaneURL, lacrosperf.StabilizeAfterOpeningURL)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to setup lacros-chrome test page")
	}
	defer cleanup(ctx)

	ctconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to connect to test API")
	}

	return runOctaneTest(ctx, ctconn, conn)
}

func runCrosOctaneTest(ctx context.Context, cr *chrome.Chrome) (float64, error) {
	conn, cleanup, err := lacrosperf.SetupCrosTestWithPage(ctx, cr, octaneURL, lacrosperf.StabilizeAfterOpeningURL)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to setup cros-chrome test page")
	}
	defer cleanup(ctx)

	ctconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to connect to test API")
	}

	return runOctaneTest(ctx, ctconn, conn)
}

func Octane(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := lacrosperf.SetupPerfTest(ctx, tconn, "lacros.Octane")
	if err != nil {
		s.Fatal("Failed to setup test: ", err)
	}
	defer func() {
		if err := cleanup(ctx); err != nil {
			s.Fatal("Failed to cleanup after creating test: ", err)
		}
	}()

	pv := perf.NewValues()

	for _, bt := range s.Param().([]browser.Type) {
		switch bt {
		case browser.TypeLacros:
			lscore, err := runLacrosOctaneTest(ctx, cr)
			if err != nil {
				s.Fatal("Failed to run lacros Octane test: ", err)
			}
			testing.ContextLog(ctx, "Lacros Octane score: ", lscore)
			pv.Set(perf.Metric{
				Name:      "octane.lacros",
				Unit:      "count",
				Direction: perf.BiggerIsBetter,
			}, lscore)
		case browser.TypeAsh:
			cscore, err := runCrosOctaneTest(ctx, cr)
			if err != nil {
				s.Fatal("Failed to run cros Octane test: ", err)
			}
			testing.ContextLog(ctx, "Cros Octane score: ", cscore)
			pv.Set(perf.Metric{
				Name:      "octane.cros",
				Unit:      "count",
				Direction: perf.BiggerIsBetter,
			}, cscore)
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
