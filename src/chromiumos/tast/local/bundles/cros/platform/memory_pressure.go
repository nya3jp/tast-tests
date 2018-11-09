// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MemoryPressure,
		Desc:         "Create memory pressure and collect various measurements from Chrome and from the kernel",
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      30 * time.Minute,
		Data:         []string{wprArchiveName, "quiescence.js"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

// List of URLs to visit in the test.
var tabURLs = []string{
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
	"https://www.cnn.com/",
}

const wprArchiveName = "memory_pressure_mixed_sites.wprgo"

// This test creates one renderer for each tab.
type renderer struct {
	conn  *chrome.Conn
	tabID int
}

var nextURLIndex = 0

// nextURL returns successive URLs from |tabURLs|.
func nextURL() string {
	url := tabURLs[nextURLIndex]
	nextURLIndex++
	if nextURLIndex >= len(tabURLs) {
		nextURLIndex = 0
	}
	return url
}

// evalPromiseBody executes a JS promise on connection |conn|.  |promiseBody|
// is the code run as a promise, and it must contain a call to resolve().
// Returns in |out| a value whose type must match the type of the object
// returned by the "resolve" call.
func evalPromiseBody(ctx context.Context, s *testing.State, conn *chrome.Conn,
	promiseBody string, out interface{}) error {
	promise := fmt.Sprintf("new Promise((resolve, reject) => { %s });", promiseBody)
	if err := conn.EvalPromise(ctx, promise, out); err != nil {
		return errors.Wrapf(err, "cannot execute promise (%s)", promise)
	}
	return nil
}

// execPromiseBody performs as above, but no out parameter.
func execPromiseBody(ctx context.Context, s *testing.State, conn *chrome.Conn,
	promiseBody string) error {
	return evalPromiseBody(ctx, s, conn, promiseBody, nil)
}

// evalPromiseBodyInBrowser performs as above, but executes the promise in the browser.
func evalPromiseBodyInBrowser(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	promiseBody string, out interface{}) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create test API connection")
	}
	return evalPromiseBody(ctx, s, tconn, promiseBody, out)
}

// execPromiseBodyInBrowser connects to Chrome and executes a JS promise
// which does not return a value.
func execPromiseBodyInBrowser(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	promiseBody string) error {
	return evalPromiseBodyInBrowser(ctx, s, cr, promiseBody, nil)
}

// getActiveTabID returns the tab ID for the currently active tab.
func getActiveTabID(ctx context.Context, s *testing.State, cr *chrome.Chrome) (int, error) {
	var tabID int
	promiseBody := "chrome.tabs.query({'active': true}, (tlist) => { resolve(tlist[0]['id']) })"
	err := evalPromiseBodyInBrowser(ctx, s, cr, promiseBody, &tabID)
	if err != nil {
		return 0, errors.Wrap(err, "cannot get tabID")
	}
	return tabID, nil
}

// addTab creates a new renderer and the associated tab, which loads |url|.
// Returns the renderer instance.
func addTab(ctx context.Context, s *testing.State, cr *chrome.Chrome, url string) (*renderer, error) {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new renderer")
	}
	tabID, err := getActiveTabID(ctx, s, cr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get tab id for new renderer")
	}
	r := &renderer{
		conn:  conn,
		tabID: tabID,
	}
	return r, nil
}

// addTabFromList creates a new renderer/tab with the next URL from a URL list.
func addTabFromList(ctx context.Context, s *testing.State,
	cr *chrome.Chrome, quiescenceCode string) (*renderer, error) {
	tab, err := addTab(ctx, s, cr, nextURL())
	if err != nil {
		return nil, err
	}
	// Wait for tab loading quiescence.  Ignore timeouts, but return other
	// errors.
	const tabLoadTimeout = 20 * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, tabLoadTimeout)
	defer cancel()
	startTime := time.Now()
	if err = tab.conn.WaitForExpr(waitCtx, quiescenceCode); err != nil {
		if waitCtx.Err() == context.DeadlineExceeded {
			s.Logf("Ignoring tab quiesce timeout (%v)", tabLoadTimeout)
			return tab, nil
		}
	} else {
		s.Logf("Tab quiesce time: %v", time.Now().Sub(startTime))
	}
	return tab, err
}

