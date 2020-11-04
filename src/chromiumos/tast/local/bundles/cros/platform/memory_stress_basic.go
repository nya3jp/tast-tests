// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/testing"
)

type testCaseResult struct {
	// jankyCount is the average janky count in 30 seconds histogram.
	jankyCount *metrics.Histogram
	// discardLatency is the discard latency histogram.
	discardLatency *metrics.Histogram
	// reloadCount is the tab reload count.
	reloadCount int
	// oomCount is the oom kill count.
	oomCount uint64
}

const (
	allocPageFilename  = "memory_stress.html"
	javascriptFilename = "memory_stress.js"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MemoryStressBasic,
		Desc:     "Create heavy memory pressure and check if oom-killer is invoked",
		Contacts: []string{"vovoy@chromium.org", "chromeos-memory@google.com"},
		Attr:     []string{"group:crosbolt", "crosbolt_memory_nightly"},
		// This test takes 15-30 minutes to run.
		Timeout: 45 * time.Minute,
		Data: []string{
			allocPageFilename,
			javascriptFilename,
		},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"platform.MemoryStressBasic.enableARC", "platform.MemoryStressBasic.minFilelistKB", "platform.MemoryStressBasic.seed"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})

}

func MemoryStressBasic(ctx context.Context, s *testing.State) {
	const (
		mbPerTab    = 800
		switchCount = 150
	)

	minFilelistKB := -1
	if val, ok := s.Var("platform.MemoryStressBasic.minFilelistKB"); ok {
		testing.ContextLog(ctx, "minFilelistKB: ", val)
		val, err := strconv.Atoi(val)
		if err != nil {
			s.Fatal("Cannot parse argument platform.MemoryStressBasic.minFilelistKB: ", err)
		}
		minFilelistKB = val
	}

	// The memory pressure is higher when ARC is enabled (without launching Android apps).
	// Checks the ARC enabled case by default.
	enableARC := true
	if val, ok := s.Var("platform.MemoryStressBasic.enableARC"); ok {
		testing.ContextLog(ctx, "enableARC: ", val)
		intVal, err := strconv.Atoi(val)
		if err != nil {
			s.Fatal("Cannot parse argument platform.MemoryStressBasic.enableARC: ", err)
		}
		if intVal == 0 {
			enableARC = false
		}
	}

	seed := time.Now().UTC().UnixNano()
	if val, ok := s.Var("platform.MemoryStressBasic.seed"); ok {
		intVal, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			s.Fatal("Cannot parse argument platform.MemoryStressBasic.seed: ", err)
		}
		seed = intVal
	}
	testing.ContextLog(ctx, "Seed: ", seed)
	localRand := rand.New(rand.NewSource(seed))

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	baseURL := server.URL + "/" + allocPageFilename

	perfValues := perf.NewValues()

	// Tests both the low compress ratio and high compress ratio cases.
	// When there is more random data (67 percent random), the compress ratio is low,
	// the low memory notification is triggered by low uncompressed anonymous memory.
	// When there is less random data (33 percent random), the compress ratio is high,
	// the low memory notification is triggered by low swap free.
	label67 := "67_percent_random"
	result67, err := stressTestCase(ctx, perfValues, localRand, mbPerTab, switchCount, minFilelistKB, 0.67, baseURL, label67, enableARC)
	if err != nil {
		s.Fatal("67_percent_random test case failed: ", err)
	}
	label33 := "33_percent_random"
	result33, err := stressTestCase(ctx, perfValues, localRand, mbPerTab, switchCount, minFilelistKB, 0.33, baseURL, label33, enableARC)
	if err != nil {
		s.Fatal("33_percent_random test case failed: ", err)
	}

	if err := reportTestCaseResult(ctx, perfValues, result67, label67); err != nil {
		s.Fatal("Reporting 67_percent_random failed: ", err)
	}
	if err := reportTestCaseResult(ctx, perfValues, result33, label33); err != nil {
		s.Fatal("Reporting 33_percent_random failed: ", err)
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

func stressTestCase(ctx context.Context, perfValues *perf.Values, localRand *rand.Rand, mbPerTab, switchCount, minFilelistKB int, compressRatio float64, baseURL, label string, enableARC bool) (testCaseResult, error) {
	var opts []chrome.Option
	if enableARC {
		opts = append(opts, chrome.ARCEnabled())
	}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "cannot start chrome")
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to wait for idle CPU")
	}

	// Setup min_filelist_kbytes after chrome start.
	if minFilelistKB >= 0 {
		if err := ioutil.WriteFile("/proc/sys/vm/min_filelist_kbytes", []byte(strconv.Itoa(minFilelistKB)), 0644); err != nil {
			return testCaseResult{}, errors.Wrap(err, "could not write to /proc/sys/vm/min_filelist_kbytes")
		}
	}

	vmstatsStart, err := kernelmeter.VMStats()
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to get vmstat")
	}

	createTabCount, err := openTabCount(mbPerTab)
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to get open tab count")
	}
	testing.ContextLog(ctx, "Tab count to create: ", createTabCount)

	if err := openTabs(ctx, cr, createTabCount, mbPerTab, compressRatio, baseURL); err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to open tabs")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to connect to test API")
	}

	histogramNames := []string{"Browser.Responsiveness.JankyIntervalsPerThirtySeconds", "Memory.LowMemoryKiller.FirstKillLatency"}
	startHistograms, err := metrics.GetHistograms(ctx, tconn, histogramNames)
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to get histograms")
	}

	reloadCount, err := switchTabs(ctx, cr, switchCount, localRand)
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to switch tabs")
	}

	endHistograms, err := metrics.GetHistograms(ctx, tconn, histogramNames)
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to get histograms")
	}
	histograms, err := metrics.DiffHistograms(startHistograms, endHistograms)
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to diff histograms")
	}
	jankyCount := histograms[0]
	discardLatency := histograms[1]

	vmstatsEnd, err := kernelmeter.VMStats()
	if err != nil {
		return testCaseResult{}, errors.Wrap(err, "failed to get vmstat")
	}
	oomCount := vmstatsEnd["oom_kill"] - vmstatsStart["oom_kill"]

	return testCaseResult{
		reloadCount:    reloadCount,
		jankyCount:     jankyCount,
		discardLatency: discardLatency,
		oomCount:       oomCount,
	}, nil
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
			testing.ContextLogf(ctx, "Ignoring tab quiesce timeout (%v)", timeout)
			return nil
		}
		return errors.Wrap(err, "unexpected error waiting for tab quiesce")
	}
	return nil
}

