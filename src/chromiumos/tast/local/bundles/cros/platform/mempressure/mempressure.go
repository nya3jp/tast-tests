// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mempressure creates a realistic memory pressure situation and takes
// related measurements.
package mempressure

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/shirou/gopsutil/mem"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/kernelmeter"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/wpr"
	"chromiumos/tast/testing"
)

const (
	// CompressibleData is a file containing compressible data for preallocation.
	CompressibleData = "memory_pressure_page.lzo.40"
	// WPRArchiveName is the external file name for the wpr archive.
	WPRArchiveName = "memory_pressure_mixed_sites.wprgo"
)

// tabURLs is a list of URLs to visit in the test.
var tabURLs = []string{
	// Start with a few chapters of War And Peace.
	"https://docs.google.com/document/d/19R_RWgGAqcHtgXic_YPQho7EwZyUAuUZyBq4n_V-BJ0/edit?usp=sharing",
	// And a spreadsheet.
	"https://docs.google.com/spreadsheets/d/1oLBzYb41xxXtn5yoeaqACJ6t2bb3IIN03ug8U7e6uBo/edit?usp=sharing",
	"https://www.google.com/intl/en/drive/",
	"https://www.google.com/photos/about/",
	"https://news.google.com/?hl=en-US&gl=US&ceid=US:en",
	"https://plus.google.com/discover",
	"https://www.google.com/maps/@37.4150659,-122.0788224,15z",
	"https://play.google.com/store",
	"https://play.google.com/music/listen",
	"https://www.youtube.com/",
	"https://www.nytimes.com/",
	"https://www.whitehouse.gov/",
	"https://www.wsj.com/",
	"https://www.newsweek.com/",
	"https://www.washingtonpost.com/",
	"https://www.foxnews.com/",
	"https://www.nbc.com/",
	"https://www.npr.org/",
	"https://www.amazon.com/",
	"https://www.walmart.com/",
	"https://www.target.com/",
	"https://www.facebook.com/",
	"https://www.cnn.com/",
	"https://www.cnn.com/us",
	"https://www.cnn.com/world",
	"https://www.cnn.com/politics",
	"https://www.cnn.com/business",
	"https://www.cnn.com/opinions",
	"https://www.cnn.com/health",
	"https://www.cnn.com/entertainment",
	"https://www.cnn.com/business/tech",
	"https://www.cnn.com/travel",
	"https://www.cnn.com/style",
	"https://bleacherreport.com/",
	"https://chrome.google.com/webstore/category/extensions",
}

// tabSwitchMetric holds tab switch times.
var tabSwitchMetric = perf.Metric{
	Name:      "tast_tab_switch_times",
	Unit:      "second",
	Multiple:  true,
	Direction: perf.SmallerIsBetter,
}

// mean returns the mean of time.Duration values.
func mean(values []time.Duration) time.Duration {
	var sum float64
	for _, v := range values {
		sum += v.Seconds()
	}
	return time.Duration(float64(time.Second) * sum / float64(len(values)))
}

// stdDev returns the standard deviation of time.Duration values.
func stdDev(values []time.Duration) time.Duration {
	var s, s2 float64
	for _, v := range values {
		fv := v.Seconds()
		s += fv
		s2 += fv * fv
	}
	n := float64(len(values))
	return time.Duration(float64(time.Second) * math.Sqrt((s2-s*s/n)/(n-1)))
}

// tab represents a tab on Chrome, providing several APIs to control the tab.
type tab struct {
	// id is an identifier used in chrome.tabs API for this tab.
	id int

	// conn is a connection to the tab.
	conn *chrome.Conn

	// tconn is a connection to the Tast test extension.
	tconn *chrome.Conn
}