// activateTab activates the tab for tabID.
func activateTab(ctx context.Context, s *testing.State, cr *chrome.Chrome, tabID int) error {
	code := fmt.Sprintf(`chrome.tabs.update(%d, {"active": true}, () => { resolve() })`, tabID)
	err := execPromiseBodyInBrowser(ctx, s, cr, code)
	if err != nil {
		return err
	}
	startTime := time.Now()
	r := renderers[tabID]
	if r == nil {
		return nil
	}
	err = execPromiseBody(ctx, s, r.conn, `
// Code which calls resolve() when a tab's frame has been rendered.
(function () {
  // We wait for two calls to requestAnimationFrame. When the first
  // requestAnimationFrame is called, we know that a frame is in the
  // pipeline. When the second requestAnimationFrame is called, we know that
  // the first frame has reached the screen.
  var frame_count = 0;
  var wait_for_raf = function() {
    frame_count++;
    if (frame_count == 2) {
      resolve();
    } else {
      window.requestAnimationFrame(wait_for_raf);
    }
  };
  window.requestAnimationFrame(wait_for_raf);
})()

`)
	if err != nil {
		return err
	}
	switchTime := time.Now().Sub(startTime)
	s.Logf("tab switch time for tab %d: %v", tabID, switchTime)
	perfValues.Append(tabSwitchMetric, float64(switchTime/time.Millisecond))
	return nil
}

// getValidTabIDs returns a list of non-discarded tab IDs.
func getValidTabIDs(ctx context.Context, s *testing.State, cr *chrome.Chrome) []int {
	var out []int
	err := evalPromiseBodyInBrowser(ctx, s, cr, `
chrome.tabs.query({"discarded": false}, function(tab_list) {
	resolve(tab_list.map((tab) => { return tab["id"]; }))
});
`, &out)
	if err != nil {
		s.Fatal("Cannot query tab list: ", err)
	}
	return out
}

// emulateTyping emulates typing from some layer outside the browser.
func emulateTyping(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	r *renderer, text string) error {
	s.Log("Finding and opening keyboard device")
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot open keyboard device")
	}
	defer keyboard.Close()
	if err = keyboard.Type(ctx, text); err != nil {
		return errors.Wrap(err, "cannot emulate typing")
	}
	return nil
}

// waitForElement waits until the DOM element specified by |selector| appears in
// the tab backed by rendered |r|.
func waitForElement(ctx context.Context, s *testing.State, r *renderer, selector string) error {
	queryCode := fmt.Sprintf("resolve(document.querySelector(%q) !== null)", selector)

	// Wait for element to appear.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var pageReady bool
		err := evalPromiseBody(ctx, s, r.conn, queryCode, &pageReady)
		if err != nil {
			return errors.Wrap(err, "cannot determine page status")
		}
		if pageReady {
			return nil
		}
		return errors.New("element not present")
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 100 * time.Millisecond,
	})
	if err != nil {
		return errors.Wrap(err, "polling for element failed")
	}
	return nil
}

// focusOnElement places keyboard input focus on the DOM specified by
// |selector| in the tab backed by renderer |r|.
func focusOnElement(ctx context.Context, s *testing.State, r *renderer, selector string) error {
	focusCode := fmt.Sprintf("{ document.querySelector('%s').focus(); resolve(); }", selector)
	if err := execPromiseBody(ctx, s, r.conn, focusCode); err != nil {
		return errors.Wrap(err, "cannot focus on element")
	}
	return nil
}

