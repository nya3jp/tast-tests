// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"net"
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
		Attr:         []string{"group:crosbolt", "crosbolt_nightly", "disable:true"},
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

// mean returns the mean.
func mean(v []float64) float64 {
	var sum float64
	for i := range v {
		sum += v[i]
	}
	return sum / float64(len(v))
}

// stddev returns the stddev.
func stdDev(v []float64) float64 {
	var s float64
	var s2 float64
	var n = float64(len(v))
	for i := range v {
		s += v[i]
		s2 += v[i] * v[i]
	}
	return math.Sqrt((s2 - s*s/n) / float64(n-1))
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
	promiseBody := "chrome.tabs.query({active: true}, (tlist) => { resolve(tlist[0].id) })"
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
func addTabQuiesce(ctx context.Context, s *testing.State,
	cr *chrome.Chrome, url string, quiescenceCode string) (*renderer, error) {
	tab, err := addTab(ctx, s, cr, url)
	if err != nil {
		return nil, err
	}
	// Wait for tab loading quiescence.  Ignore timeouts, but return other
	// errors.
	const tabLoadTimeout = 20 * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, tabLoadTimeout)
	defer cancel()
	startTime := time.Now()
	// Try the code once to check for errors, since WaitForExpr hides them.
	var quiesced bool
	err = tab.conn.Eval(ctx, quiescenceCode, &quiesced)
	if err != nil {
		s.Fatal("quiescence code eval: ", err)
	}
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
	code := fmt.Sprintf(`chrome.tabs.update(%d, {active: true}, () => { resolve() })`, tabID)
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
  let frameCount = 0;
  let waitForRaf = function() {
    frameCount++;
    if (frameCount == 2) {
      resolve();
    } else {
      window.requestAnimationFrame(waitForRaf);
    }
  };
  window.requestAnimationFrame(waitForRaf);
})()
`)
	if err != nil {
		return err
	}
	switchTime := time.Now().Sub(startTime)
	s.Logf("tab switch time for tab %d: %v", tabID, switchTime)
	switchTimeMs := float64(switchTime / time.Millisecond)
	perfValues.Append(tabSwitchMetric, switchTimeMs)
	tabSwitchTimesMs = append(tabSwitchTimesMs, switchTimeMs)
	return nil
}

// getValidTabIDs returns a list of non-discarded tab IDs.
func getValidTabIDs(ctx context.Context, s *testing.State, cr *chrome.Chrome) []int {
	var out []int
	err := evalPromiseBodyInBrowser(ctx, s, cr, `
