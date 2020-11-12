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
		Func:         Speedometer,
		Desc:         "Lacros Speedometer test",
		Contacts:     []string{"edcourtney@chromium.org", "erikchen@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Timeout:      60 * time.Minute,
		Data:         []string{launcher.DataArtifact},
		Pre:          launcher.StartedByData(),
	})
}

const (
	speedometerURL = "https://browserbench.org/Speedometer2.0/"
)

func runSpeedometerTest(ctx context.Context, pd launcher.PreData, conn *chrome.Conn) (float64, error) {
	w, err := lacros.FindFirstNonBlankWindow(ctx, pd.TestAPIConn)
	if err != nil {
		return 0.0, err
	}

	if err := ash.SetWindowStateAndWait(ctx, pd.TestAPIConn, w.ID, ash.WindowStateMaximized); err != nil {
		return 0.0, errors.Wrap(err, "failed to maximize window")
	}

	var score float64
	if err := conn.Eval(ctx, `
		new Promise(resolve => {
			benchmarkClient.totalScore = 0;
			benchmarkClient.iterCount = 0;
			benchmarkClient.didRunSuites = function(measuredValues) {
				benchmarkClient.totalScore += measuredValues['score'];
				benchmarkClient.iterCount += 1;
			};
			benchmarkClient.didFinishLastIteration = function() {
				resolve(benchmarkClient.totalScore / benchmarkClient.iterCount);
			};
			var runner = new BenchmarkRunner(Suites, benchmarkClient);
			runner.runMultipleIterations(benchmarkClient.iterationCount);
		})`, &score); err != nil {
		return 0.0, errors.Wrap(err, "speedometer tests did not run")
	}

	return score, nil
}

func runLacrosSpeedometerTest(ctx context.Context, pd launcher.PreData) (float64, error) {
	conn, _, _, cleanup, err := lacros.SetupLacrosTestWithPage(ctx, pd, speedometerURL)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to setup lacros-chrome test page")
	}
	defer cleanup(ctx)

	return runSpeedometerTest(ctx, pd, conn)
}

func runCrosSpeedometerTest(ctx context.Context, pd launcher.PreData) (float64, error) {
	conn, cleanup, err := lacros.SetupCrosTestWithPage(ctx, pd, speedometerURL)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to setup cros-chrome test page")
	}
	defer cleanup(ctx)

	return runSpeedometerTest(ctx, pd, conn)
}

func Speedometer(ctx context.Context, s *testing.State) {
	tconn, err := s.PreValue().(launcher.PreData).Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := lacros.SetupPerfTest(ctx, tconn, "lacros.Speedometer")
	if err != nil {
		s.Fatal("Failed to setup test: ", err)
	}
	defer func() {
		if err := cleanup(ctx); err != nil {
			s.Fatal("Failed to cleanup after creating test: ", err)
		}
	}()

	pv := perf.NewValues()

	lscore, err := runLacrosSpeedometerTest(ctx, s.PreValue().(launcher.PreData))
	if err != nil {
		s.Fatal("Failed to run lacros Speedometer test: ", err)
	}
	testing.ContextLog(ctx, "Lacros Speedometer score: ", lscore)
	pv.Set(perf.Metric{
		Name:      "speedometer.lacros",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, lscore)

	cscore, err := runCrosSpeedometerTest(ctx, s.PreValue().(launcher.PreData))
	if err != nil {
		s.Fatal("Failed to run cros Speedometer test: ", err)
	}
	testing.ContextLog(ctx, "Cros Speedometer score: ", cscore)
	pv.Set(perf.Metric{
		Name:      "speedometer.cros",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, cscore)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