// googleLogin logs onto GAIA (NOT WORKING YET).
func googleLogIn(ctx context.Context, s *testing.State, cr *chrome.Chrome) error {
	loginURL := "https://accounts.google.com/ServiceLogin?continue=https%3A%2F%2Faccounts.google.com%2FManageAccount"
	loginTab, err := addTab(ctx, s, cr, loginURL)
	if err != nil {
		return errors.Wrap(err, "cannot add login tab")
	}
	// emailSelector := "input[type=email]:not([aria-hidden=true]),#Email:not(.hidden)"
	emailSelector := "input[type=email]"
	if err := waitForElement(ctx, s, loginTab, emailSelector); err != nil {
		return errors.Wrap(err, "email entry field not found")
	}
	// Get focus on email field.
	if err := focusOnElement(ctx, s, loginTab, emailSelector); err != nil {
		return errors.Wrap(err, "cannot focus on email entry field")
	}
	lightSleep(ctx, 5*time.Second)
	// Enter email.
	err = emulateTyping(ctx, s, cr, loginTab, "wpr.memory.pressure.test@gmail.com")
	if err != nil {
		return errors.Wrap(err, "cannot enter login name")
	}
	s.Log("Email entered")
	lightSleep(ctx, 1*time.Second)
	err = emulateTyping(ctx, s, cr, loginTab, "\n")
	if err != nil {
		return errors.Wrap(err, "cannot enter login name")
	}
	passwordSelector := "input[type=password]"
	// TODO: need to figure out why waitForElement below is not sufficient
	// to properly delay further input.
	lightSleep(ctx, 5*time.Second)
	// Wait for password prompt.
	if err := waitForElement(ctx, s, loginTab, passwordSelector); err != nil {
		return errors.Wrap(err, "password field not found")
	}
	// Focus on password field.
	if err := focusOnElement(ctx, s, loginTab, passwordSelector); err != nil {
		return errors.Wrap(err, "cannot focus on password field")
	}
	// Enter password.
	err = emulateTyping(ctx, s, cr, loginTab, "google.memory.chrome")
	if err != nil {
		return errors.Wrap(err, "cannot enter password")
	}
	s.Log("Password entered")
	// TODO: figure out if and why this wait is needed.
	lightSleep(ctx, 5*time.Second)
	err = emulateTyping(ctx, s, cr, loginTab, "\n")
	// TODO: figure out if and why this wait is needed.
	lightSleep(ctx, 10*time.Second)
	return nil
}

// wiggleTab scrolls the main window down and up a few times.
// If the main window is not scrollable, it does nothing.
func wiggleTab(ctx context.Context, s *testing.State, r *renderer) {
	if r == nil {
		return
	}
	const (
		scrollCount  = 10
		scrollDelay  = 50 * time.Millisecond
		scrollAmount = 100
	)
	scrollDownCode := fmt.Sprintf("window.scrollBy(0, %d)", scrollAmount)
	scrollUpCode := fmt.Sprintf("window.scrollBy(0, -%d)", scrollAmount)

	for i := 0; i < scrollCount; i++ {
		if err := r.conn.Exec(ctx, scrollDownCode); err != nil {
			s.Fatal("Scroll down failed: ", err)
		}
		lightSleep(ctx, scrollDelay)
	}
	for i := 0; i < scrollCount; i++ {
		if err := r.conn.Exec(ctx, scrollUpCode); err != nil {
			s.Fatal("Scroll up failed: ", err)
		}
		lightSleep(ctx, scrollDelay)
	}
}

// lightSleep pauses execution for time span |t|, or less if a timeout intervenes.
func lightSleep(ctx context.Context, t time.Duration) {
	select {
	case <-time.After(t):
	case <-ctx.Done():
	}
}

var (
	// renderers maps a tab ID to its renderer struct.  The initial tab is
	// not mapped here.
	renderers map[int]*renderer
	// perfValues contains all performance measurements.
	perfValues *perf.Values
	// tabSwitchMetric contains all tab switching times.
	tabSwitchMetric perf.Metric
)

func initMetrics() {
	perfValues = &perf.Values{}
	tabSwitchMetric = perf.Metric{
		Name:      "tast_tab_switch_times",
		Unit:      "millisecond",
		Multiple:  true,
		Direction: perf.SmallerIsBetter,
	}
}

