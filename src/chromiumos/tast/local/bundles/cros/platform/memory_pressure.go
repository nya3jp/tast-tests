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
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/kernelmeter"
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
		Data:         []string{wprArchiveName, "dormant.js"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

// List of URLs to visit in the test.
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

// wprArchiveName is the external file name for the wpr archive.
const wprArchiveName = "memory_pressure_mixed_sites.wprgo"

// renderer represents a Chrome renderer creaded by devtools.  Such renderer is
// associated with a single tab, whose ID is also in this struct.
type renderer struct {
	conn  *chrome.Conn
	tabID int
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

// evalPromiseBody executes a JS promise on connection conn.  promiseBody
// is the code run as a promise, and it must contain a call to resolve().
// Returns in out a value whose type must match the type of the object
// passed to resolve().
func evalPromiseBody(ctx context.Context, conn *chrome.Conn,
	promiseBody string, out interface{}) error {
	promise := fmt.Sprintf("new Promise((resolve, reject) => { %s });", promiseBody)
	if err := conn.EvalPromise(ctx, promise, out); err != nil {
		return errors.Wrapf(err, "cannot execute promise (%s)", promise)
	}
	return nil
}

// execPromiseBody performs as above, but without the out parameter.
func execPromiseBody(ctx context.Context, conn *chrome.Conn,
	promiseBody string) error {
	return evalPromiseBody(ctx, conn, promiseBody, nil)
}

// evalPromiseBodyInBrowser performs as above, but executes the promise in the browser.
func evalPromiseBodyInBrowser(ctx context.Context, cr *chrome.Chrome, promiseBody string, out interface{}) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create test API connection")
	}
	return evalPromiseBody(ctx, tconn, promiseBody, out)
}

// execPromiseBodyInBrowser connects to Chrome and executes a JS promise
// which does not return a value.
func execPromiseBodyInBrowser(ctx context.Context, cr *chrome.Chrome, promiseBody string) error {
	return evalPromiseBodyInBrowser(ctx, cr, promiseBody, nil)
}

// getActiveTabID returns the tab ID for the currently active tab.
func getActiveTabID(ctx context.Context, cr *chrome.Chrome) (int, error) {
	const promiseBody = "chrome.tabs.query({active: true}, (tlist) => { resolve(tlist[0].id) })"
	var tabID int
	if err := evalPromiseBodyInBrowser(ctx, cr, promiseBody, &tabID); err != nil {
		return 0, errors.Wrap(err, "cannot get tabID")
	}
	return tabID, nil
}

// addTab creates a new renderer and the associated tab, which loads url.
// Returns the renderer instance.  If isDormantExpr is not empty, waits for the
// tab load to quiesce by executing the JS code in isDormantExpr until it
// returns true.  If rset is not nil, and there are no errors, the tab is added to rset.
func addTab(ctx context.Context, cr *chrome.Chrome, rset *rendererSet, url, isDormantExpr string) (*renderer, error) {
	conn, err := cr.NewConn(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new renderer")
	}
	tabID, err := getActiveTabID(ctx, cr)
	if err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "cannot get tab id for new renderer")
	}
	r := &renderer{
		conn:  conn,
		tabID: tabID,
	}
	// Optionally add r and tabID to rset before returning.  If errors
	// have occurred, r.conn is closed and r is set to nil.
	defer func() {
		if r != nil && rset != nil {
			rset.add(tabID, r)
		}
	}()

	if isDormantExpr == "" {
		return r, nil
	}

	// Wait for tab load to become dormant.  Ignore timeouts.
	const tabLoadTimeout = 20 * time.Second
	waitCtx, cancel := context.WithTimeout(ctx, tabLoadTimeout)
	defer cancel()
	startTime := time.Now()
	if err = r.conn.WaitForExprFailOnErr(waitCtx, isDormantExpr); err != nil {
		if waitCtx.Err() == context.DeadlineExceeded {
			testing.ContextLogf(ctx, "Ignoring tab quiesce timeout (%v)", tabLoadTimeout)
			return r, nil
		}
		r.conn.Close()
		r = nil
		return nil, errors.Wrap(err, "unexpected error waiting for tab quiesce")
	}
	testing.ContextLog(ctx, "Tab quiesce time: ", time.Now().Sub(startTime))
	return r, nil
}