// newTab opens a new tab which loads the url, and return a tab instance.
func newTab(ctx context.Context, cr *chrome.Chrome, url string) (*tab, error) {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new renderer")
	}
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	// Because chrome.tabs is not available on the conn, query active tabs
	// assuming there's only one window so only one active tab, and the active tab is
	// the newly created tab, in order to get its TabID.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the connection to the test extension")
	}
	var tabID int
	if err := tconn.EvalPromise(ctx, `(async () => {
	  const tabs = await tast.promisify(chrome.tabs.query)({active: true});
	  if (tabs.length !== 1) {
	    throw new Error("unexpected number of active tabs: got " + tabs.length)
	  }
	  return tabs[0].id;
	})()`, &tabID); err != nil {
		return nil, errors.Wrap(err, "cannot get tab id for the new tab")
	}

	t := &tab{id: tabID, conn: conn, tconn: tconn}
	conn = nil
	return t, nil
}

// close closes the connection to the tab.
func (t *tab) close() error {
	return t.conn.Close()
}

// waitForQuiescence waits for the tab gets quiescence by timeout.
// This does not return an error even if timed out.
func (t *tab) waitForQuiescence(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	// Each resourceTimings element contains the load start time and load end time
	// for a resource.  If a load has not completed yet, the end time is set to
	// the current time.  Then we can tell that a load has completed by detecting
	// that the end time diverges from the current time.
	//
	// resourceTimings is sorted by event start time, so we need to look through
	// the entire array to find the latest activity.
	if err := t.conn.WaitForExprFailOnErr(ctx, `(() => {
	  if (document.readyState !== 'complete') {
	    return false;
	  }

	  const QUIESCENCE_TIMEOUT_MS = 2000;
	  let lastEventTime = performance.timing.loadEventEnd -
	      performance.timing.navigationStart;
	  const resourceTimings = performance.getEntriesByType('resource');
	  lastEventTime = resourceTimings.reduce(
	      (current, timing) => Math.max(current, timing.responseEnd),
	      lastEventTime);
	  return performance.now() >= lastEventTime + QUIESCENCE_TIMEOUT_MS;
	})()`); err != nil {
		if ctx.Err() != context.DeadlineExceeded {
			return errors.Wrap(err, "failed to wait for tab quiesce")
		}
		testing.ContextLogf(ctx, "Ignoring tab quiesce timeout (%v)", timeout)
	} else {
		testing.ContextLog(ctx, "Tab quiescence time: ", time.Now().Sub(start))
	}
	return nil
}

// discarded returns true if the tab with ID tabID was discarded.
func (t *tab) discarded(ctx context.Context) (bool, error) {
	// Discarded tab may be reported by "No tab with id" error by JS, or
	// "discarded" property.
	var discarded bool
	if err := t.tconn.EvalPromise(ctx, fmt.Sprintf(`(async () => {
	  try {
	    const tab = await tast.promisify(chrome.tabs.get)(%d);
	    return tab.discarded;
	  } catch (e) {
	    if (e.message.startsWith("No tab with id: "))
	      return true;
	    throw e;
	  }
	})()`, t.id), &discarded); err != nil {
		return false, err
	}
	return discarded, nil
}

// activate activates the tab, i.e. it selects the tab and brings
// it to the foreground (equivalent to clicking on the tab).
// Returns the duration to perform the switching, or an error is failed.
func (t *tab) activate(ctx context.Context) (time.Duration, error) {
	startTime := time.Now()

	// Request to activate the tab.
	if err := t.tconn.EvalPromise(
		ctx,
		fmt.Sprintf(`tast.promisify(chrome.tabs.update)(%d, {active: true})`, t.id),
		nil,
	); err != nil {
		return 0, err
	}

	// Sometimes tabs crash and the devtools connection goes away.  To avoid waiting 30 minutes
	// for this we use a shorter timeout.
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// We wait for two calls to requestAnimationFrame. When the first
	// requestAnimationFrame is called, we know that a frame is in the
	// pipeline. When the second requestAnimationFrame is called, we know that
	// the first frame has reached the screen.
	if err := t.conn.EvalPromise(waitCtx, `(async () => {
	  await new Promise(window.requestAnimationFrame);
	  await new Promise(window.requestAnimationFrame);
	})()`, nil); err != nil {
		return 0, err
	}

	elapsed := time.Now().Sub(startTime)
	testing.ContextLogf(ctx, "Tab switch time for tab %3d: %7.2f ms", t.id, elapsed.Seconds()*1000)
	return elapsed, nil
}

