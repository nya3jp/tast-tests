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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/platform/kernelmeter"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const (
	allocPageFilename  = "memory_stress.html"
	javascriptFilename = "memory_stress.js"
	compressRaio       = 0.67
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
		Pre:          arc.Booted(),
		SoftwareDeps: []string{"android", "chrome"},
		Vars:         []string{"platform.MemoryStressBasic.minFilelistKB"},
	})

}

func MemoryStressBasic(ctx context.Context, s *testing.State) {
	const (
		mbPerTab    = 800
		switchCount = 300
	)

	rand.Seed(time.Now().UTC().UnixNano())

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := cpu.WaitUntilIdle(waitCtx); err != nil {
		s.Fatal("Failed to wait for idle CPU: ", err)
	}

	perfValues := perf.NewValues()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// The memory pressure is higher when ARC is enabled (without launching Android apps).
	// Checks if the memory policy works with ARC enabled.
	cr := s.PreValue().(arc.PreData).Chrome

	// Setup min_filelist_kbytes.
	if val, ok := s.Var("platform.MemoryStressBasic.minFilelistKB"); ok {
		testing.ContextLog(ctx, "minFilelistKB: ", val)
		if _, err := strconv.Atoi(val); err != nil {
			s.Fatal("Cannot parse argument platform.MemoryStressBasic.minFilelistKB: ", err)
		}
		if err := ioutil.WriteFile("/proc/sys/vm/min_filelist_kbytes", []byte(val), 0644); err != nil {
			s.Fatal("Could not write to /proc/sys/vm/min_filelist_kbytes: ", err)
		}
	}

	vmstatsStart, err := kernelmeter.VMStats()
	if err != nil {
		s.Fatal("Failed to get vmstat: ", err)
	}

	createTabCount, err := openTabCount(mbPerTab)
	if err != nil {
		s.Fatal("Failed to get open tab count: ", err)
	}
	testing.ContextLog(ctx, "Tab count to create: ", createTabCount)

	url := server.URL + "/" + allocPageFilename

	if err := openTabs(ctx, cr, createTabCount, mbPerTab, url); err != nil {
		s.Fatal("Failed to open tabs: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	histogramNames := []string{"Browser.Responsiveness.JankyIntervalsPerThirtySeconds", "Arc.LowMemoryKiller.FirstKillLatency"}
	startHistograms, err := metrics.GetHistograms(ctx, tconn, histogramNames)
	if err != nil {
		s.Fatal("Failed to get histograms: ", err)
	}

	reloadCount, err := switchTabs(ctx, s, cr, switchCount)
	if err != nil {
		s.Fatal("Failed to switch tabs: ", err)
	}

	endHistograms, err := metrics.GetHistograms(ctx, tconn, histogramNames)
	if err != nil {
		s.Fatal("Failed to get histograms: ", err)
	}
	histograms, err := metrics.DiffHistograms(startHistograms, endHistograms)
	if err != nil {
		s.Fatal("Failed to diff histograms: ", err)
	}
	err = reportMemoryStressHistograms(ctx, perfValues, histograms)
	if err != nil {
		s.Fatal("Failed to report histograms: ", err)
	}

	reloadTabMetric := perf.Metric{
		Name:      "tast_reload_tab_count",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(reloadTabMetric, float64(reloadCount))
	testing.ContextLog(ctx, "Reload tab count: ", reloadCount)

	vmstatsEnd, err := kernelmeter.VMStats()
	if err != nil {
		s.Fatal("Failed to get vmstat: ", err)
	}
	oomCount := vmstatsEnd["oom_kill"] - vmstatsStart["oom_kill"]

	oomKillerMetric := perf.Metric{
		Name:      "tast_oom_killer_count",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(oomKillerMetric, float64(oomCount))
	testing.ContextLog(ctx, "OOM Kill count: ", oomCount)

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

// waitAllocation waits for completion of JavaScript memory allocation.
func waitAllocation(ctx context.Context, conn *chrome.Conn) error {
	const timeout = 60 * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Waits for completion of JavaScript allocation.
	// Checks completion only for the allocation page memory_stress.html.
	// memory_stress.html saves the allocation result to document.out.
	expr := "!window.location.href.includes('memory_stress') || document.hasOwnProperty('out') == true"
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
func openTabs(ctx context.Context, cr *chrome.Chrome, createTabCount int, mbPerTab int, baseURL string) error {
	for i := 0; i < createTabCount; i++ {
		url := fmt.Sprintf("%s?alloc=%d&ratio=%.3f&id=%d", baseURL, mbPerTab, compressRaio, i)
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
	const exp = `
		new Promise((resolve, reject) => {
			chrome.tabs.query({active: true}, (tlist) => {
				resolve(tlist[0].url);
			});
		});`
	if err := tconn.EvalPromise(ctx, exp, &tabURL); err != nil {
		return "", errors.Wrap(err, "EvalPromise failed")
	}
	return tabURL, nil
}

// reloadActiveTab reloads the active tab.
func reloadActiveTab(ctx context.Context, cr *chrome.Chrome) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create test connection")
	}

	return tconn.Eval(ctx, "chrome.tabs.reload();", nil)
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

// switchTabs switches between tabs and reloads crashed tabs. Returns the reload cound.
func switchTabs(ctx context.Context, s *testing.State, cr *chrome.Chrome, switchCount int) (int, error) {
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
		waitTime += time.Duration(rand.Intn(2001)-1000) * time.Millisecond
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

// reportMemoryStressHistograms sets the histogram averages to perfValues.
func reportMemoryStressHistograms(ctx context.Context, perfValues *perf.Values, histograms []*metrics.Histogram) error {
	if len(histograms) != 2 {
		return errors.New("unexpected histogram count")
	}

	jankMean, err := histograms[0].Mean()
	if err != nil {
		return errors.Wrap(err, "failed to get mean for tast_janky_count")
	}
	jankyMetric := perf.Metric{
		Name:      "tast_janky_count",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(jankyMetric, jankMean)
	testing.ContextLog(ctx, "Average janky count in 30s: ", jankMean)

	killLatency, err := histograms[1].Mean()
	if err != nil {
		return errors.Wrap(err, "failed to get mean for tast_discard_latency")
	}
	killLatencyMetric := perf.Metric{
		Name:      "tast_discard_latency",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(killLatencyMetric, killLatency)
	testing.ContextLog(ctx, "Average discard latency(ms): ", killLatency)

	return nil
}