// activateTab activates the tab for tabID, i.e. it selects the tab and brings
// it to the foreground (equivalent to clicking on the tab).  Returns the time
// it took to perform the switch.
func activateTab(ctx context.Context, cr *chrome.Chrome, tabID int, r *renderer) (time.Duration, error) {
	code := fmt.Sprintf(`chrome.tabs.update(%d, {active: true}, () => { resolve() })`, tabID)
	startTime := time.Now()
	if err := execPromiseBodyInBrowser(ctx, cr, code); err != nil {
		return 0, err
	}
	const promiseBody = `
// Code which calls resolve() when a tab frame has been rendered.
(function () {
  // We wait for two calls to requestAnimationFrame. When the first
  // requestAnimationFrame is called, we know that a frame is in the
  // pipeline. When the second requestAnimationFrame is called, we know that
  // the first frame has reached the screen.
  let frameCount = 0;
  const waitForRaf = function() {
    frameCount++;
    if (frameCount == 2) {
      resolve();
    } else {
      window.requestAnimationFrame(waitForRaf);
    }
  };
  window.requestAnimationFrame(waitForRaf);
})()
`
	if err := execPromiseBody(ctx, r.conn, promiseBody); err != nil {
		return 0, err
	}
	switchTime := time.Now().Sub(startTime)
	testing.ContextLogf(ctx, "Tab switch time for tab %3d: %7.2f ms", tabID, switchTime.Seconds()*1000)
	return switchTime, nil
}

// getValidTabIDs returns a list of non-discarded tab IDs.
func getValidTabIDs(ctx context.Context, cr *chrome.Chrome) ([]int, error) {
	var out []int
	const promiseBody = `
chrome.tabs.query({discarded: false}, function(tabList) {
	resolve(tabList.map((tab) => tab.id))
});
`
	if err := evalPromiseBodyInBrowser(ctx, cr, promiseBody, &out); err != nil {
		return nil, errors.Wrap(err, "cannot query tab list")
	}
	return out, nil
}