// wiggle scrolls the main window down in short steps, then jumps back up.
// If the main window is not scrollable, it does nothing.
func (t *tab) wiggle(ctx context.Context) error {
	const (
		scrollCount  = 50
		scrollDelay  = 50 * time.Millisecond
		scrollAmount = 100
	)
	scrollDownCode := fmt.Sprintf("window.scrollBy(0, %d)", scrollAmount)
	scrollUpCode := fmt.Sprintf("window.scrollBy(0, -%d)", scrollAmount*scrollCount)

	for i := 0; i < scrollCount; i++ {
		if err := t.conn.Exec(ctx, scrollDownCode); err != nil {
			return errors.Wrap(err, "scroll down failed")
		}
		if err := testing.Sleep(ctx, scrollDelay); err != nil {
			return err
		}
	}
	if err := t.conn.Exec(ctx, scrollUpCode); err != nil {
		return errors.Wrap(err, "scroll up failed")
	}
	if err := testing.Sleep(ctx, scrollDelay); err != nil {
		return err
	}
	return nil
}

// pin pins the tab. This makes them less likely to be chosen as discard candidates.
func (t *tab) pin(ctx context.Context) error {
	return t.tconn.EvalPromise(
		ctx,
		fmt.Sprintf("tast.promisify(chrome.tabs.update(%d, {pinned: true})", t.id),
		nil,
	)
}

// getValidTabIDs returns a list of non-discarded tab IDs.
func getValidTabIDs(ctx context.Context, tconn *chrome.Conn) ([]int, error) {
	var out []int
	if err := tconn.EvalPromise(ctx, `(async () => {
	  let tabs = await tast.promisify(chrome.tabs.query)({discarded: false});
	  return tabs.map((tab) => tab.id);
	})()`, &out); err != nil {
		return nil, errors.Wrap(err, "cannot query tab list")
	}
	return out, nil
}

// logAndResetStats logs the VM stats from meter, identifying them with
// label.  Then it resets meter.
func logAndResetStats(s *testing.State, meter *kernelmeter.Meter, label string) {
	defer meter.Reset()
	stats, err := meter.VMStats()
	if err != nil {
		// The only possible error from VMStats is that we
		// called it too soon and we prefer to just log it rather than
		// failing the test.
		s.Logf("Metrics: could not log page fault stats for %s: %s", label, err)
		return
	}
	s.Logf("Metrics: %s: total page fault count %d", label, stats.PageFault.Count)
	s.Logf("Metrics: %s: average page fault rate %.1f pf/second", label, stats.PageFault.AverageRate)
	s.Logf("Metrics: %s: max page fault rate %.1f pf/second", label, stats.PageFault.MaxRate)

	logPSIStats(s)
}