// initBrowser restarts the browser on the DUT in preparation for testing.
func initBrowser(ctx context.Context, s *testing.State, useLiveSites bool) (*chrome.Chrome, *testexec.Cmd, error) {
	var (
		cr  *chrome.Chrome
		wpr *testexec.Cmd
		err error
	)

	if useLiveSites {
		s.Log("Starting chrome with live sites")
		cr, err = chrome.New(ctx)
		return cr, wpr, err
	}

	s.Log("Starting chrome with WPR")
	// Start the Web Page Replay in replay mode.
	//
	// This test can also be used to record a page set with WPR.  To do
	// that, change "replay" to "record" below, set |wprArchivePath| to a
	// file of your choice, and change |newTabDelay| to a large number,
	// like 1 minute.
	wprArchivePath := s.DataPath(wprArchiveName)
	// TEMPORARY NOTE.  The WPR archive is not public and should be
	// installed manually the first time the test is run on a DUT.  The GS
	// URL of the archive is:
	//
	// gs://chromiumos-test-assets-public/tast/cros/platform/memory_pressure_mixed_sites_20181211.wprgo
	//
	// and the DUT location is
	//
	// /usr/local/share/tast/data_pushed/chromiumos/tast/local/bundles/cros/platform/data/memory_pressure_mixed_sites.wprgo
	//
	// This will be fixed when private tests become available.
	s.Logf("Using WPR archive %s", wprArchivePath)
	wpr = testexec.CommandContext(ctx, "wpr", "replay",
		"--http_port=8080", "--https_port=8081",
		"--https_cert_file=/usr/share/wpr/wpr_cert.pem",
		"--https_key_file=/usr/share/wpr/wpr_key.pem",
		"--inject_scripts=/usr/share/wpr/deterministic.js",
		wprArchivePath)
	err = wpr.Start()
	if err != nil {
		wpr.DumpLog(ctx)
		return nil, wpr, errors.Wrap(err, "cannot start WPR")
	}
	// Wait a little for WPR to initialize.  (There's no simple way
	// of telling when it has.)
	lightSleep(ctx, 2*time.Second)

	// Restart chrome for use with WPR.
	resolverRules := "MAP *:80 127.0.0.1:8080,MAP *:443 127.0.0.1:8081,EXCLUDE localhost"
	resolverRulesFlag := fmt.Sprintf("--host-resolver-rules=%q", resolverRules)
	spkiList := "PhrPvGIaAMmd29hj8BCZOq096yj7uMpRNHpn5PDxI6I="
	spkiListFlag := fmt.Sprintf("--ignore-certificate-errors-spki-list=%s", spkiList)
	extraArgs := []string{resolverRulesFlag, spkiListFlag}
	cr, err = chrome.New(ctx, chrome.ExtraArgs(extraArgs))
	return cr, wpr, err
}

// getPfCount returns the total number of major page faults since boot.
func getPfCount(s *testing.State) int64 {
	bytes, err := ioutil.ReadFile("/proc/vmstat")
	if err != nil {
		s.Fatal("Cannot read /proc/vmstat")
	}
	chars := string(bytes)
	lines := strings.Split(chars, "\n")
	var value string
	for i := range lines {
		if strings.HasPrefix(lines[i], "pgmajfault ") {
			value = strings.Split(lines[i], " ")[1]
		}
	}
	if len(value) == 0 {
		s.Fatal("Cannot find pgmajfault in /proc/vmstat")
	}
	pfCount, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		s.Fatal("Cannot parse pgmajfault value: ", value)
	}
	return pfCount
}

const pfMeterSamplePeriod = 1 * time.Second

var (
	pfMeterMutex         = &sync.Mutex{}
	pfMeterStartTime     time.Time
	pfMeterStartCount    int64
	pfMeterMaxStartTime  time.Time
	pfMeterMaxStartCount int64
	pfMeterMaxRate       = 0.0
)

// pfMeterReset resets the page fault meter.
func pfMeterReset(s *testing.State) {
	pfMeterMutex.Lock()
	pfMeterStartTime = time.Now()
	pfMeterStartCount = getPfCount(s)
	pfMeterMaxStartTime = pfMeterStartTime
	pfMeterMaxStartCount = pfMeterStartCount
	pfMeterMaxRate = 0.0
	pfMeterMutex.Unlock()
}

