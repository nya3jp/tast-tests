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
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Octane,
		Desc:         "Lacros Octane test",
		Contacts:     []string{"edcourtney@chromium.org", "erikchen@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Timeout:      60 * time.Minute,
		Fixture:      "lacros",
	})
}

const (
	octaneURL = "https://chromium.github.io/octane/"
)

func runOctaneTest(ctx context.Context, f launcher.FixtData, conn *chrome.Conn) (float64, error) {
	w, err := lacros.FindFirstNonBlankWindow(ctx, f.TestAPIConn)
	if err != nil {
		return 0.0, err
	}

	if err := ash.SetWindowStateAndWait(ctx, f.TestAPIConn, w.ID, ash.WindowStateMaximized); err != nil {
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

func runLacrosOctaneTest(ctx context.Context, f launcher.FixtData) (float64, error) {
	conn, _, _, cleanup, err := lacros.SetupLacrosTestWithPage(ctx, f, octaneURL)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to setup lacros-chrome test page")
	}
	defer cleanup(ctx)

	return runOctaneTest(ctx, f, conn)
}

func runCrosOctaneTest(ctx context.Context, f launcher.FixtData) (float64, error) {
	conn, cleanup, err := lacros.SetupCrosTestWithPage(ctx, f, octaneURL)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to setup cros-chrome test page")
	}
	defer cleanup(ctx)

	return runOctaneTest(ctx, f, conn)
}

func Octane(ctx context.Context, s *testing.State) {
	tconn, err := s.FixtValue().(launcher.FixtData).Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := lacros.SetupPerfTest(ctx, tconn, "lacros.Octane")
	if err != nil {
		s.Fatal("Failed to setup test: ", err)
	}
	defer func() {
		if err := cleanup(ctx); err != nil {
			s.Fatal("Failed to cleanup after creating test: ", err)
		}
	}()

	pv := perf.NewValues()

	lscore, err := runLacrosOctaneTest(ctx, s.FixtValue().(launcher.FixtData))
	if err != nil {
		s.Fatal("Failed to run lacros Octane test: ", err)
	}
	testing.ContextLog(ctx, "Lacros Octane score: ", lscore)
	pv.Set(perf.Metric{
		Name:      "octane.lacros",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, lscore)

	cscore, err := runCrosOctaneTest(ctx, s.FixtValue().(launcher.FixtData))
	if err != nil {
		s.Fatal("Failed to run cros Octane test: ", err)
	}
	testing.ContextLog(ctx, "Cros Octane score: ", cscore)
	pv.Set(perf.Metric{
		Name:      "octane.cros",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, cscore)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