// recordAndResetStats records the VM stats from meter, identifying them with
// label.  Then it resets meter.
func recordAndResetStats(s *testing.State, meter *kernelmeter.Meter, values *perf.Values, label string) {
	defer meter.Reset()
	stats, err := meter.VMStats()
	if err != nil {
		s.Errorf("Cannot compute page fault stats (%s): %v", label, err)
		return
	}
	totalPageFaultCountMetric := perf.Metric{
		Name:      "tast_total_page_fault_count_" + label,
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	averagePageFaultRateMetric := perf.Metric{
		Name:      "tast_average_page_fault_rate_" + label,
		Unit:      "faults_per_second",
		Direction: perf.SmallerIsBetter,
	}
	maxPageFaultRateMetric := perf.Metric{
		Name:      "tast_max_page_fault_rate_" + label,
		Unit:      "faults_per_second",
		Direction: perf.SmallerIsBetter,
	}
	values.Set(totalPageFaultCountMetric, float64(stats.PageFault.Count))
	values.Set(averagePageFaultRateMetric, stats.PageFault.AverageRate)
	values.Set(maxPageFaultRateMetric, stats.PageFault.MaxRate)
	s.Logf("Metrics: %s: total page fault count %v", label, stats.PageFault.Count)
	s.Logf("Metrics: %s: oom count %v", label, stats.OOM.Count)
	s.Logf("Metrics: %s: average page fault rate %v pf/second", label, stats.PageFault.AverageRate)
	s.Logf("Metrics: %s: max page fault rate %v pf/second", label, stats.PageFault.MaxRate)
}

// logPSIStats logs the content of /proc/pressure/memory.  If that file is not
// present, this function does nothing.  Other errors are logged.
func logPSIStats(s *testing.State) {
	psi, err := kernelmeter.PSIMemoryLines()
	if err != nil {
		// Here we also don't want to fail the test, just log any error.
		s.Log("Cannot get PSI info: ", err)
	}
	if psi == nil {
		return
	}
	for _, l := range psi {
		s.Log("Metrics: PSI memory: ", l)
	}
}

// cycleTabs activates in turn each tab passed in tabs, then it wiggles it or
// pauses for a duration of pause, depending on the value of wiggle.  It
// returns a slice of tab switch times.
func cycleTabs(ctx context.Context, tabs []*tab, pause time.Duration, wiggle bool) ([]time.Duration, error) {
	var times []time.Duration
	for _, t := range tabs {
		time, err := t.activate(ctx)
		if err != nil {
			// Failed to activate a tab. Check if it is caused by the discarded tab.
			if discarded, inErr := t.discarded(ctx); inErr != nil {
				return times, errors.Wrap(inErr, "failed to verify discard status")
			} else if discarded {
				testing.ContextLogf(ctx, "Tab %d is discarded", t.id)
				continue
			}
			return times, errors.Wrapf(err, "cannot activate tab %d", t.id)
		}
		times = append(times, time)
		if wiggle {
			if err := t.wiggle(ctx); err != nil {
				return times, errors.Wrapf(err, "cannot wiggle tab %d", t.id)
			}
		} else {
			if err := testing.Sleep(ctx, pause); err != nil {
				return times, err
			}
		}
	}
	return times, nil
}

// logTabSwitchTimes takes a slice of tab switch times produced by switching
// through tabCount tabs multiple times, and outputs per-tab stats of those
// times.
func logTabSwitchTimes(ctx context.Context, switchTimes []time.Duration, tabCount int, outDir, label string) {
	logTabSwitchTimesToFile(ctx, switchTimes, outDir, label)
	if len(switchTimes) == tabCount {
		// One switch per tab
		for i, t := range switchTimes {
			testing.ContextLogf(ctx, "Metrics: %s: switch time for tab index %d: %7.2f (ms)",
				label, i, t.Seconds()*1000)
		}
		return
	}
	// Multiple switches per tab.
	tabTimes := make([][]time.Duration, tabCount)
	for i, t := 0, 0; i < len(switchTimes); i++ {
		tabTimes[t] = append(tabTimes[t], switchTimes[i])
		t = (t + 1) % tabCount
	}
	for i, times := range tabTimes {
		testing.ContextLogf(ctx, "Metrics: %s: mean/stddev switch time for tab index %d: %7.2f %7.2f (ms)",
			label, i, mean(times).Seconds()*1000, stdDev(times).Seconds()*1000)
	}
}

// logTabSwitchTimesToFile takes a slice of tab switch times and writes them to a file in the provided
// outDir output directory, using the label string in the file name to give context.
func logTabSwitchTimesToFile(ctx context.Context, switchTimes []time.Duration, outDir, label string) error {
	filename := fmt.Sprintf("%s_times.txt", label)
	f, err := os.OpenFile(filepath.Join(outDir, filename), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return errors.Wrap(err, "failed to open switch times file")
	}
	defer f.Close()
	allFile, err := os.OpenFile(filepath.Join(outDir, "all_times.txt"), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return errors.Wrap(err, "failed to open switch times file")
	}
	defer allFile.Close()
	for _, t := range switchTimes {
		str := fmt.Sprintf("%7.2f\n", t.Seconds()*1000)
		if _, err = fmt.Fprintf(f, str); err != nil {
			return errors.Wrap(err, "failed to write switch times to file")
		}
		if _, err = fmt.Fprintf(allFile, str); err != nil {
			return errors.Wrap(err, "failed to write switch times to file")
		}
	}
	return nil
}

// runTabSwitches performs multiple set of tab switches through the tabs,
// and logs switch times and their stats.
func runTabSwitches(ctx context.Context, tabs []*tab, outDir, label string, repeatCount int) error {
	// Cycle through the tabs once to warm them up (no wiggling).
	if _, err := cycleTabs(ctx, tabs, time.Second, false); err != nil {
		return errors.Wrap(err, "cannot warm-up initial set of tabs")
	}
	// Cycle through tabs a few times, still without wiggling, and collect
	// the switch times.
	var switchTimes []time.Duration
	const shortTabSwitchDelay = 200 * time.Millisecond
	for i := 0; i < repeatCount; i++ {
		times, err := cycleTabs(ctx, tabs, shortTabSwitchDelay, false)
		if err != nil {
			return errors.Wrap(err, "failed to run tab switches")
		}
		switchTimes = append(switchTimes, times...)
	}

	testing.ContextLogf(ctx, "Metrics: %s: mean/stddev of switch times for all tabs: %7.2f %7.2f (ms)",
		label, mean(switchTimes).Seconds()*1000, stdDev(switchTimes).Seconds()*1000)

	// Log tab switch stats on a per-tab basis.
	logTabSwitchTimes(ctx, switchTimes, len(tabs), outDir, label)
	return nil
}

// runAndLogSwapStats runs f and outputs swap stats that correspond to its
// execution.
func runAndLogSwapStats(ctx context.Context, f func(), meter *kernelmeter.Meter) {
	meter.Reset()
	f()
	stats, err := meter.VMStats()
	if err != nil {
		testing.ContextLog(ctx, "Cannot log tab switch stats: ", err)
		return
	}
	testing.ContextLogf(ctx, "Metrics: tab switch swap-in average rate, 10s rate, and count: %.1f %.1f swaps/second, %d swaps",
		stats.SwapIn.AverageRate, stats.SwapIn.RecentRate, stats.SwapIn.Count)
	testing.ContextLogf(ctx, "Metrics: tab switch swap-out average rate, 10s rate, and count: %.1f %.1f swaps/second, %d swaps",
		stats.SwapOut.AverageRate, stats.SwapOut.RecentRate, stats.SwapOut.Count)
	if swapInfo, err := mem.SwapMemory(); err == nil {
		testing.ContextLogf(ctx, "Metrics: free swap %v MiB", (swapInfo.Total-swapInfo.Used)/(1<<20))
	}
	if availableMiB, _, _, err := kernelmeter.ChromeosLowMem(); err == nil {
		testing.ContextLogf(ctx, "Metrics: available %v MiB", availableMiB)
	}
	if m, err := kernelmeter.MemInfo(); err == nil {
		testing.ContextLogf(ctx, "Metrics: free %v MiB, anon %v MiB, file %v MiB", m.Free, m.Anon, m.File)
	}
}

// runPhase1 runs the first phase of the test, creating a memory pressure situation by loading multiple tabs
// into Chrome until the first tab discard occurs. Various measurements are taken as the pressure increases.
func runPhase1(ctx context.Context, s *testing.State, cr *chrome.Chrome, p *RunParameters, initialTabSetSize, recentTabSetSize, tabSwitchRepeatCount int, fullMeter *kernelmeter.Meter, perfValues *perf.Values) (
	pinnedTabs, workTabs []*tab) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Cannot get TestConn: ", err)
	}

	// Create and start the performance meters.  partialMeter takes
	// a measurement after the addition of each tab.  switchMeter
	// takes measurements around tab switches.
	partialMeter := kernelmeter.New(ctx)
	defer partialMeter.Close(ctx)
	switchMeter := kernelmeter.New(ctx)
	defer switchMeter.Close(ctx)

	// Figure out how many tabs already exist (typically 1).
	validTabIDs, err := getValidTabIDs(ctx, tconn)
	if err != nil {
		s.Fatal("Cannot get tab list: ", err)
	}
	initialTabCount := len(validTabIDs)

	var tabs []*tab
	defer func() {
		for _, t := range tabs {
			if err := t.close(); err != nil {
				s.Errorf("Failed to close a tab %d: %v", t.id, err)
			}
		}
	}()

	// Open enough tabs for a "working set", i.e. the number of tabs that an
	// imaginary user will cycle through in their imaginary workflow.
	s.Logf("Opening %d initial tabs", initialTabSetSize)
	tabLoadTimeout := 20 * time.Second
	if p.Mode == wpr.Record {
		tabLoadTimeout = 50 * time.Second
	}
	urlIndex := 0
	for i := 0; i < initialTabSetSize; i++ {
		t, err := newTab(ctx, cr, tabURLs[urlIndex])
		urlIndex = (1 + urlIndex) % len(tabURLs)
		if err != nil {
			s.Fatal("Cannot add initial tab from list: ", err)
		}
		tabs = append(tabs, t)
		if err := t.waitForQuiescence(ctx, tabLoadTimeout); err != nil {
			s.Fatal("Failed to wait for quiescence: ", err)
		}
		if err := t.wiggle(ctx); err != nil {
			s.Error("Cannot wiggle initial tab: ", err)
		}
	}

	for _, t := range tabs {
		if err := t.pin(ctx); err != nil {
			testing.ContextLogf(ctx, "Cannot pin tab %d: %v", t.id, err)
		}
	}
	pinnedTabs = tabs[:]

	// Collect and log tab-switching times in the absence of memory pressure.
	if err := runTabSwitches(ctx, tabs, s.OutDir(), "light", tabSwitchRepeatCount); err != nil {
		s.Error("Cannot run tab switches with light load: ", err)
	}
	logAndResetStats(s, partialMeter, "initial")
	loggedMissingZramStats := false
	var allTabSwitchTimes []time.Duration
	// Allocate memory by opening more tabs and cycling through recently
	// opened tabs until a tab discard occurs.
	for {
		// When recording load each page only once.
		if p.Mode == wpr.Record && len(tabs) > len(tabURLs) {
			break
		}
		validTabIDs, err = getValidTabIDs(ctx, tconn)
		if err != nil {
			s.Fatal("Cannot get tab list: ", err)
		}
		s.Logf("Cycling tabs (opened %d, present %d, initial %d)", len(tabs), len(validTabIDs), initialTabCount)
		if len(tabs)+initialTabCount > len(validTabIDs) {
			s.Log("Ending allocation because one or more targets (tabs) have gone")
			break
		}
		if p.MaxTabCount != 0 && len(tabs) >= p.MaxTabCount {
			s.Log("MaxTabCount reached. Tab count: ", len(tabs))
			break
		}
		// Switch among recently loaded tabs to encourage loading.
		// Errors are usually from a renderer crash or, less likely, a tab discard.
		// We fail in those cases because they are not expected.
		recentTabs := tabs[len(tabs)-recentTabSetSize:]
		times, err := cycleTabs(ctx, recentTabs, time.Second, true)
		if err != nil {
			s.Fatal("Tab cycling error: ", err)
		}
		allTabSwitchTimes = append(allTabSwitchTimes, times...)
		// Quickly switch among initial set of tabs to collect
		// measurements and position the tabs high in the LRU list.
		s.Log("Refreshing LRU order of initial tab set")
		runAndLogSwapStats(ctx, func() {
			if _, err := cycleTabs(ctx, pinnedTabs, 0, false); err != nil {
				s.Fatal("Tab LRU refresh error: ", err)
			}
		}, switchMeter)
		logPSIStats(s)
		t, err := newTab(ctx, cr, tabURLs[urlIndex])
		urlIndex = (1 + urlIndex) % len(tabURLs)
		if err != nil {
			s.Fatal("Cannot add tab from list: ", err)
		}
		tabs = append(tabs, t)
		if err := t.waitForQuiescence(ctx, tabLoadTimeout); err != nil {
			s.Fatal("Failed to wait for quiescence: ", err)
		}
		logAndResetStats(s, partialMeter, fmt.Sprintf("tab %d", t.id))
		if z, err := kernelmeter.ZramStats(ctx); err != nil {
			if !loggedMissingZramStats {
				s.Log("Cannot read zram stats")
				loggedMissingZramStats = true
			}
		} else {
			s.Logf("Metrics: tab %d: swap used %.3f MiB, effective compression %0.3f, utilization %0.3f",
				len(tabs), float64(z.Original)/(1024*1024),
				float64(z.Used)/float64(z.Original),
				float64(z.Compressed)/float64(z.Used))
		}
		if p.Mode == wpr.Record {
			// When recording, add extra time in case the quiesce
			// test had a false positive.
			if err := testing.Sleep(ctx, 10*time.Second); err != nil {
				s.Fatal("Timed out: ", err)
			}
		}
	}
	// Wait a bit so we will notice any additional tab discards.
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		s.Fatal("Timed out: ", err)
	}
	validTabIDs, err = getValidTabIDs(ctx, tconn)
	if err != nil {
		s.Fatal("Cannot get tab list: ", err)
	}

	// Output metrics.
	openedTabsMetric := perf.Metric{
		Name:      "tast_opened_tab_count_1",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}
	lostTabsMetric := perf.Metric{
		Name:      "tast_lost_tab_count_1",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(openedTabsMetric, float64(len(tabs)))
	lostTabs := len(tabs) + initialTabCount - len(validTabIDs)
	perfValues.Set(lostTabsMetric, float64(lostTabs))
	s.Log("Metrics: Phase 1: opened tab count ", len(tabs))
	s.Log("Metrics: Phase 1: lost tab count ", lostTabs)

	times := allTabSwitchTimes
	logTabSwitchTimesToFile(ctx, times, s.OutDir(), "phase1")
	s.Logf("Metrics: Phase 1: mean tab switch time %7.2f ms", mean(times).Seconds()*1000)
	s.Logf("Metrics: Phase 1: stddev of tab switch times %7.2f ms", stdDev(times).Seconds()*1000)

	recordAndResetStats(s, fullMeter, perfValues, "phase_1")
	rtabs := tabs[len(pinnedTabs):]
	tabs = nil // Do not close tabs and let a caller do.
	return pinnedTabs, rtabs
}