// emulateTyping emulates typing from some layer outside the browser.
func emulateTyping(ctx context.Context, cr *chrome.Chrome,
	r *renderer, text string) error {
	testing.ContextLog(ctx, "Finding and opening keyboard device")
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

// waitForElement waits until the DOM element specified by selector appears in
// the tab backed by rendered r.
func waitForElement(ctx context.Context, r *renderer, selector string) error {
	queryCode := fmt.Sprintf("resolve(document.querySelector(%q) !== null)", selector)

	// Wait for element to appear.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var pageReady bool
		err := evalPromiseBody(ctx, r.conn, queryCode, &pageReady)
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

// focusElement places keyboard input focus on the DOM specified by
// selector in the tab backed by renderer r.
func focusElement(ctx context.Context, r *renderer, selector string) error {
	focusCode := fmt.Sprintf("document.querySelector(%q).focus();", selector)
	return r.conn.Exec(ctx, focusCode)
}

// googleLogin logs onto GAIA (NOT WORKING YET).
func googleLogIn(ctx context.Context, cr *chrome.Chrome) error {
	const loginURL = "https://accounts.google.com/ServiceLogin?continue=https%3A%2F%2Faccounts.google.com%2FManageAccount"
	loginTab, err := addTab(ctx, cr, nil, loginURL, "")
	if err != nil {
		return errors.Wrap(err, "cannot add login tab")
	}
	defer loginTab.conn.Close()
	// Existing Telemetry code uses this more precise selector:
	// "input[type=email]:not([aria-hidden=true]),#Email:not(.hidden)"
	// but I am not sure it is necessary or even correct.
	const emailSelector = "input[type=email]"
	if err := waitForElement(ctx, loginTab, emailSelector); err != nil {
		return errors.Wrap(err, "email entry field not found")
	}
	// Get focus on email field.
	if err := focusElement(ctx, loginTab, emailSelector); err != nil {
		return errors.Wrap(err, "cannot focus on email entry field")
	}
	lightSleep(ctx, 5*time.Second)
	// Enter email.
	if err = emulateTyping(ctx, cr, loginTab, "wpr.memory.pressure.test@gmail.com"); err != nil {
		return errors.Wrap(err, "cannot enter login name")
	}
	testing.ContextLog(ctx, "Email entered")
	lightSleep(ctx, 1*time.Second)
	if err = emulateTyping(ctx, cr, loginTab, "\n"); err != nil {
		return errors.Wrap(err, "cannot enter login name")
	}
	const passwordSelector = "input[type=password]"
	// TODO: need to figure out why waitForElement below is not sufficient
	// to properly delay further input.
	lightSleep(ctx, 5*time.Second)
	// Wait for password prompt.
	if err := waitForElement(ctx, loginTab, passwordSelector); err != nil {
		return errors.Wrap(err, "password field not found")
	}
	// Focus on password field.
	if err := focusElement(ctx, loginTab, passwordSelector); err != nil {
		return errors.Wrap(err, "cannot focus on password field")
	}
	// Enter password.
	if err = emulateTyping(ctx, cr, loginTab, "google.memory.chrome"); err != nil {
		return errors.Wrap(err, "cannot enter password")
	}
	testing.ContextLog(ctx, "Password entered")
	// TODO: figure out if and why this wait is needed.
	lightSleep(ctx, 5*time.Second)
	if err = emulateTyping(ctx, cr, loginTab, "\n"); err != nil {
		return errors.Wrap(err, "cannot enter 'enter'")
	}
	// TODO: figure out if and why this wait is needed.
	lightSleep(ctx, 10*time.Second)
	return nil
}

// wiggleTab scrolls the main window down in short steps, then jumps back up.
// If the main window is not scrollable, it does nothing.
func wiggleTab(ctx context.Context, r *renderer) error {
	const (
		scrollCount  = 50
		scrollDelay  = 50 * time.Millisecond
		scrollAmount = 100
	)
	scrollDownCode := fmt.Sprintf("window.scrollBy(0, %d)", scrollAmount)
	scrollUpCode := fmt.Sprintf("window.scrollBy(0, -%d)", scrollAmount*scrollCount)

	for i := 0; i < scrollCount; i++ {
		if err := r.conn.Exec(ctx, scrollDownCode); err != nil {
			return errors.Wrap(err, "scroll down failed")
		}
		lightSleep(ctx, scrollDelay)
	}
	if err := r.conn.Exec(ctx, scrollUpCode); err != nil {
		return errors.Wrap(err, "scroll up failed")
	}
	lightSleep(ctx, scrollDelay)
	return nil
}

// lightSleep pauses execution for time span t, or less if a timeout intervenes.
func lightSleep(ctx context.Context, t time.Duration) {
	select {
	case <-time.After(t):
	case <-ctx.Done():
	}
}

// rendererSet maintains a set of renderers and tab IDs in the order in which
// they are added.  Only "working" tabs are included, i.e. tabs may exist
// outside of this structure but they are mostly ignored in the test.
type rendererSet struct {
	// tabIDs is an array of tab IDs in the order in which new tabs are added.
	tabIDs []int
	// renderersByTabID maps a tab ID to its renderer struct.  The initial
	// tab is not included.
	renderersByTabID map[int]*renderer
}

// add adds a new tab/renderer to rset.
func (rset *rendererSet) add(id int, r *renderer) {
	rset.tabIDs = append(rset.tabIDs, id)
	rset.renderersByTabID[id] = r
}

// waitForTCPSocket tries to connect to socket, which is a string in the form
// "host:port", e.g. "localhost:8080"
func waitForTCPSocket(ctx context.Context, socket string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		conn, err := net.Dial("tcp", socket)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}, &testing.PollOptions{
		Interval: 1 * time.Second,
		Timeout:  60 * time.Second,
	})
}

