// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskswitchcuj

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cuj"
)

// simpleWebsites are websites to be opened in individual browsers
// with no additional setup required.
// 1. WebGL Aquarium -- considerable load on graphics.
// 2. Chromium issue tracker -- considerable amount of elements.
// 3. CrosVideo -- customizable video player.
// 5. Google Slides -- large Google Slides for RAM pressure.
var simpleWebsites = []string{
	"https://bugs.chromium.org/p/chromium/issues/list",
	"https://crosvideo.appspot.com/?codec=h264_60&loop=true&mute=true",
	"https://webglsamples.org/aquarium/aquarium.html?numFish=1000",
	"https://docs.google.com/presentation/d/1lItrhkgBqXF_bsP-tOqbjcbBFa86--m3DT5cLxegR2k/edit?usp=sharing&resourcekey=0-FmuN4N-UehRS2q4CdQzRXA",
}

// openChromeTabs opens Chrome tabs and returns a function to
// initialize all of the tabs, a function to cleanup all of the
// tabs, and the number of windows that were opened.
//
// This function opens an individual window for each URL in
// simpleWebsites and for the Speedometer browser benchmark. It
// also opens a window with multiple tabs, to increase RAM pressure
// during the test.
func openChromeTabs(ctx context.Context, tconn, bTconn *chrome.TestConn, cs ash.ConnSource, isLacros, tabletMode bool, pv *perf.Values) (func(ctx context.Context) error, func(ctx context.Context) error, int, error) {
	const numExtraWebsites int = 5

	// Keep track of the initial number of windows to ensure
	// we open the right number of windows.
	var ws []*ash.Window
	var err error
	if ws, err = ash.GetAllWindows(ctx, tconn); err != nil {
		return nil, nil, 0, errors.Wrap(err, "failed to get window list")
	}
	initialNumWindows := len(ws)

	var initFns []func(ctx context.Context) error
	var cleanupFns []func(ctx context.Context) error
	var tabs []cuj.TabConn

	// Open up a single window with a lot of tabs to increase RAM pressure.
	var manyTabs []cuj.TabConn
	if manyTabs, err = cuj.NewTabs(ctx, bTconn, cs, false, numExtraWebsites); err != nil {
		return nil, nil, 0, errors.Wrap(err, "failed to bulk open tabs")
	}
	tabs = append(tabs, manyTabs...)

	// Lacros specific setup to close "New Tab" window.
	if isLacros {
		// Don't include the "New Tab" window in the initial window count.
		initialNumWindows--

		if err = cuj.CloseBrowserTabByTitle(ctx, bTconn, "New Tab"); err != nil {
			return nil, nil, 0, errors.Wrap(err, `failed to close "New Tab" tab`)
		}
	}

	// Open up test specific websites.
	var taskSwitchTabs []cuj.TabConn
	if taskSwitchTabs, err = cuj.NewTabsByURLs(ctx, bTconn, cs, true, simpleWebsites); err != nil {
		return nil, nil, 0, err
	}
	tabs = append(tabs, taskSwitchTabs...)

	// Close all current connections to tabs because we don't need them.
	for _, t := range tabs {
		if err := t.Conn.Close(); err != nil {
			return errors.Wrapf(err, "failed to close connection to %s", t.URL)
		}
	}

	// Open the Speedometer benchmark to increase CPU pressure.
	var speedometerTab *cuj.TabConn
	if speedometerTab, err = cuj.NewTabByURL(ctx, bTconn, cs, true, "https://browserbench.org/Speedometer2.0/"); err != nil {
		return nil, nil, 0, errors.Wrap(err, "failed to open Speedometer tabs")
	}
	initFns = append(initFns, func(ctx context.Context) error {
		return initializeSpeedometer(ctx, speedometerTab.Conn)
	})
	cleanupFns = append(cleanupFns, func(ctx context.Context) error {
		var score float64
		if score, err = readSpeedometerScore(ctx, speedometerTab.Conn); err != nil {
			return errors.Wrap(err, "failed to read Speedometer score")
		}
		pv.Set(perf.Metric{
			Name:      "Stress.Speedometer.Score",
			Unit:      "score",
			Direction: perf.BiggerIsBetter,
		}, score)
		return nil
	})

	if !tabletMode {
		if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
			return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateNormal)
		}); err != nil {
			return nil, nil, 0, errors.Wrap(err, "failed to set each window to normal state")
		}
	}

	init := func(ctx context.Context) error {
		for _, f := range initFns {
			if err := f(ctx); err != nil {
				return err
			}
		}
		return nil
	}
	cleanup := func(ctx context.Context) error {
		for _, f := range cleanupFns {
			if err := f(ctx); err != nil {
				return err
			}
		}
		return nil
	}

	// Expected number of browser windows should include the number
	// of websites that didn't need initialization, the Speedometer
	// tab, and the window with many tabs.
	expectedNumBrowserWindows := len(simpleWebsites) + 2
	expectedNumWindows := expectedNumBrowserWindows + initialNumWindows
	if ws, err := ash.GetAllWindows(ctx, tconn); err != nil {
		return nil, nil, 0, errors.Wrap(err, "failed to get window list after opening Chrome tabs")
	} else if len(ws) != expectedNumWindows {
		return nil, nil, 0, errors.Wrapf(err, "unexpected number of windows open after launching Chrome tabs, got: %d, expected: %d", len(ws), expectedNumWindows)
	}

	return init, cleanup, expectedNumBrowserWindows, nil
}

// initializeSpeedometer sets up the Speedometer benchmark, and starts the test.
func initializeSpeedometer(ctx context.Context, conn *chrome.Conn) error {
	return conn.Eval(ctx, `
		// Set how many iterations we want the test to run for. Pick a number
		// that should take longer than 10 minutes on a powerful device.
		benchmarkClient.iterationCount = 150;

		// Maintain the score and iteration count of how many test iterations
		// have already completed. This is used to calculate the final score when the
		// test is finished.
		benchmarkClient.totalScore = 0;
		benchmarkClient.iterCount = 0;
		benchmarkClient.didRunSuites = (measuredValues) => {
			benchmarkClient.totalScore += measuredValues['score']
			benchmarkClient.iterCount += 1;
		}

		var runner = new BenchmarkRunner(Suites, benchmarkClient);
		runner.runMultipleIterations(benchmarkClient.iterationCount)`, nil)
}

// readSpeedometerScore reads the currently available score for the speedometer benchmark.
// Speedometer runs multiple iterations, and we keep track of the score after each iteration.
// When reading the speedometer score, we return the average score across all iterations.
func readSpeedometerScore(ctx context.Context, conn *chrome.Conn) (float64, error) {
	var score float64
	if err := conn.Eval(ctx, `
		new Promise(resolve => {
			if (!benchmarkClient || !benchmarkClient.totalScore || !benchmarkClient.iterCount) {
				resolve(-1.0);
			}
			resolve(benchmarkClient.totalScore / benchmarkClient.iterCount);
		})`, &score); err != nil {
		return 0, errors.Wrap(err, "failed to read Speedometer score")
	}
	if score < 0 {
		return 0, errors.New("speedometer crashed during the test")
	}
	return score, nil
}
