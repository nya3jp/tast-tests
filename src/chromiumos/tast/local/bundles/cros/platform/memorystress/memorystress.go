// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memorystress opens synthetic pages to create memory pressure.
package memorystress

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/testing"
)

// Web page filenames to allocate a lot of JavaScript objects.
const (
	AllocPageFilename  = "memory_stress.html"
	JavascriptFilename = "memory_stress.js"
)

// TestCaseResult is the result of a stress test case.
type TestCaseResult struct {
	// jankyCount is the average janky count in 30 seconds histogram.
	jankyCount *metrics.Histogram
	// discardLatency is the discard latency histogram.
	discardLatency *metrics.Histogram
	// reloadCount is the tab reload count.
	reloadCount uint64
	// oomCount is the oom kill count.
	oomCount uint64
}

// activeTabURL returns the URL of the active tab.
func activeTabURL(ctx context.Context, br *browser.Browser) (string, error) {
	tconn, err := br.TestAPIConn(ctx)
	if err != nil {
		return "", errors.Wrap(err, "cannot create test connection")
	}

	var tabURL string
	if err := tconn.Call(ctx, &tabURL, `async () => {
                let tabs = await tast.promisify(chrome.tabs.query)({active: true});
                return tabs[0].url;
        }`); err != nil {
		return "", errors.Wrap(err, "active tab URL not found")
	}
	return tabURL, nil
}

// isTargetAvailable checks if there is any matched target.
func isTargetAvailable(ctx context.Context, br *browser.Browser, tm chrome.TargetMatcher) (bool, error) {
	targets, err := br.FindTargets(ctx, tm)
	if err != nil {
		return false, errors.Wrap(err, "failed to get targets")
	}
	return len(targets) != 0, nil
}

// waitAllocation waits for completion of JavaScript memory allocation.
func waitAllocation(ctx context.Context, conn *chrome.Conn) error {
	const timeout = 60 * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Waits for completion of JavaScript allocation.
	// Checks completion only for the allocation page memory_stress.html.
	// memory_stress.html saves the allocation result to document.out.
	const expr = "!window.location.href.includes('memory_stress') || document.hasOwnProperty('out') == true"
	if err := conn.WaitForExprFailOnErr(waitCtx, expr); err != nil {
		if waitCtx.Err() == context.DeadlineExceeded {
			// Quiesce timeout is common under memory stress, do not interrupt the test in this case.
			testing.ContextLogf(ctx, "Ignoring tab quiesce timeout (%v)", timeout)
			return nil
		}
		return errors.Wrap(err, "unexpected error waiting for tab quiesce")
	}
	return nil
}

// waitAllocationForURL waits for completion of JavaScript memory allocation on the tab with specified URL.
func waitAllocationForURL(ctx context.Context, br *browser.Browser, url string) error {
	conn, err := br.NewConnForTarget(ctx, chrome.MatchTargetURL(url))
	if err != nil {
		return errors.Wrap(err, "NewConnForTarget failed")
	}
	defer conn.Close()

	return waitAllocation(ctx, conn)
}

// openAllocationPage opens a page to allocate many JavaScript objects.
func openAllocationPage(ctx context.Context, url string, br *browser.Browser) error {
	conn, err := br.NewConn(ctx, url)
	if err != nil {
		return errors.Wrap(err, "cannot create new renderer")
	}
	defer conn.Close()

	return waitAllocation(ctx, conn)
}

// openTabCount returns the count of tabs to open.
func openTabCount(mbPerTab int) (int, error) {
	memPerTab := kernelmeter.NewMemSizeMiB(mbPerTab)
	memInfo, err := kernelmeter.MemInfo()
	if err != nil {
		return 0, errors.Wrap(err, "cannot obtain memory info")
	}
	// Allocates more than total memory and swap space size to trigger low memory.
	return int((memInfo.Total+memInfo.SwapTotal)/memPerTab) + 1, nil
}

