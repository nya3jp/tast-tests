// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package mempressure creates a realistic memory pressure situation and takes
// related measurements.
package mempressure

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/shirou/gopsutil/mem"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/platform/kernelmeter"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	// CompressibleData is a file containing compressible data for preallocation.
	CompressibleData = "memory_pressure_page.lzo.40"
	// DormantCode is JS code that detects the end of a page load.
	DormantCode = "memory_pressure_dormant.js"
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

// evalInBrowser evaluates synchronous code in the browser.
func evalInBrowser(ctx context.Context, cr *chrome.Chrome, code string, out interface{}) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create test API connection")
	}
	return tconn.Eval(ctx, code, out)
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
// returns true, or timeout is reached.  If rset is not nil, and there are no
// errors, the tab is added to rset.
func addTab(ctx context.Context, cr *chrome.Chrome, rset *rendererSet, url, isDormantExpr string, timeout time.Duration) (*renderer, error) {
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

	// Wait for tab load to become dormant.  Ignore timeout expiration.
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	startTime := time.Now()
	if err = r.conn.WaitForExprFailOnErr(waitCtx, isDormantExpr); err != nil {
		if waitCtx.Err() == context.DeadlineExceeded {
			testing.ContextLogf(ctx, "Ignoring tab quiesce timeout (%v)", timeout)
			return r, nil
		}
		r.conn.Close()
		r = nil
		return nil, errors.Wrap(err, "unexpected error waiting for tab quiesce")
	}
	testing.ContextLog(ctx, "Tab quiesce time: ", time.Now().Sub(startTime))
	return r, nil
}

// tabIsDiscarded returns true if the tab with ID tabID was discarded.
func tabIsDiscarded(ctx context.Context, cr *chrome.Chrome, tabID int) (bool, error) {
	valid, err := getValidTabIDs(ctx, cr)
	if err != nil {
		return false, err
	}
	for _, v := range valid {
		if v == tabID {
			return false, nil
		}
	}
	return true, nil
}

// activateTab activates the tab for tabID, i.e. it selects the tab and brings
// it to the foreground (equivalent to clicking on the tab).  Returns whether
// the activation succeeds and the time it took to perform the switch.
// Tolerates an activation failure if the tab was discarded and returns false
// but no error in this case.
func activateTab(ctx context.Context, cr *chrome.Chrome, tabID int, r *renderer) (bool, time.Duration, error) {
	code := fmt.Sprintf(`chrome.tabs.update(%d, {active: true}, () => { resolve() })`, tabID)
	startTime := time.Now()
	if err := execPromiseBodyInBrowser(ctx, cr, code); err != nil {
		return false, 0, err
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
	// Sometimes tabs crash and the devtools connection goes away.  To avoid waiting 30 minutes
	// for this we use a shorter timeout.
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := execPromiseBody(waitCtx, r.conn, promiseBody); err != nil {
		// Check if the tab was discarded, and if so blame the error on
		// the discard and ignore it.
		discarded, innerErr := tabIsDiscarded(ctx, cr, tabID)
		if innerErr != nil {
			return false, 0, errors.Wrap(innerErr, "failed to verify discard status")
		}
		if discarded {
			testing.ContextLogf(ctx, "Tab %d is discarded", tabID)
			return false, 0, nil
		}
		// Some other type of error occurred.
		return false, 0, err
	}
	switchTime := time.Now().Sub(startTime)
	testing.ContextLogf(ctx, "Tab switch time for tab %3d: %7.2f ms", tabID, switchTime.Seconds()*1000)
	return true, switchTime, nil
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

// logScreenDimensions logs width and height of the current window (outer
// dimensions) and screen.
func logScreenDimensions(ctx context.Context, cr *chrome.Chrome) error {
	var out []int
	const code = `[window.outerWidth, window.outerHeight, window.screen.width, window.screen.height]`
	if err := evalInBrowser(ctx, cr, code, &out); err != nil {
		return err
	}
	testing.ContextLogf(ctx, "Display: window %vx%v, screen %vx%v", out[0], out[1], out[2], out[3])
	return nil
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

// googleLogIn logs onto GAIA (NOT WORKING YET).
func googleLogIn(ctx context.Context, cr *chrome.Chrome) error {
	const loginURL = "https://accounts.google.com/ServiceLogin?continue=https%3A%2F%2Faccounts.google.com%2FManageAccount"
	loginTab, err := addTab(ctx, cr, nil, loginURL, "", 0)
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
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return err
	}
	// Enter email.
	if err := emulateTyping(ctx, cr, loginTab, "wpr.memory.pressure.test@gmail.com"); err != nil {
		return errors.Wrap(err, "cannot enter login name")
	}
	testing.ContextLog(ctx, "Email entered")
	if err := testing.Sleep(ctx, 1*time.Second); err != nil {
		return err
	}
	if err := emulateTyping(ctx, cr, loginTab, "\n"); err != nil {
		return errors.Wrap(err, "cannot enter login name")
	}
	const passwordSelector = "input[type=password]"
	// TODO: need to figure out why waitForElement below is not sufficient
	// to properly delay further input.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
	}
	// Wait for password prompt.
	if err := waitForElement(ctx, loginTab, passwordSelector); err != nil {
		return errors.Wrap(err, "password field not found")
	}
	// Focus on password field.
	if err := focusElement(ctx, loginTab, passwordSelector); err != nil {
		return errors.Wrap(err, "cannot focus on password field")
	}
	// Enter password.
	if err := emulateTyping(ctx, cr, loginTab, "google.memory.chrome"); err != nil {
		return errors.Wrap(err, "cannot enter password")
	}
	testing.ContextLog(ctx, "Password entered")
	// TODO: figure out if and why this wait is needed.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return err
	}
	if err := emulateTyping(ctx, cr, loginTab, "\n"); err != nil {
		return errors.Wrap(err, "cannot enter 'enter'")
	}
	// TODO: figure out if and why this wait is needed.
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		return err
	}
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
		if err := testing.Sleep(ctx, scrollDelay); err != nil {
			return err
		}
	}
	if err := r.conn.Exec(ctx, scrollUpCode); err != nil {
		return errors.Wrap(err, "scroll up failed")
	}
	if err := testing.Sleep(ctx, scrollDelay); err != nil {
		return err
	}
	return nil
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