// pfMeterRun runs the page fault meter, which samples the page fault rate
// periodically according to pfMeterSamplePeriod and tracks its max value.
func pfMeterRun(ctx context.Context, s *testing.State) {
	for {
		lightSleep(ctx, pfMeterSamplePeriod)
		count := getPfCount(s)
		now := time.Now()
		pfMeterMutex.Lock()
		interval := float64(now.Sub(pfMeterMaxStartTime) / time.Second)
		if interval > 0 {
			rate := float64(count-pfMeterMaxStartCount) / interval
			if rate > pfMeterMaxRate {
				pfMeterMaxRate = rate
			}
		}
		pfMeterMaxStartTime = now
		pfMeterMaxStartCount = count
		pfMeterMutex.Unlock()
	}
}

// pfMeterGetStats returns the total number of page faults and the average and
// max page fault rate.
func pfMeterGetStats(ctx context.Context, s *testing.State) (int64, float64, float64) {
	count := getPfCount(s)
	now := time.Now()
	pfMeterMutex.Lock()
	interval := float64(now.Sub(pfMeterStartTime) / time.Second)
	if interval == 0.0 {
		pfMeterMutex.Unlock()
		lightSleep(ctx, 10*time.Millisecond)
		return pfMeterGetStats(ctx, s)
	}
	countDelta := count - pfMeterStartCount
	averageRate := float64(countDelta) / interval
	maxRate := pfMeterMaxRate
	pfMeterMutex.Unlock()
	return countDelta, averageRate, maxRate
}