// runPhase2 runs the second phase of the test, measuring tab switch times to cold tabs.
func runPhase2(ctx context.Context, s *testing.State, workTabs []*tab, coldTabSetSize int, fullMeter *kernelmeter.Meter, perfValues *perf.Values) {
	if coldTabSetSize > len(workTabs) {
		coldTabSetSize = len(workTabs)
	}
	coldTabs := workTabs[:coldTabSetSize]
	times, err := cycleTabs(ctx, coldTabs, 0, false)
	if err != nil {
		s.Fatal("Cannot switch to cold tabs: ", err)
	}
	logTabSwitchTimes(ctx, times, len(coldTabs), s.OutDir(), "coldswitch")

	recordAndResetStats(s, fullMeter, perfValues, "coldswitch")

}

// runPhase3 runs the third phase of the test, quiesce.
func runPhase3(ctx context.Context, s *testing.State, pinnedTabs []*tab, tabSwitchRepeatCount int, fullMeter *kernelmeter.Meter, perfValues *perf.Values) {
	// Wait a bit to help the system stabilize.
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		s.Fatal("Timed out: ", err)
	}
	// Measure tab switching under pressure.
	if err := runTabSwitches(ctx, pinnedTabs, s.OutDir(), "heavy", tabSwitchRepeatCount); err != nil {
		s.Error("Cannot run tab switches with heavy load: ", err)
	}
	recordAndResetStats(s, fullMeter, perfValues, "phase_3")
}