// initBrowser restarts the browser on the DUT in preparation for testing.
func initBrowser(ctx context.Context, useLiveSites bool, wprArchivePath string) (*chrome.Chrome, *testexec.Cmd, error) {
	const (
		httpPort  = 8080
		httpsPort = 8081
	)
	var (
		tentativeCr  *chrome.Chrome
		tentativeWPR *testexec.Cmd
	)
	defer func() {
		if tentativeCr != nil {
			tentativeCr.Close(ctx)
		}
		if tentativeWPR != nil {
			if err := tentativeWPR.Kill(); err != nil {
				testing.ContextLog(ctx, "cannot kill WPR: ", err)
			}
		}
	}()

	if useLiveSites {
		testing.ContextLog(ctx, "Starting Chrome with live sites")
		cr, err := chrome.New(ctx)
		return cr, nil, err
	}

	testing.ContextLog(ctx, "Starting Chrome with WPR")
	// Start the Web Page Replay in replay mode.
	//
	// This test can be modified to record a page set, roughly as follows:
	//
	// -- increase the tabLoadTimeout to 40 or 50 seconds
	// -- also increase newTabDelay to 10 or 20 seconds
	// -- prevent the test from starting wpr (because wpr will not save
	// the archive when killed by the Go library); start it
	// manually instead with:
	//
	//   wpr record --http_port=8080 --https_port=8081 --https_cert_file=/usr/share/wpr/wpr_cert.pem \
	//   --https_key_file=/usr/share/wpr/wpr_key.pem --inject_scripts=/usr/share/wpr/deterministic.js /tmp/archive.wprgo
	//
	// (the required files are installed by the test.)
	// When the test has finished loading the last URL from tabURLs, kill wpr with ^C.
	// At that point wpr will save the archive (it is all in RAM before that)
	//
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
	testing.ContextLog(ctx, "Using WPR archive ", wprArchivePath)
	tentativeWPR = testexec.CommandContext(ctx, "wpr", "replay",
		fmt.Sprintf("--http_port=%d", httpPort),
		fmt.Sprintf("--https_port=%d", httpsPort),
		"--https_cert_file=/usr/share/wpr/wpr_cert.pem",
		"--https_key_file=/usr/share/wpr/wpr_key.pem",
		"--inject_scripts=/usr/share/wpr/deterministic.js",
		wprArchivePath)

	if err := tentativeWPR.Start(); err != nil {
		tentativeWPR.DumpLog(ctx)
		return nil, nil, errors.Wrap(err, "cannot start WPR")
	}

	// Restart chrome for use with WPR.  Chrome can start before WPR is
	// ready because it will not need it until we start opening tabs.
	resolverRules := fmt.Sprintf("MAP *:80 127.0.0.1:%d,MAP *:443 127.0.0.1:%d,EXCLUDE localhost",
		httpPort, httpsPort)
	resolverRulesFlag := fmt.Sprintf("--host-resolver-rules=%q", resolverRules)
	spkiList := "PhrPvGIaAMmd29hj8BCZOq096yj7uMpRNHpn5PDxI6I="
	spkiListFlag := fmt.Sprintf("--ignore-certificate-errors-spki-list=%s", spkiList)
	extraArgs := []string{resolverRulesFlag, spkiListFlag}
	tentativeCr, err := chrome.New(ctx, chrome.ExtraArgs(extraArgs))
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot start Chrome")
	}

	// Wait for WPR to initialize.
	httpSocketName := fmt.Sprintf("localhost:%d", httpPort)
	httpsSocketName := fmt.Sprintf("localhost:%d", httpsPort)
	if err := waitForTCPSocket(ctx, httpSocketName); err != nil {
		return nil, nil, errors.Wrapf(err, "cannot connect to WPR at %s", httpSocketName)
	}
	testing.ContextLog(ctx, "WPR HTTP socket is up at ", httpSocketName)
	if err := waitForTCPSocket(ctx, httpsSocketName); err != nil {
		return nil, nil, errors.Wrapf(err, "cannot connect to WPR at %s", httpsSocketName)
	}
	testing.ContextLog(ctx, "WPR HTTPS socket is up at ", httpsSocketName)
	cr := tentativeCr
	tentativeCr = nil
	wpr := tentativeWPR
	tentativeWPR = nil
	return cr, wpr, nil
}

