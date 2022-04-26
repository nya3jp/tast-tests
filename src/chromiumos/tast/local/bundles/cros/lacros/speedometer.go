// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests lacros-chrome running on ChromeOS.
package lacros

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	_ "chromiumos/tast/local/bundles/cros/lacros/fixtures" // Include the speedometer specific lacros WPR fixture.
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros/lacrosperf"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

type testType int

const (
	// testTypeNormal says to run the Speedometer test as usual.
	testTypeNormal testType = iota
	// testTypeDisplayNone says to run the Speedometer test without showing the page.
	testTypeDisplayNone
)

type testParam struct {
	runOrder []browser.Type
	testType testType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Speedometer,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Lacros Speedometer test",
		Contacts:     []string{"edcourtney@chromium.org", "erikchen@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Timeout:      60 * time.Minute,
		Params: []testing.Param{{
			Name:    "",
			Fixture: "speedometerWPRLacros",
			Val: testParam{
				runOrder: []browser.Type{browser.TypeLacros, browser.TypeAsh},
				testType: testTypeNormal,
			},
		}, {
			Name:    "displaynone",
			Fixture: "speedometerWPRLacros",
			Val: testParam{
				runOrder: []browser.Type{browser.TypeLacros, browser.TypeAsh},
				testType: testTypeDisplayNone,
			},
		}, {
			Name:    "reverse",
			Fixture: "speedometerWPRLacros2",
			Val: testParam{
				runOrder: []browser.Type{browser.TypeAsh, browser.TypeLacros},
				testType: testTypeNormal,
			},
		}, {
			Name:    "reverse_displaynone",
			Fixture: "speedometerWPRLacros2",
			Val: testParam{
				runOrder: []browser.Type{browser.TypeAsh, browser.TypeLacros},
				testType: testTypeDisplayNone,
			},
		}},
	})
}

const (
	speedometerURL = "https://browserbench.org/Speedometer2.0/"
)

func runSpeedometerTest(ctx context.Context, ctconn *chrome.TestConn, t testType, conn *chrome.Conn) (float64, error) {
	w, err := ash.WaitForAnyWindowWithoutTitle(ctx, ctconn, "about:blank")
	if err != nil {
		return 0.0, err
	}

	if err := ash.SetWindowStateAndWait(ctx, ctconn, w.ID, ash.WindowStateMaximized); err != nil {
		return 0.0, errors.Wrap(err, "failed to maximize window")
	}

	if t == testTypeDisplayNone {
		if err := conn.Call(ctx, nil, `() => {
				document.documentElement.style.display = 'none';
			}`); err != nil {
			return 0.0, errors.Wrap(err, "failed to set display none")
		}
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
		// Save a point-of-failure faillog.
		dir, ok := testing.ContextOutDir(ctx)
		if ok {
			dir = filepath.Join(dir, "speedometer_faillog")
			if err := os.MkdirAll(dir, 0755); err != nil {
				testing.ContextLog(ctx, "Error creating speedometer_faillog directory: ", err)
			}
			faillog.SaveToDir(ctx, dir)
		}
		return 0.0, errors.Wrap(err, "speedometer tests did not run")
	}

	return score, nil
}

func runLacrosSpeedometerTest(ctx context.Context, cr *chrome.Chrome, t testType) (float64, error) {
	conn, _, _, cleanup, err := lacrosperf.SetupLacrosTestWithPage(ctx, cr, speedometerURL, lacrosperf.StabilizeAfterOpeningURL)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to setup lacros-chrome test page")
	}
	defer cleanup(ctx)

	ctconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to connect to test API")
	}

	return runSpeedometerTest(ctx, ctconn, t, conn)
}

func runCrosSpeedometerTest(ctx context.Context, cr *chrome.Chrome, t testType) (float64, error) {
	conn, cleanup, err := lacrosperf.SetupCrosTestWithPage(ctx, cr, speedometerURL, lacrosperf.StabilizeAfterOpeningURL)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to setup cros-chrome test page")
	}
	defer cleanup(ctx)

	ctconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0.0, errors.Wrap(err, "failed to connect to test API")
	}

	return runSpeedometerTest(ctx, ctconn, t, conn)
}

func Speedometer(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := lacrosperf.SetupPerfTest(ctx, tconn, "lacros.Speedometer")
	if err != nil {
		s.Fatal("Failed to setup test: ", err)
	}
	defer func() {
		if err := cleanup(ctx); err != nil {
			s.Fatal("Failed to cleanup after creating test: ", err)
		}
	}()

	pv := perf.NewValues()
	for _, bt := range s.Param().(testParam).runOrder {
		switch bt {
		case browser.TypeLacros:
			lscore, err := runLacrosSpeedometerTest(ctx, cr, s.Param().(testParam).testType)
			if err != nil {
				s.Fatal("Failed to run lacros Speedometer test: ", err)
			}
			testing.ContextLog(ctx, "Lacros Speedometer score: ", lscore)
			pv.Set(perf.Metric{
				Name:      "speedometer.lacros",
				Unit:      "count",
				Direction: perf.BiggerIsBetter,
			}, lscore)
			break
		case browser.TypeAsh:
			cscore, err := runCrosSpeedometerTest(ctx, cr, s.Param().(testParam).testType)
			if err != nil {
				s.Fatal("Failed to run cros Speedometer test: ", err)
			}
			testing.ContextLog(ctx, "Cros Speedometer score: ", cscore)
			pv.Set(perf.Metric{
				Name:      "speedometer.cros",
				Unit:      "count",
				Direction: perf.BiggerIsBetter,
			}, cscore)
		}
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