// RunParameters contains the configurable parameters for Run.
type RunParameters struct {
	// PageFilePath is the path name of a file with one page (4096 bytes)
	// of data.
	PageFilePath string
	// PageFileCompressionRatio is the approximate zram compression ratio of the content of PageFilePath.
	PageFileCompressionRatio float64
	// MaxTabCount is the maximal tab count to open
	MaxTabCount int
	// Mode indicates whether to run in record mode
	// vs. replay mode.
	Mode wpr.Mode
}

// Run creates a memory pressure situation by loading multiple tabs into Chrome
// until the first tab discard occurs.  It takes various measurements as the
// pressure increases (phase 1) and afterwards (phase 2).
func Run(ctx context.Context, s *testing.State, cr *chrome.Chrome, p *RunParameters) {
	const (
		initialTabSetSize    = 5
		recentTabSetSize     = 5
		coldTabSetSize       = 10
		tabCycleDelay        = 300 * time.Millisecond
		tabSwitchRepeatCount = 10
	)

	memInfo, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Cannot obtain memory info: ", err)
	}

	if p.Mode == wpr.Record {
		// Don't attempt to record the pageset on a 2GB device.
		minimumRAM := kernelmeter.NewMemSizeMiB(3 * 1024)
		if memInfo.Total < minimumRAM {
			s.Fatalf("Not enough RAM to record page set: have %v, want %v or more",
				memInfo.Total, minimumRAM)
		}
	}

	// Create and start the performance meter.  fullMeter takes
	// measurements through each full phase of the test.
	fullMeter := kernelmeter.New(ctx)
	defer fullMeter.Close(ctx)

	perfValues := perf.NewValues()

	// Log various system measurements, to help understand the memory
	// manager behavior.
	if err := kernelmeter.LogMemoryParameters(ctx, p.PageFileCompressionRatio); err != nil {
		s.Fatal("Cannot log memory parameters: ", err)
	}

	// Log display size.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Cannot get TestConn: ", err)
	}
	if info, err := display.GetInternalInfo(ctx, tconn); err != nil {
		s.Fatal("Cannot get screen dimensions: ", err)
	} else {
		s.Logf("Display: screen %vx%v", info.Bounds.Width, info.Bounds.Height)
	}

	// -----------------
	// Phase 1: Open several pinned tabs, and then continue to open more tabs until a tab is discarded.
	// -----------------
	pinnedTabs, workTabs := runPhase1(ctx, s, cr, p, initialTabSetSize, recentTabSetSize, tabSwitchRepeatCount, fullMeter, perfValues)
	defer func() {
		tabs := append(pinnedTabs, workTabs...)
		for _, t := range tabs {
			if err := t.close(); err != nil {
				s.Errorf("Failed to close tab %d: %v", t.id, err)
			}
		}
	}()

	// -----------------
	// Phase 2: measure tab switch times to cold tabs.
	// -----------------
	runPhase2(ctx, s, workTabs, coldTabSetSize, fullMeter, perfValues)

	// -----------------
	// Phase 3: quiesce.
	// -----------------
	runPhase3(ctx, s, pinnedTabs, tabSwitchRepeatCount, fullMeter, perfValues)

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