// pinTabs pins each tab in tabIDs.  This makes them less likely to be chosen
// as discard candidates.
func pinTabs(ctx context.Context, cr *chrome.Chrome, tabIDs []int) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Cannot get test connection: ", err)
	}
	// Pin tabs
	for _, id := range tabIDs {
		pinCode := fmt.Sprintf("chrome.tabs.update(%d, {pinned: true})", id)
		if err := tconn.Exec(ctx, pinCode); err != nil {
			testing.ContextLogf(ctx, "Cannot pin tab %d: %v", id, err)
		}
	}
}

// cycleTabs activates in turn each tab passed in tabIDs, then it wiggles it or
// pauses for a duration of pause, depending on the value of wiggle.  It
// returns a slice of tab switch times.
func cycleTabs(ctx context.Context, cr *chrome.Chrome, tabIDs []int, rset *rendererSet,
	pause time.Duration, wiggle bool) ([]time.Duration, error) {
	var times []time.Duration
	for _, id := range tabIDs {
		r := rset.renderersByTabID[id]
		t, err := activateTab(ctx, cr, id, r)
		if err != nil {
			// Likely tab was discarded and connection was closed.
			return times, errors.Wrapf(err, "cannot activate tab %d", id)
		}
		times = append(times, t)
		if wiggle {
			if err := wiggleTab(ctx, r); err != nil {
				// Here it is also possible to get a "connection closed" error.
				return times, errors.Wrapf(err, "cannot wiggle tab %d", id)
			}
		} else {
			lightSleep(ctx, pause)
		}
	}
	return times, nil
}

// runTabSwitches performs multiple set of tab switches through the tabs in
// tabIDs, and logs switch times and their stats.
func runTabSwitches(ctx context.Context, cr *chrome.Chrome, rset *rendererSet,
	tabIDs []int, label string, repeatCount int) error {
	// Cycle through the tabs once to warm them up (no wiggling).
	if _, err := cycleTabs(ctx, cr, tabIDs, rset, time.Second, false); err != nil {
		return errors.Wrap(err, "cannot warm-up initial set of tabs")
	}
	// Cycle through tabs a few times, still without wiggling, and collect
	// the switch times.
	var switchTimes []time.Duration
	const shortTabSwitchDelay = 200 * time.Millisecond
	for i := 0; i < repeatCount; i++ {
		times, err := cycleTabs(ctx, cr, tabIDs, rset, shortTabSwitchDelay, false)
		if err != nil {
			return errors.Wrap(err, "failed to run tab switches")
		}
		switchTimes = append(switchTimes, times...)
	}

	testing.ContextLogf(ctx, "Metrics: %s: mean/stddev of switch times for all tabs: %7.2f %7.2f (ms)",
		label, mean(switchTimes).Seconds()*1000, stdDev(switchTimes).Seconds()*1000)

	// Log tab switch stats on a per-tab basis.
	tabTimes := make([][]time.Duration, len(tabIDs))
	for i, t := 0, 0; i < len(switchTimes); i++ {
		tabTimes[t] = append(tabTimes[t], switchTimes[i])
		t = (t + 1) % len(tabIDs)
	}
	for i, times := range tabTimes {
		testing.ContextLogf(ctx, "Metrics: %s: mean/stdev switch time for tab index %d: %7.2f %7.2f (ms)",
			label, i, mean(times).Seconds()*1000, stdDev(times).Seconds()*1000)
	}
	return nil
}