// openTabs opens tabs to create memory pressure.
func openTabs(ctx context.Context, br *browser.Browser, createTabCount, mbPerTab int, compressRatio float64, baseURL string) error {
	for i := 0; i < createTabCount; i++ {
		url := fmt.Sprintf("%s?alloc=%d&ratio=%.3f&id=%d", baseURL, mbPerTab, compressRatio, i)
		if err := openAllocationPage(ctx, url, br); err != nil {
			return errors.Wrap(err, "cannot create tab")
		}
	}
	return nil
}

// reloadCrashedTab reload the active tab if it's crashed. Returns whether the tab is reloaded.
func reloadCrashedTab(ctx context.Context, br *browser.Browser) (bool, error) {
	tabURL, err := activeTabURL(ctx, br)
	if err != nil {
		return false, errors.Wrap(err, "cannot get active tab URL")
	}

	// If the active tab's URL is not in the devtools targets, the active tab is crashed.
	targetAvailable, err := isTargetAvailable(ctx, br, chrome.MatchTargetURL(tabURL))
	if err != nil {
		return false, errors.Wrap(err, "isTargetAvailable failed")
	}

	if !targetAvailable {
		testing.ContextLog(ctx, "Reload tab:", tabURL)
		if err := br.ReloadActiveTab(ctx); err != nil {
			return false, errors.Wrap(err, "failed to reload active tab")
		}
		if err := waitAllocationForURL(ctx, br, tabURL); err != nil {
			return false, errors.Wrap(err, "waitAllocationForURL failed")
		}
		return true, nil
	}
	return false, nil
}

// waitMoveCursor moves the mouse cursor until after the specified waiting time.
func waitMoveCursor(ctx context.Context, mw *input.MouseEventWriter, d time.Duration) error {
	// Reset the cursor to the top left.
	mw.Move(-10000, -10000)

	const sleepTime = 15 * time.Millisecond
	total := time.Duration(0)
	i := 0
	for total < d {
		// Moves mouse cursor back and forth diagonally.
		if i%100 < 50 {
			mw.Move(5, 5)
		} else {
			mw.Move(-5, -5)
		}
		// Sleeps briefly after each cursor move.
		if err := testing.Sleep(ctx, sleepTime); err != nil {
			return errors.Wrap(err, "sleep timeout")
		}
		i++
		total += sleepTime
	}

	return nil
}

// switchTabs switches between tabs and reloads crashed tabs. Returns the reload count.
func switchTabs(ctx context.Context, br *browser.Browser, switchCount int, localRand *rand.Rand) (uint64, error) {
	mouse, err := input.Mouse(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "cannot initialize mouse")
	}
	defer mouse.Close()

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "cannot initialize keyboard")
	}
	defer keyboard.Close()

	waitTime := 3 * time.Second
	var reloadCount uint64
	for i := 0; i < switchCount; i++ {
		if err := keyboard.Accel(ctx, "ctrl+tab"); err != nil {
			return 0, errors.Wrap(err, "Accel(Ctrl+Tab) failed")
		}

		// Waits between tab switches.
		// +/- within 1000ms from the previous wait time to cluster long and short wait times.
		// On some devices, it's easier to trigger OOM with clustered short wait times.
		// On other devices, it's easier with clustered long wait times.
		// It's necessary to make wait time depending on previous wait time.
		// If each wait time is independent, there will be less clusters of long or short wait times.
		// Wait time is in [1, 5] seconds range.
		waitTime += time.Duration(localRand.Intn(2001)-1000) * time.Millisecond
		if waitTime < time.Second {
			waitTime = time.Second
		} else if waitTime > 5*time.Second {
			waitTime = 5 * time.Second
		}

		if err := waitMoveCursor(ctx, mouse, waitTime); err != nil {
			return 0, errors.Wrap(err, "error when moving mouse cursor")
		}
		testing.ContextLogf(ctx, "%3d, wait time: %v", i, waitTime)

		reloaded, err := reloadCrashedTab(ctx, br)
		if err != nil {
			return 0, errors.Wrap(err, "reloadCrashedTab failed")
		}
		if reloaded {
			reloadCount++
		}
	}
	return reloadCount, nil
}