chrome.tabs.query({discarded: false}, function(tabList) {
	resolve(tabList.map((tab) => { return tab.id; }))
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
	focusCode := fmt.Sprintf("document.querySelector('%s').focus();", selector)
	if err := r.conn.Exec(ctx, focusCode); err != nil {
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

// wiggleTab scrolls the main window down in short steps, then jumps back up.
// If the main window is not scrollable, it does nothing.
func wiggleTab(ctx context.Context, s *testing.State, r *renderer) {
	if r == nil {
		return
	}
	const (
		scrollCount  = 50
		scrollDelay  = 50 * time.Millisecond
		scrollAmount = 100
	)
	scrollDownCode := fmt.Sprintf("window.scrollBy(0, %d)", scrollAmount)
	scrollUpCode := fmt.Sprintf("window.scrollBy(0, -%d)", scrollAmount*scrollCount)

	for i := 0; i < scrollCount; i++ {
		if err := r.conn.Exec(ctx, scrollDownCode); err != nil {
			s.Fatal("Scroll down failed: ", err)
		}
		lightSleep(ctx, scrollDelay)
	}
	if err := r.conn.Exec(ctx, scrollUpCode); err != nil {
		s.Fatal("Scroll up failed: ", err)
	}
	lightSleep(ctx, scrollDelay)
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
	// tabSwitchTimes also contains all tab switching times in ms.
	tabSwitchTimesMs []float64
)

func waitForTCPSocket(ctx context.Context, socket string) error {
	err := testing.Poll(ctx, func(ctx context.Context) error {
		conn, err := net.Dial("tcp", "localhost:8080")
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}, &testing.PollOptions{
		Interval: 1 * time.Second,
		Timeout:  60 * time.Second,
	})
	if err != nil {
		return errors.Wrapf(err, "cannot connect to socket %s", socket)
	}
	return nil
}

// initBrowser restarts the browser on the DUT in preparation for testing.
func initBrowser(ctx context.Context, s *testing.State, useLiveSites bool) (cr *chrome.Chrome, wpr *testexec.Cmd, err error) {
	if useLiveSites {
		s.Log("Starting chrome with live sites")
		cr, err = chrome.New(ctx)
		wpr = nil
		return
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

	if err := wpr.Start(); err != nil {
		wpr.DumpLog(ctx)
		return nil, nil, errors.Wrap(err, "cannot start WPR")
	}

	cleanUp := func() {
		if cr != nil {
			cr.Close(ctx)
		}
		if err := wpr.Kill(); err != nil {
			s.Fatal("cannot kill WPR")
		}
	}

	// Restart chrome for use with WPR.  Chrome can start before WPR is
	// ready because it won't need it until we start opening tabs.
	resolverRules := "MAP *:80 127.0.0.1:8080,MAP *:443 127.0.0.1:8081,EXCLUDE localhost"
	resolverRulesFlag := fmt.Sprintf("--host-resolver-rules=%q", resolverRules)
	spkiList := "PhrPvGIaAMmd29hj8BCZOq096yj7uMpRNHpn5PDxI6I="
	spkiListFlag := fmt.Sprintf("--ignore-certificate-errors-spki-list=%s", spkiList)
	extraArgs := []string{resolverRulesFlag, spkiListFlag}
	cr, err = chrome.New(ctx, chrome.ExtraArgs(extraArgs))
	if err != nil {
		err = errors.Wrap(err, "cannot start Chrome")
		cleanUp()
		return
	}

	// Wait for WPR to initialize.
	if err := waitForTCPSocket(ctx, "localhost:8080"); err != nil {
		cleanUp()
		return nil, nil, errors.Wrap(err, "cannot connect to WPR at localhost:8080")
	}
	s.Log("WPR is up and running on localhost:8080")
	if err := waitForTCPSocket(ctx, "localhost:8081"); err != nil {
		cleanUp()
		return nil, nil, errors.Wrap(err, "cannot connect to WPR at localhost:8081")
	}
	s.Log("WPR is up and running on localhost:8081")
	return
}

// getPageFaultCount returns the total number of major page faults since boot.
func getPageFaultCount(s *testing.State) int64 {
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
	pageFaultCount, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		s.Fatal("Cannot parse pgmajfault value: ", value)
	}
	return pageFaultCount
}

const pageFaultMeterSamplePeriod = 1 * time.Second

var (
	pageFaultMeterMutex         = &sync.Mutex{}
	pageFaultMeterStartTime     time.Time
	pageFaultMeterStartCount    int64
	pageFaultMeterMaxStartTime  time.Time
	pageFaultMeterMaxStartCount int64
	pageFaultMeterMaxRate       = 0.0
)

// pageFaultMeterReset resets the page fault meter.
func pageFaultMeterReset(s *testing.State) {
	pageFaultMeterMutex.Lock()
	pageFaultMeterStartTime = time.Now()
	pageFaultMeterStartCount = getPageFaultCount(s)
	pageFaultMeterMaxStartTime = pageFaultMeterStartTime
	pageFaultMeterMaxStartCount = pageFaultMeterStartCount
	pageFaultMeterMaxRate = 0.0
	pageFaultMeterMutex.Unlock()
}

// pageFaultMeterRun runs the page fault meter, which samples the page fault rate
// periodically according to pageFaultMeterSamplePeriod and tracks its max value.
func pageFaultMeterRun(ctx context.Context, s *testing.State) {
	for {
		lightSleep(ctx, pageFaultMeterSamplePeriod)
		count := getPageFaultCount(s)
		now := time.Now()
		pageFaultMeterMutex.Lock()
		interval := float64(now.Sub(pageFaultMeterMaxStartTime) / time.Second)
		if interval > 0 {
			rate := float64(count-pageFaultMeterMaxStartCount) / interval
			if rate > pageFaultMeterMaxRate {
				pageFaultMeterMaxRate = rate
			}
		}
		pageFaultMeterMaxStartTime = now
		pageFaultMeterMaxStartCount = count
		pageFaultMeterMutex.Unlock()
	}
}

// pageFaultMeterGetStats returns the total number of page faults and the average and
// max page fault rate.
func pageFaultMeterGetStats(ctx context.Context, s *testing.State) (faultCount int64, averageRate float64, maxRate float64) {
	count := getPageFaultCount(s)
	now := time.Now()
	pageFaultMeterMutex.Lock()
	interval := float64(now.Sub(pageFaultMeterStartTime) / time.Second)
	if interval == 0.0 {
		pageFaultMeterMutex.Unlock()
		lightSleep(ctx, 10*time.Millisecond)
		return pageFaultMeterGetStats(ctx, s)
	}
	faultCount = count - pageFaultMeterStartCount
	averageRate = float64(faultCount) / interval
	maxRate = pageFaultMeterMaxRate
	pageFaultMeterMutex.Unlock()
	return
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

	// Start the page fault meter.
	pageFaultMeterReset(s)
	go pageFaultMeterRun(ctx, s)

	// Load the JS definition of testHasReachedNetworkQuiescence.
	var bytes []byte
	bytes, err := ioutil.ReadFile(s.DataPath("quiescence.js"))
	if err != nil {
		s.Fatal("Cannot read quiescence.js: ", err)
	}
	quiescenceCode := string(bytes)

	perfValues = &perf.Values{}
	tabSwitchMetric = perf.Metric{
		Name:      "tast_tab_switch_times",
		Unit:      "millisecond",
		Multiple:  true,
		Direction: perf.SmallerIsBetter,
	}

	// Remove HTTP cache for consistency.  Chrome will recreate it.
	s.Log("Clearing HTTP cache")
	err = os.RemoveAll("/home/chronos/user/Cache")
	if err != nil {
		s.Fatal("Cannot clear HTTP cache: ", err)
	}

	cr, wpr, err := initBrowser(ctx, s, useLiveSites)
	if err != nil {
		s.Fatal("Cannot start browser: ", err)
	}
	defer cr.Close(ctx)
	defer func() {
		if err := wpr.Kill(); err != nil {
			s.Fatal("cannot kill WPR")
		}
	}()

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
	var newRenderers []*renderer
	defer func() {
		// Close all connections.
		for _, r := range newRenderers {
			r.conn.Close()
		}
	}()

	renderers = make(map[int]*renderer)

	// Open enough tabs for a "working set", i.e. the number of tabs that an
	// imaginary user will cycle through in their imaginary workflow.
	s.Logf("Opening %d initial tabs", tabWorkingSetSize)
	urlIndex := 0
	for i := 0; i < tabWorkingSetSize; i++ {
		renderer, err := addTabQuiesce(ctx, s, cr, tabURLs[urlIndex], quiescenceCode)
		urlIndex = (1 + urlIndex) % len(tabURLs)
		if err != nil {
			s.Fatal("Cannot add initial tab from list: ", err)
		}
		defer renderer.conn.Close()
		newRenderers = append(newRenderers, renderer)
		renderers[renderer.tabID] = renderer
		lightSleep(ctx, newTabDelay)
	}
	// Allocate memory by opening more tabs and cycling through recently
	// opened tabs until a tab discard occurs.
	var validTabIDs []int
	for {
		validTabIDs = getValidTabIDs(ctx, s, cr)
		s.Logf("Cycling tabs (opened %v, present %v, initial %v",
			len(newRenderers), len(validTabIDs), initialTabCount)
		if len(newRenderers)+initialTabCount > len(validTabIDs) {
			s.Log("Ending allocation because one or more targets (tabs) have gone")
			break
		}
		for i := 0; i < tabWorkingSetSize; i++ {
			recent := i + len(validTabIDs) - tabWorkingSetSize
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
		renderer, err := addTabQuiesce(ctx, s, cr, tabURLs[urlIndex], quiescenceCode)
		urlIndex = (1 + urlIndex) % len(tabURLs)
		if err != nil {
			s.Fatal("Cannot add tab from list: ", err)
		}
		defer renderer.conn.Close()
		newRenderers = append(newRenderers, renderer)
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
	totalPageFaultCount1Metric := perf.Metric{
		Name:      "tast_total_page_fault_count_1",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	averagePageFaultRate1Metric := perf.Metric{
		Name:      "tast_average_page_fault_rate_1",
		Unit:      "faults_per_second",
		Direction: perf.SmallerIsBetter,
	}
	maxPageFaultRate1Metric := perf.Metric{
		Name:      "tast_max_page_fault_rate_1",
		Unit:      "faults_per_second",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(openedTabsMetric, float64(len(newRenderers)))
	lostTabs := len(newRenderers) + initialTabCount - len(validTabIDs)
	perfValues.Set(lostTabsMetric, float64(lostTabs))
	totalPageFaultCount, averagePageFaultRate, maxPageFaultRate := pageFaultMeterGetStats(ctx, s)
	perfValues.Set(totalPageFaultCount1Metric, float64(totalPageFaultCount))
	perfValues.Set(averagePageFaultRate1Metric, averagePageFaultRate)
	perfValues.Set(maxPageFaultRate1Metric, maxPageFaultRate)
	s.Logf("Metrics: Phase 1: opened tab count %v", len(newRenderers))
	s.Logf("Metrics: Phase 1: lost tab count %v", lostTabs)
	s.Logf("Metrics: Phase 1: total page fault count %v", totalPageFaultCount)
	s.Logf("Metrics: Phase 1: average page fault rate %v", averagePageFaultRate)
	s.Logf("Metrics: Phase 1: max page fault rate %v", maxPageFaultRate)
	times := tabSwitchTimesMs
	s.Logf("Metrics: Phase 1: mean tab switch time %v", mean(times))
	s.Logf("Metrics: Phase 1: stddev of tab switch times %v", stdDev(times))

	// Phase 2: quiesce.
	lightSleep(ctx, 5*time.Second)
	pageFaultMeterReset(s)
	lightSleep(ctx, 1*time.Minute)
	totalPageFaultCount, averagePageFaultRate, maxPageFaultRate = pageFaultMeterGetStats(ctx, s)
	totalPageFaultCount2Metric := perf.Metric{
		Name:      "tast_total_pagefault_count_2",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}
	averagePageFaultRate2Metric := perf.Metric{
		Name:      "tast_average_pageFault_rate_2",
		Unit:      "faults_per_second",
		Direction: perf.SmallerIsBetter,
	}
	maxPageFaultRate2Metric := perf.Metric{
		Name:      "tast_max_pageFault_rate_2",
		Unit:      "faults_per_second",
		Direction: perf.SmallerIsBetter,
	}
	perfValues.Set(totalPageFaultCount2Metric, float64(totalPageFaultCount))
	perfValues.Set(averagePageFaultRate2Metric, averagePageFaultRate)
	perfValues.Set(maxPageFaultRate2Metric, maxPageFaultRate)
	s.Logf("Metrics: Phase 2: total page fault count %v", totalPageFaultCount)
	s.Logf("Metrics: Phase 2: average page fault rate %v", averagePageFaultRate)
	s.Logf("Metrics: Phase 2: max page fault rate %v", maxPageFaultRate)

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