// MemoryPressure is the main test function.
func MemoryPressure(ctx context.Context, s *testing.State) {
	const (
		useLogIn             = false
		useLiveSites         = false
		tabWorkingSetSize    = 5
		newTabDelay          = 0 * time.Second
		tabCycleDelay        = 300 * time.Millisecond
		tabSwitchRepeatCount = 10
	)

	rset := &rendererSet{renderersByTabID: make(map[int]*renderer)}

	// Create and start the performance meter.
	kernelMeter := kernelmeter.New(ctx)
	defer kernelMeter.Close(ctx)

	// Load the JS expression that checks if a load has become dormant.
	bytes, err := ioutil.ReadFile(s.DataPath("dormant.js"))
	if err != nil {
		s.Fatal("Cannot read dormant.js: ", err)
	}
	isDormantExpr := string(bytes)

	perfValues := &perf.Values{}

	cr, wpr, err := initBrowser(ctx, useLiveSites, s.DataPath(wprArchiveName))
	if err != nil {
		s.Fatal("Cannot start browser: ", err)
	}
	defer cr.Close(ctx)
	defer func() {
		if wpr == nil {
			return
		}
		defer wpr.Wait()
		if err := wpr.Kill(); err != nil {
			s.Fatal("cannot kill WPR")
		}
	}()

	// Log in.  TODO(semenzato): this is not working (yet), we would like
	// to have it for gmail and similar.
	if useLogIn {
		s.Log("Logging in")
		if err := googleLogIn(ctx, cr); err != nil {
			s.Fatal("Cannot login to google: ", err)
		}
	}

	// Figure out how many tabs already exist (typically 1).
	validTabIDs, err := getValidTabIDs(ctx, cr)
	if err != nil {
		s.Fatal("Cannot get tab list: ", err)
	}
	initialTabCount := len(validTabIDs)

	// Open enough tabs for a "working set", i.e. the number of tabs that an
	// imaginary user will cycle through in their imaginary workflow.
	s.Logf("Opening %d initial tabs", tabWorkingSetSize)
	urlIndex := 0
	for i := 0; i < tabWorkingSetSize; i++ {
		renderer, err := addTab(ctx, cr, rset, tabURLs[urlIndex], isDormantExpr)
		urlIndex = (1 + urlIndex) % len(tabURLs)
		if err != nil {
			s.Fatal("Cannot add initial tab from list: ", err)
		}
		defer renderer.conn.Close()
		if err := wiggleTab(ctx, renderer); err != nil {
			s.Error("Cannot wiggle initial tab: ", err)
		}
	}
	workingTabIDs := rset.tabIDs[:tabWorkingSetSize]
	pinTabs(ctx, cr, workingTabIDs)
	// Collect and log tab-switching times in the absence of memory pressure.
	if err := runTabSwitches(ctx, cr, rset, workingTabIDs, "light", tabSwitchRepeatCount); err != nil {
		s.Error("Cannot run tab switches with light load: ", err)
	}

	var allTabSwitchTimes []time.Duration
	// Allocate memory by opening more tabs and cycling through recently
	// opened tabs until a tab discard occurs.
	for {
		validTabIDs, err = getValidTabIDs(ctx, cr)
		if err != nil {
			s.Fatal("Cannot get tab list: ", err)
		}
		s.Logf("Cycling tabs (opened %v, present %v, initial %v)",
			len(rset.tabIDs), len(validTabIDs), initialTabCount)
		if len(rset.tabIDs)+initialTabCount > len(validTabIDs) {
			s.Log("Ending allocation because one or more targets (tabs) have gone")
			break
		}
		// Switch among recently loaded tabs to encourage loading.
		// Errors are likely due to the tab having been discarded, so
		// we will catch the discard next time around the loop.  Log
		// the error and ignore it.
		recentTabs := rset.tabIDs[len(rset.tabIDs)-tabWorkingSetSize:]
		times, err := cycleTabs(ctx, cr, recentTabs, rset, time.Second, true)
		if err != nil {
			s.Log("Tab cycling error: ", err)
		}
		allTabSwitchTimes = append(allTabSwitchTimes, times...)
		if len(rset.tabIDs)%tabWorkingSetSize == tabWorkingSetSize-1 {
			// Quickly switch among initial set of tabs to position
			// them high in the LRU list.
			s.Log("Refreshing LRU order of initial tab set")
			if _, err := cycleTabs(ctx, cr, rset.tabIDs[0:tabWorkingSetSize], rset, 0, false); err != nil {
				s.Log("Tab LRU refresh error: ", err)
			}
		}
		renderer, err := addTab(ctx, cr, rset, tabURLs[urlIndex], isDormantExpr)
		urlIndex = (1 + urlIndex) % len(tabURLs)
		if err != nil {
			s.Fatal("Cannot add tab from list: ", err)
		}
		defer renderer.conn.Close()
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
	perfValues.Set(openedTabsMetric, float64(len(rset.tabIDs)))
	lostTabs := len(rset.tabIDs) + initialTabCount - len(validTabIDs)
	perfValues.Set(lostTabsMetric, float64(lostTabs))
	stats, err := kernelMeter.PageFaultStats()
	if err != nil {
		s.Error("Cannot compute page fault stats: ", err)
	}
	perfValues.Set(totalPageFaultCount1Metric, float64(stats.Count))
	perfValues.Set(averagePageFaultRate1Metric, stats.AverageRate)
	perfValues.Set(maxPageFaultRate1Metric, stats.MaxRate)
	s.Log("Metrics: Phase 1: opened tab count ", len(rset.tabIDs))
	s.Log("Metrics: Phase 1: lost tab count ", lostTabs)
	s.Log("Metrics: Phase 1: total page fault count ", stats.Count)
	s.Logf("Metrics: Phase 1: average page fault rate %v pf/second", stats.AverageRate)
	s.Logf("Metrics: Phase 1: max page fault rate %v pf/second", stats.MaxRate)
	times := allTabSwitchTimes
	s.Logf("Metrics: Phase 1: mean tab switch time %7.2f ms", mean(times).Seconds()*1000)
	s.Logf("Metrics: Phase 1: stddev of tab switch times %7.2f ms", stdDev(times).Seconds()*1000)

	// -----------------
	// Phase 2: quiesce.
	// -----------------
	// Wait a bit to help the system stabilize.
	lightSleep(ctx, 10*time.Second)
	kernelMeter.Reset()
	// Measure tab switching under pressure.
	if err := runTabSwitches(ctx, cr, rset, workingTabIDs, "heavy", tabSwitchRepeatCount); err != nil {
		s.Error("Cannot run tab switches with heavy load: ", err)
	}
	stats, err = kernelMeter.PageFaultStats()
	if err != nil {
		s.Error("Cannot compute page fault stats (phase 2): ", err)
	}
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
	perfValues.Set(totalPageFaultCount2Metric, float64(stats.Count))
	perfValues.Set(averagePageFaultRate2Metric, stats.AverageRate)
	perfValues.Set(maxPageFaultRate2Metric, stats.MaxRate)
	s.Log("Metrics: Phase 2: total page fault count ", stats.Count)
	s.Logf("Metrics: Phase 2: average page fault rate %v pf/second", stats.AverageRate)
	s.Logf("Metrics: Phase 2: max page fault rate %v pf/second", stats.MaxRate)

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