// MemoryPressure is the main test function.
func MemoryPressure(ctx context.Context, s *testing.State) {
	const (
		useLogIn          = false
		useLiveSites      = false
		tabWorkingSetSize = 5
		newTabDelay       = 0 * time.Second
		tabCycleDelay     = 300 * time.Millisecond
	)
	var err error

	// Start the page fault meter.
	pfMeterReset(s)
	go pfMeterRun(ctx, s)

	// Load the JS definition of testHasReachedNetworkQuiescence.
	var bytes []byte
	bytes, err = ioutil.ReadFile(s.DataPath("quiescence.js"))
	if err != nil {
		s.Fatal("Cannot read quiescence.js: ", err)
	}
	quiescenceCode := string(bytes)

	initMetrics()
	cr, wpr, err := initBrowser(ctx, s, useLiveSites)
	defer func() {
		if wpr != nil {
			if err := wpr.Kill(); err != nil {
				s.Fatal("Cannot kill WPR: ", err)
			}
		}
	}()
	if err != nil {
		s.Fatal("Cannot start browser: ", err)
	}
	defer cr.Close(ctx)

	// Remove HTTP cache for consistency.  Chrome will recreate it.
	s.Log("Clearing http cache")
	err = os.RemoveAll("/home/chronos/user/Cache")
	if err != nil {
		s.Fatal("Cannot clear HTTP cache: ", err)
	}

	// Log in.  This isn't working (yet).
	if useLogIn {
		s.Log("Logging in")
		err = googleLogIn(ctx, s, cr)
		if err != nil {
			s.Fatal("Cannot login to google: ", err)
		}
	}

	// Figure out how many tabs already exist (typically 1).
	initialTabCount := len(getValidTabIDs(ctx, s, cr))
	var openedTabs []*renderer
	defer func() {
		// Close all connections.
		for i := range openedTabs {
			openedTabs[i].conn.Close()
		}
	}()

	renderers = make(map[int]*renderer)

	// Open enough tabs for a "working set", i.e. the number of tabs that an
	// imaginary user will cycle through in their imaginary workflow.
	s.Logf("Opening %d initial tabs", tabWorkingSetSize)
	for i := 0; i < tabWorkingSetSize; i++ {
		renderer, err := addTabFromList(ctx, s, cr, quiescenceCode)
		if err != nil {
			s.Fatal("Cannot add initial tab from list: ", err)
		}
		openedTabs = append(openedTabs, renderer)
		renderers[renderer.tabID] = renderer
		lightSleep(ctx, newTabDelay)
	}
	// Allocate memory by opening more tabs and cycling through recently
	// opened tabs until a tab discard occurs.
	var validTabIDs []int
	for {
		validTabIDs = getValidTabIDs(ctx, s, cr)
		s.Logf("Cycling tabs (opened %v, present %v, initial %v",
			len(openedTabs), len(validTabIDs), initialTabCount)
		if len(openedTabs)+initialTabCount > len(validTabIDs) {
			s.Log("Ending allocation because one or more targets (tabs) have gone")
			break
		}
		for i := 0; i < tabWorkingSetSize; i++ {
			recent := i + len(openedTabs) - tabWorkingSetSize
			err := activateTab(ctx, s, cr, validTabIDs[recent])
			if err != nil {
				// If the error is due to the tab having been
				// discarded (although it is not expected that
				// a discarded tab would cause an error here),
				// we'll catch the discard next time around the
				// loop.  Log the error and ignore it.
				s.Logf("Cannot activate tab: %v", err)
			}
			lightSleep(ctx, tabCycleDelay)
			wiggleTab(ctx, s, renderers[validTabIDs[recent]])
		}
		renderer, err := addTabFromList(ctx, s, cr, quiescenceCode)
		if err != nil {
			s.Fatal("Cannot add tab from list: ", err)
		}
		openedTabs = append(openedTabs, renderer)
		renderers[renderer.tabID] = renderer
		lightSleep(ctx, newTabDelay)
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
	totalPfCount1Metric := perf.Metric{
		Name:      "tast_total_pagefault_count_1",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	averagePfRate1Metric := perf.Metric{
		Name:      "tast_average_pf_rate_1",
		Unit:      "faults-per-second",
		Direction: perf.SmallerIsBetter,
	}
	maxPfRate1Metric := perf.Metric{
		Name:      "tast_max_pf_rate_1",
		Unit:      "faults-per-second",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(openedTabsMetric, float64(len(openedTabs)))
	lostTabs := len(openedTabs) + initialTabCount - len(validTabIDs)
	perfValues.Set(lostTabsMetric, float64(lostTabs))
	totalPfCount, averagePfRate, maxPfRate := pfMeterGetStats(ctx, s)
	perfValues.Set(totalPfCount1Metric, float64(totalPfCount))
	perfValues.Set(averagePfRate1Metric, averagePfRate)
	perfValues.Set(maxPfRate1Metric, maxPfRate)
	s.Logf("Metrics: Phase 1: opened tab count %v", len(openedTabs))
	s.Logf("Metrics: Phase 1: lost tab count %v", lostTabs)
	s.Logf("Metrics: Phase 1: total PF count %v", totalPfCount)
	s.Logf("Metrics: Phase 1: average PF rate %v", averagePfRate)
	s.Logf("Metrics: Phase 1: max PF rate %v", maxPfRate)

	// Phase 2: quiesce.
	lightSleep(ctx, 5*time.Second)
	pfMeterReset(s)
	lightSleep(ctx, 1*time.Minute)
	totalPfCount, averagePfRate, maxPfRate = pfMeterGetStats(ctx, s)
	totalPfCount2Metric := perf.Metric{
		Name:      "tast_total_pagefault_count_2",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	averagePfRate2Metric := perf.Metric{
		Name:      "tast_average_pf_rate_2",
		Unit:      "faults-per-second",
		Direction: perf.SmallerIsBetter,
	}
	maxPfRate2Metric := perf.Metric{
		Name:      "tast_max_pf_rate_2",
		Unit:      "faults-per-second",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(totalPfCount2Metric, float64(totalPfCount))
	perfValues.Set(averagePfRate2Metric, averagePfRate)
	perfValues.Set(maxPfRate2Metric, maxPfRate)
	s.Logf("Metrics: Phase 2: total PF count %v", totalPfCount)
	s.Logf("Metrics: Phase 2: average PF rate %v", averagePfRate)
	s.Logf("Metrics: Phase 2: max PF rate %v", maxPfRate)

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