// waitForServerSocket tries to connect to a TCP socket, which is a string in
// the form "host:port", e.g. "localhost:8080", served by server, which is an
// already-started server process. If connecting to the socket fails,
// server.DumpLog is called to log more information.
func waitForServerSocket(ctx context.Context, socket string, server *testexec.Cmd) error {
	err := testing.Poll(ctx, func(ctx context.Context) error {
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
	if err != nil {
		// Try to collect the server log to understand why we could not connect.
		if dumpErr := server.DumpLog(ctx); dumpErr != nil {
			// Log error but do not return it since the earlier
			// error is more informative.
			testing.ContextLog(ctx, "Could not dump server log: ", dumpErr)
		}
	}
	return err
}

// availableTCPPorts returns a list of TCP ports on localhost that are not in
// use.  Returns an error if one or more ports cannot be allocated.  Note that
// the ports are not reserved, but chances that they remain available for at
// least a short time after this call are very high.
func availableTCPPorts(count int) ([]int, error) {
	var ls []net.Listener
	defer func() {
		for _, l := range ls {
			l.Close()
		}
	}()
	var ports []int
	for i := 0; i < count; i++ {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	return ports, nil
}

// InitChrome restarts Chrome on the DUT. It returns a Chrome pointer and
// Chrome ports used later to start WPR.
func InitChrome(ctx context.Context, p *RunParameters) (*chrome.Chrome, []int, error) {
	ports, err := availableTCPPorts(2)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot allocate WPR ports")
	}
	httpPort := ports[0]
	httpsPort := ports[1]

	testing.ContextLogf(ctx, "Starting Chrome with WPR at ports %d and %d", httpPort, httpsPort)

	// Restart chrome for use with WPR.  Chrome can start before WPR is
	// ready because it will not need it until we start opening tabs.
	resolverRules := fmt.Sprintf("MAP *:80 127.0.0.1:%d,MAP *:443 127.0.0.1:%d,EXCLUDE localhost",
		httpPort, httpsPort)
	resolverRulesFlag := fmt.Sprintf("--host-resolver-rules=%q", resolverRules)
	spkiList := "PhrPvGIaAMmd29hj8BCZOq096yj7uMpRNHpn5PDxI6I="
	spkiListFlag := fmt.Sprintf("--ignore-certificate-errors-spki-list=%s", spkiList)
	args := []string{resolverRulesFlag, spkiListFlag}
	if p.FakeLargeScreen {
		// The first flag makes Chrome create a larger window.  The
		// second flag is only effective if the device does not have a
		// display (chromeboxes).  Without it, Chrome uses a 1366x768
		// default.
		args = append(args, "--ash-host-window-bounds=3840x2048", "--screen-config=3840x2048/i")
	}
	var cr *chrome.Chrome
	if p.UseARC {
		cr, err = chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(args...))
	} else {
		cr, err = chrome.New(ctx, chrome.ExtraArgs(args...))
	}
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot start Chrome")
	}
	return cr, ports, nil

}