// waitAllocationForURL waits for completion of JavaScript memory allocation on the tab with specified URL.
func waitAllocationForURL(ctx context.Context, cr *chrome.Chrome, url string) error {
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(url))
	if err != nil {
		return errors.Wrap(err, "NewConnForTarget failed")
	}
	defer conn.Close()

	return waitAllocation(ctx, conn)
}

// openAllocationPage opens a page to allocate many JavaScript objects.
func openAllocationPage(ctx context.Context, url string, cr *chrome.Chrome) error {
	conn, err := cr.NewConn(ctx, url)
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
func openTabs(ctx context.Context, cr *chrome.Chrome, createTabCount, mbPerTab int, compressRatio float64, baseURL string) error {
	for i := 0; i < createTabCount; i++ {
		url := fmt.Sprintf("%s?alloc=%d&ratio=%.3f&id=%d", baseURL, mbPerTab, compressRatio, i)
		if err := openAllocationPage(ctx, url, cr); err != nil {
			return errors.Wrap(err, "cannot create tab")
		}
	}
	return nil
}

// activeTabURL returns the URL of the acitve tab.
func activeTabURL(ctx context.Context, cr *chrome.Chrome) (string, error) {
	tconn, err := cr.TestAPIConn(ctx)
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

// reloadActiveTab reloads the active tab.
func reloadActiveTab(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create test connection")
	}

	return tconn.Eval(ctx, "chrome.tabs.reload()", nil)
}

// reloadCrashedTab reload the active tab if it's crashed. Returns whether the tab is reloaded.
func reloadCrashedTab(ctx context.Context, cr *chrome.Chrome) (bool, error) {
	tabURL, err := activeTabURL(ctx, cr)
	if err != nil {
		return false, errors.Wrap(err, "cannot get active tab URL")
	}

	// If the active tab's URL is not in the devtools targets, the active tab is crashed.
	targetAvailable, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(tabURL))
	if err != nil {
		return false, errors.Wrap(err, "IsTargetAvailable failed")
	}

	if !targetAvailable {
		testing.ContextLog(ctx, "Reload tab:", tabURL)
		if err := reloadActiveTab(ctx, cr); err != nil {
			return false, errors.Wrap(err, "reloadActiveTab failed")
		}
		if err := waitAllocationForURL(ctx, cr, tabURL); err != nil {
			return false, errors.Wrap(err, "waitAllocationForURL failed")
		}
		return true, nil
	}
	return false, nil
}

// waitMoveCursor moves the mouse cursor until after the specified waiting time.
func waitMoveCursor(ctx context.Context, mw *input.MouseEventWriter, d time.Duration) error {
	total := time.Duration(0)
	sleepTime := 15 * time.Millisecond
	i := 0

	// Reset the cursor to the top left.
	mw.Move(-10000, -10000)

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
func switchTabs(ctx context.Context, cr *chrome.Chrome, switchCount int, localRand *rand.Rand) (int, error) {
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
	reloadCount := 0
	for i := 0; i < switchCount; i++ {
		if err := keyboard.Accel(ctx, "ctrl+tab"); err != nil {
			return 0, errors.Wrap(err, "Accel(Ctrl+Tab) failed")
		}
		// Waits between tab switches.
		// +/- within 1000ms from the previous wait time to cluster long and short wait times.
		// On some settings, it's easier to trigger OOM with clustered short wait times.
		// On other settings, it's easier with clustered long wait times.
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

		reloaded, err := reloadCrashedTab(ctx, cr)
		if err != nil {
			return 0, errors.Wrap(err, "reloadCrashedTab failed")
		}
		if reloaded {
			reloadCount++
		}
	}
	return reloadCount, nil
}

// reportTestCaseResult writes the test case result to perfValues and prints the test case result.
func reportTestCaseResult(ctx context.Context, perfValues *perf.Values, result testCaseResult, label string) error {
	testing.ContextLog(ctx, "===== "+label+" test results =====")

	jankMean, err := result.jankyCount.Mean()
	if err != nil {
		return errors.Wrap(err, "failed to get mean for tast_janky_count")
	}
	jankyMetric := perf.Metric{
		Name:      "tast_janky_count_" + label,
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(jankyMetric, jankMean)
	testing.ContextLog(ctx, "Average janky count in 30s: ", jankMean)

	killLatency, err := result.discardLatency.Mean()
	if err != nil {
		return errors.Wrap(err, "failed to get mean for tast_discard_latency")
	}
	killLatencyMetric := perf.Metric{
		Name:      "tast_discard_latency_" + label,
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(killLatencyMetric, killLatency)
	testing.ContextLog(ctx, "Average discard latency(ms): ", killLatency)

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