// ReportTestCaseResult writes the test case result to perfValues and prints the test case result.
func ReportTestCaseResult(ctx context.Context, perfValues *perf.Values, result TestCaseResult, label string) error {
	testing.ContextLog(ctx, "===== "+label+" test results =====")

	jankMean, err := result.jankyCount.Mean()
	if err == nil {
		jankyMetric := perf.Metric{
			Name:      "tast_janky_count_" + label,
			Unit:      "count",
			Direction: perf.SmallerIsBetter,
		}
		perfValues.Set(jankyMetric, jankMean)
		testing.ContextLog(ctx, "Average janky count in 30s: ", jankMean)
	} else {
		testing.ContextLog(ctx, "Failed to get mean for tast_janky_count")
	}

	killLatency, err := result.discardLatency.Mean()
	if err == nil {
		killLatencyMetric := perf.Metric{
			Name:      "tast_discard_latency_" + label,
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}
		perfValues.Set(killLatencyMetric, killLatency)
		testing.ContextLog(ctx, "Average discard latency(ms): ", killLatency)
	} else {
		testing.ContextLog(ctx, "Failed to get mean for tast_discard_latency")
	}

	reloadTabMetric := perf.Metric{
		Name:      "tast_reload_tab_count_" + label,
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(reloadTabMetric, float64(result.reloadCount))
	testing.ContextLog(ctx, "Reload tab count: ", result.reloadCount)

	oomKillerMetric := perf.Metric{
		Name:      "tast_oom_killer_count_" + label,
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(oomKillerMetric, float64(result.oomCount))
	testing.ContextLog(ctx, "OOM Kill count: ", result.oomCount)

	return nil
}

// TestCase opens synthetic pages to allocate JavaScript objects to create memory pressure.
func TestCase(ctx context.Context, br *browser.Browser, localRand *rand.Rand, mbPerTab, switchCount int, compressRatio float64, baseURL string) (TestCaseResult, error) {
	vmstatsStart, err := kernelmeter.VMStats()
	if err != nil {
		return TestCaseResult{}, errors.Wrap(err, "failed to get vmstat")
	}

	createTabCount, err := openTabCount(mbPerTab)
	if err != nil {
		return TestCaseResult{}, errors.Wrap(err, "failed to get open tab count")
	}
	testing.ContextLog(ctx, "Tab count to create: ", createTabCount)

	if err := openTabs(ctx, br, createTabCount, mbPerTab, compressRatio, baseURL); err != nil {
		return TestCaseResult{}, errors.Wrap(err, "failed to open tabs")
	}

	tconn, err := br.TestAPIConn(ctx)
	if err != nil {
		return TestCaseResult{}, errors.Wrap(err, "failed to connect to test API")
	}

	histogramNames := []string{"Browser.Responsiveness.JankyIntervalsPerThirtySeconds", "Memory.LowMemoryKiller.FirstKillLatency"}
	startHistograms, err := metrics.GetHistograms(ctx, tconn, histogramNames)
	if err != nil {
		return TestCaseResult{}, errors.Wrap(err, "failed to get histograms")
	}

	reloadCount, err := switchTabs(ctx, br, switchCount, localRand)
	if err != nil {
		return TestCaseResult{}, errors.Wrap(err, "failed to switch tabs")
	}

	endHistograms, err := metrics.GetHistograms(ctx, tconn, histogramNames)
	if err != nil {
		return TestCaseResult{}, errors.Wrap(err, "failed to get histograms")
	}
	histograms, err := metrics.DiffHistograms(startHistograms, endHistograms)
	if err != nil {
		return TestCaseResult{}, errors.Wrap(err, "failed to diff histograms")
	}
	jankyCount := histograms[0]
	discardLatency := histograms[1]

	vmstatsEnd, err := kernelmeter.VMStats()
	if err != nil {
		return TestCaseResult{}, errors.Wrap(err, "failed to get vmstat")
	}
	oomCount := vmstatsEnd["oom_kill"] - vmstatsStart["oom_kill"]

	return TestCaseResult{
		reloadCount:    reloadCount,
		jankyCount:     jankyCount,
		discardLatency: discardLatency,
		oomCount:       oomCount,
	}, nil
}