// initBrowser restarts the browser on the DUT in preparation for testing.  It
// returns a Chrome pointer, used for later communication with the browser, and
// a Cmd pointer for an already-started WPR process, which needs to be killed
// when the test ends (successfully or not).  If the returned error is not nil,
// the first two return values are nil.  The WPR process pointer is nil also
// when useLiveSites is true, since WPR is not started in that case.
//
// If recordPageSet is true, the test records a page set instead of replaying
// the pre-recorded set.
func initBrowser(ctx context.Context, p *RunParameters) (*chrome.Chrome, *testexec.Cmd, error) {
	var cr *chrome.Chrome
	var err error
	if p.ChromeInst != nil {
		cr = p.ChromeInst
	}
	if p.UseLiveSites {
		testing.ContextLog(ctx, "Starting Chrome with live sites")
		if p.ChromeInst == nil {
			cr, err = chrome.New(ctx)
		}
		return cr, nil, err
	}
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
				testing.ContextLog(ctx, "Cannot kill WPR: ", err)
			}
		}
	}()

	ports := p.ChromePorts

	if cr == nil {
		cr, ports, err = InitChrome(ctx, p)
		if err != nil {
			return nil, nil, err
		}
	}

	if len(ports) != 2 {
		ports, err = availableTCPPorts(2)
		if err != nil {
			return nil, nil, errors.Wrap(err, "cannot allocate WPR ports")
		}
	}

	httpPort := ports[0]
	httpsPort := ports[1]

	// Start the Web Page Replay process.  Normally this replays a supplied
	// WPR archive.  If recordPageSet is true, WPR records an archive
	// instead.
	//
	// The supplied WPR archive is stored in a private Google cloud storage
	// bucket and may not available in all setups.  In this case it must be
	// installed manually the first time the test is run on a DUT.  The GS
	// URL of the archive is contained in
	// data/memory_pressure_mixed_sites.wprgo.external.  The archive should
	// be copied to any location in the DUT (somewhere in /usr/local is
	// recommended) and the call to initBrowser should be updated to
	// reflect that location.
	mode := "replay"
	if p.RecordPageSet {
		mode = "record"
	}
	testing.ContextLog(ctx, "Using WPR archive ", p.WPRArchivePath)
	tentativeWPR = testexec.CommandContext(ctx, "wpr", mode,
		fmt.Sprintf("--http_port=%d", httpPort),
		fmt.Sprintf("--https_port=%d", httpsPort),
		"--https_cert_file=/usr/local/share/wpr/wpr_cert.pem",
		"--https_key_file=/usr/local/share/wpr/wpr_key.pem",
		"--inject_scripts=/usr/local/share/wpr/deterministic.js",
		p.WPRArchivePath)

	if err := tentativeWPR.Start(); err != nil {
		tentativeWPR.DumpLog(ctx)
		return nil, nil, errors.Wrap(err, "cannot start WPR")
	}

	// Wait for WPR to initialize.
	httpSocketName := fmt.Sprintf("localhost:%d", httpPort)
	httpsSocketName := fmt.Sprintf("localhost:%d", httpsPort)
	if err := waitForServerSocket(ctx, httpSocketName, tentativeWPR); err != nil {
		return nil, nil, errors.Wrapf(err, "cannot connect to WPR at %s", httpSocketName)
	}
	testing.ContextLog(ctx, "WPR HTTP socket is up at ", httpSocketName)
	if err := waitForServerSocket(ctx, httpsSocketName, tentativeWPR); err != nil {
		return nil, nil, errors.Wrapf(err, "cannot connect to WPR at %s", httpsSocketName)
	}
	testing.ContextLog(ctx, "WPR HTTPS socket is up at ", httpsSocketName)
	wpr := tentativeWPR
	tentativeWPR = nil
	return cr, wpr, nil
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
		success, t, err := activateTab(ctx, cr, id, r)
		if err != nil {
			return times, errors.Wrapf(err, "cannot activate tab %d", id)
		}
		if !success {
			continue
		}
		times = append(times, t)
		if wiggle {
			if err := wiggleTab(ctx, r); err != nil {
				return times, errors.Wrapf(err, "cannot wiggle tab %d", id)
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

// runTabSwitches performs multiple set of tab switches through the tabs in
// tabIDs, and logs switch times and their stats.
func runTabSwitches(ctx context.Context, cr *chrome.Chrome, rset *rendererSet,
	tabIDs []int, outDir, label string, repeatCount int) error {
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
	logTabSwitchTimes(ctx, switchTimes, len(tabIDs), outDir, label)
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
func runPhase1(ctx context.Context, s *testing.State, cr *chrome.Chrome, p *RunParameters, initialTabSetSize, recentTabSetSize, tabSwitchRepeatCount int, fullMeter *kernelmeter.Meter) ([]int, *rendererSet) {
	// Create and start the performance meters.  partialMeter takes
	// a measurement after the addition of each tab.  switchMeter
	// takes measurements around tab switches.
	partialMeter := kernelmeter.New(ctx)
	defer partialMeter.Close(ctx)
	switchMeter := kernelmeter.New(ctx)
	defer switchMeter.Close(ctx)

	// Load the JS expression that checks if a load has become dormant.
	bytes, err := ioutil.ReadFile(p.DormantCodePath)
	if err != nil {
		s.Fatal("Cannot read dormant JS code: ", err)
	}
	isDormantExpr := string(bytes)

	perfValues := perf.NewValues()

	rset := &rendererSet{renderersByTabID: make(map[int]*renderer)}

	// Figure out how many tabs already exist (typically 1).
	validTabIDs, err := getValidTabIDs(ctx, cr)
	if err != nil {
		s.Fatal("Cannot get tab list: ", err)
	}
	initialTabCount := len(validTabIDs)

	// Open enough tabs for a "working set", i.e. the number of tabs that an
	// imaginary user will cycle through in their imaginary workflow.
	s.Logf("Opening %d initial tabs", initialTabSetSize)
	tabLoadTimeout := 20 * time.Second
	if p.RecordPageSet {
		tabLoadTimeout = 50 * time.Second
	}
	urlIndex := 0
	for i := 0; i < initialTabSetSize; i++ {
		renderer, err := addTab(ctx, cr, rset, tabURLs[urlIndex], isDormantExpr, tabLoadTimeout)
		urlIndex = (1 + urlIndex) % len(tabURLs)
		if err != nil {
			s.Fatal("Cannot add initial tab from list: ", err)
		}

		if err := wiggleTab(ctx, renderer); err != nil {
			s.Error("Cannot wiggle initial tab: ", err)
		}
	}
	initialTabSetIDs := rset.tabIDs[:initialTabSetSize]
	pinTabs(ctx, cr, initialTabSetIDs)
	// Collect and log tab-switching times in the absence of memory pressure.
	if err := runTabSwitches(ctx, cr, rset, initialTabSetIDs, s.OutDir(), "light", tabSwitchRepeatCount); err != nil {
		s.Error("Cannot run tab switches with light load: ", err)
	}
	logAndResetStats(s, partialMeter, "initial")
	loggedMissingZramStats := false
	var allTabSwitchTimes []time.Duration
	// Allocate memory by opening more tabs and cycling through recently
	// opened tabs until a tab discard occurs.
	for {
		// When recording load each page only once.
		if p.RecordPageSet && len(rset.tabIDs) > len(tabURLs) {
			break
		}
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
		if p.MaxTabCount != 0 && len(rset.tabIDs) >= p.MaxTabCount {
			s.Log("MaxTabCount reached. Tab count: ", len(rset.tabIDs))
			break
		}
		// Switch among recently loaded tabs to encourage loading.
		// Errors are usually from a renderer crash or, less likely, a tab discard.
		// We fail in those cases because they are not expected.
		recentTabs := rset.tabIDs[len(rset.tabIDs)-recentTabSetSize:]
		times, err := cycleTabs(ctx, cr, recentTabs, rset, time.Second, true)
		if err != nil {
			s.Fatal("Tab cycling error: ", err)
		}
		allTabSwitchTimes = append(allTabSwitchTimes, times...)
		// Quickly switch among initial set of tabs to collect
		// measurements and position the tabs high in the LRU list.
		s.Log("Refreshing LRU order of initial tab set")
		runAndLogSwapStats(ctx, func() {
			if _, err := cycleTabs(ctx, cr, initialTabSetIDs, rset, 0, false); err != nil {
				s.Fatal("Tab LRU refresh error: ", err)
			}
		}, switchMeter)
		logPSIStats(s)
		_, err = addTab(ctx, cr, rset, tabURLs[urlIndex], isDormantExpr, tabLoadTimeout)
		urlIndex = (1 + urlIndex) % len(tabURLs)
		if err != nil {
			s.Fatal("Cannot add tab from list: ", err)
		}
		logAndResetStats(s, partialMeter, fmt.Sprintf("tab %d", len(rset.tabIDs)))
		if z, err := kernelmeter.ZramStats(ctx); err != nil {
			if !loggedMissingZramStats {
				s.Log("Cannot read zram stats")
				loggedMissingZramStats = true
			}
		} else {
			s.Logf("Metrics: tab %d: swap used %.3f MiB, effective compression %0.3f, utilization %0.3f",
				len(rset.tabIDs), float64(z.Original)/(1024*1024),
				float64(z.Used)/float64(z.Original),
				float64(z.Compressed)/float64(z.Used))
		}
		if p.RecordPageSet {
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
	validTabIDs, err = getValidTabIDs(ctx, cr)
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
	perfValues.Set(openedTabsMetric, float64(len(rset.tabIDs)))
	lostTabs := len(rset.tabIDs) + initialTabCount - len(validTabIDs)
	perfValues.Set(lostTabsMetric, float64(lostTabs))
	s.Log("Metrics: Phase 1: opened tab count ", len(rset.tabIDs))
	s.Log("Metrics: Phase 1: lost tab count ", lostTabs)

	times := allTabSwitchTimes
	logTabSwitchTimesToFile(ctx, times, s.OutDir(), "phase1")
	s.Logf("Metrics: Phase 1: mean tab switch time %7.2f ms", mean(times).Seconds()*1000)
	s.Logf("Metrics: Phase 1: stddev of tab switch times %7.2f ms", stdDev(times).Seconds()*1000)

	recordAndResetStats(s, fullMeter, perfValues, "phase_1")
	return initialTabSetIDs, rset
}

// runPhase2 runs the second phase of the test, measuring tab switch times to cold tabs.
func runPhase2(ctx context.Context, s *testing.State, cr *chrome.Chrome, rset *rendererSet, initialTabSetSize, coldTabSetSize int, fullMeter *kernelmeter.Meter) {
	perfValues := perf.NewValues()
	coldTabLower := initialTabSetSize
	coldTabUpper := coldTabLower + coldTabSetSize
	if coldTabUpper > len(rset.tabIDs) {
		coldTabUpper = len(rset.tabIDs)
	}
	coldTabIDs := rset.tabIDs[coldTabLower:coldTabUpper]
	times, err := cycleTabs(ctx, cr, coldTabIDs, rset, 0, false)
	if err != nil {
		s.Fatal("Cannot switch to cold tabs: ", err)
	}
	logTabSwitchTimes(ctx, times, len(coldTabIDs), s.OutDir(), "coldswitch")

	recordAndResetStats(s, fullMeter, perfValues, "coldswitch")

}

// runPhase3 runs the third phase of the test, quiesce.
func runPhase3(ctx context.Context, s *testing.State, cr *chrome.Chrome, rset *rendererSet, initialTabSetIDs []int, tabSwitchRepeatCount int, fullMeter *kernelmeter.Meter) {
	perfValues := perf.NewValues()
	// Wait a bit to help the system stabilize.
	if err := testing.Sleep(ctx, 10*time.Second); err != nil {
		s.Fatal("Timed out: ", err)
	}
	// Measure tab switching under pressure.
	if err := runTabSwitches(ctx, cr, rset, initialTabSetIDs, s.OutDir(), "heavy", tabSwitchRepeatCount); err != nil {
		s.Error("Cannot run tab switches with heavy load: ", err)
	}
	recordAndResetStats(s, fullMeter, perfValues, "phase_3")
}

// RunParameters contains the configurable parameters for Run.
type RunParameters struct {
	// MemoryUser indicates if the test is being run as part of a memoryuser test
	MemoryUser bool
	// ChromeInst is the Chrome instance to use for the test
	ChromeInst *chrome.Chrome
	// ChromePorts is the http and https ports to be used when startng WPR
	ChromePorts []int
	// DormantCodePath is the path name of a JS file with code that tests
	// for completion of a page load.
	DormantCodePath string
	// PageFilePath is the path name of a file with one page (4096 bytes)
	// of data.
	PageFilePath string
	// PageFileCompressionRatio is the approximate zram compression ratio of the content of PageFilePath.
	PageFileCompressionRatio float64
	// WPRArchivePath is the path name of a WPR archive.
	WPRArchivePath string
	// MaxTabCount is the maximal tab count to open
	MaxTabCount int
	// UseLogIn controls whether Run should use GAIA login.
	// (This is not yet functional.)
	UseLogIn bool
	// UseLiveSites controls whether Run should skip WPR and
	// load pages directly from the internet.
	UseLiveSites bool
	// RecordPageSet instructs Run to run in record mode
	// vs. replay mode.
	RecordPageSet bool
	// FakeLargeScreen instructs Chrome to use a large screen when no
	// displays are connected, which can happen, for instance, with
	// chromeboxes (otherwise Chrome will configure a default 1366x768
	// screen).
	FakeLargeScreen bool
	// UseARC instructs Chrome to be started with ARC enabled
	UseARC bool
}

// Run creates a memory pressure situation by loading multiple tabs into Chrome
// until the first tab discard occurs.  It takes various measurements as the
// pressure increases (phase 1) and afterwards (phase 2).
func Run(ctx context.Context, s *testing.State, p *RunParameters) {
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

	if p.RecordPageSet {
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

	cr, wpr, err := initBrowser(ctx, p)
	if err != nil {
		s.Fatal("Cannot start browser: ", err)
	}
	if !p.MemoryUser {
		defer cr.Close(ctx)
	}
	defer func() {
		if wpr == nil {
			return
		}
		defer wpr.Wait()
		// Send SIGINT to exit properly in recording mode.
		if err := wpr.Process.Signal(os.Interrupt); err != nil {
			s.Fatal("Cannot kill WPR")
		}
	}()

	// Log various system measurements, to help understand the memory
	// manager behavior.
	if err := kernelmeter.LogMemoryParameters(ctx, p.PageFileCompressionRatio); err != nil {
		s.Fatal("Cannot log memory parameters: ", err)
	}

	// Log in.  TODO(semenzato): this is not working (yet), we would like
	// to have it for gmail and similar.
	if p.UseLogIn {
		s.Log("Logging in")
		if err := googleLogIn(ctx, cr); err != nil {
			s.Fatal("Cannot login to google: ", err)
		}
	}

	if err := logScreenDimensions(ctx, cr); err != nil {
		s.Fatal("Cannot get screen dimensions: ", err)
	}

	initialTabSetIDs, rset := runPhase1(ctx, s, cr, p, initialTabSetSize, recentTabSetSize, tabSwitchRepeatCount, fullMeter)

	tIDs := rset.tabIDs[:]
	for _, id := range tIDs {
		r := rset.renderersByTabID[id]
		defer r.conn.Close()
	}

	// -----------------
	// Phase 2: measure tab switch times to cold tabs.
	// -----------------
	runPhase2(ctx, s, cr, rset, initialTabSetSize, coldTabSetSize, fullMeter)

	// -----------------
	// Phase 3: quiesce.
	// -----------------
	runPhase3(ctx, s, cr, rset, initialTabSetIDs, tabSwitchRepeatCount, fullMeter)

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
