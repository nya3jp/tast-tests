// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const ()

func init() {
	testing.AddTest(&testing.Test{
		Func:    MemoryPressure,
		Desc:    "Measure stuff under memory pressure",
		Attr:    []string{"informational"},
		Timeout: 30 * time.Minute,
		// Timeout: 1 * time.Hour, // temporary for wpr recording
		Data: []string{
			"wpr_archive_external.txt",
		},
	})
}

var googleURLs = []string{
	"https://mail.google.com/mail/#inbox",
	"https://plus.google.com/discover",
	"https://maps.google.com",
	"https://www.youtube.com",
	"https://play.google.com/store",
	"https://play.google.com/music/listen#/sulp",
	"https://drive.google.com/",
	"https://docs.google.com/document/d/1eaJ33otfGh1xK4pfEOk-FeT-Cu7lNGMpUKimqNGHzOs/edit#heading=h.ksjxwifgg3eq",
	"https://calendar.google.com/calendar/r?pli=1&t=AKUaPmaEIwpJ1_u67bJQOHXhwde1cBTp-75ZDm7SlvQcKjkj8ZYz2S3cm3Ssad851PpfumG9qC_RYJtQmGGEstIhZG0-So8ePA%3D%3D",
	"https://hangouts.google.com/",
}

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

// This test creates one renderer for each tab.
type renderer struct {
	conn  *chrome.Conn
	tabID int
}

var nextURLIndex = 0

func nextURL() string {
	url := tabURLs[nextURLIndex]
	nextURLIndex++
	if nextURLIndex >= len(tabURLs) {
		nextURLIndex = 0
	}
	return url
}

// Executes a JS promise on connection |conn|.  |promiseBody| is the code run as
// a promise, and it must contain a call to resolve().  Returns in |out| a
// value whose type must match the type of the object returned by the "resolve"
// call.
func evalPromiseBody(ctx context.Context, s *testing.State, conn *chrome.Conn,
	promiseBody string, out interface{}) error {
	promise := fmt.Sprintf("new Promise((resolve, reject) => { %s });", promiseBody)
	if err := conn.EvalPromise(ctx, promise, out); err != nil {
		return errors.Wrapf(err, "cannot execute promise (%s)", promise)
	}
	return nil
}

// Same as above, but no out parameter.
func execPromiseBody(ctx context.Context, s *testing.State, conn *chrome.Conn,
	promiseBody string) error {
	return evalPromiseBody(ctx, s, conn, promiseBody, nil)
}

// Same as above, but execute the promise in the browser.
func evalPromiseBodyInBrowser(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	promiseBody string, out interface{}) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create test API connection")
	}
	return evalPromiseBody(ctx, s, tconn, promiseBody, out)
}

// Similar to the above function: Connects to Chrome and executes a JS promise
// which does not return a value.
func execPromiseBodyInBrowser(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	promiseBody string) error {
	return evalPromiseBodyInBrowser(ctx, s, cr, promiseBody, nil)
}

// Gets the tab ID for the currently active tab.
func getActiveTabID(ctx context.Context, s *testing.State, cr *chrome.Chrome) (int, error) {
	var tabID int
	promiseBody := "chrome.tabs.query({'active': true}, (tlist) => { resolve(tlist[0]['id']) })"
	err := evalPromiseBodyInBrowser(ctx, s, cr, promiseBody, &tabID)
	if err != nil {
		return 0, errors.Wrap(err, "cannot get tabID")
	}
	return tabID, nil
}

// Creates a new renderer and the associated tab, which loads |url|.  Returns
// the renderer instance.
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

// Creates a new renderer/tab with the next URL from a URL list.
func addTabFromList(ctx context.Context, s *testing.State, cr *chrome.Chrome) (*renderer, error) {
	return addTab(ctx, s, cr, nextURL())
}

// Activates the tab for tabID.
func activateTab(ctx context.Context, s *testing.State, cr *chrome.Chrome, tabID int) error {
	code := fmt.Sprintf(`chrome.tabs.update(%d, {"active": true}, () => { resolve() })`, tabID)
	return execPromiseBodyInBrowser(ctx, s, cr, code)
}

// Returns a list of non-discarded tab IDs.
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

func waitForElement(ctx context.Context, s *testing.State, r *renderer, selector string) error {
	queryCode := fmt.Sprintf("resolve(document.querySelector(\"%s\") !== null)", selector)

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

func focusOnElement(ctx context.Context, s *testing.State, r *renderer, selector string) error {
	focusCode := fmt.Sprintf("{ document.querySelector('%s').focus(); resolve(); }", selector)
	if err := execPromiseBody(ctx, s, r.conn, focusCode); err != nil {
		return errors.Wrap(err, "cannot focus on element")
	}
	return nil
}

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
	time.Sleep(5 * time.Second)
	// Enter email.
	err = emulateTyping(ctx, s, cr, loginTab, "wpr.memory.pressure.test@gmail.com")
	if err != nil {
		return errors.Wrap(err, "cannot enter login name")
	}
	s.Log("email entered")
	time.Sleep(1 * time.Second)
	err = emulateTyping(ctx, s, cr, loginTab, "\n")
	if err != nil {
		return errors.Wrap(err, "cannot enter login name")
	}
	passwordSelector := "input[type=password]"
	// TODO: need to figure out why waitForElement below is not sufficient
	// to properly delay further input.
	time.Sleep(5 * time.Second)
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
	s.Log("password entered")
	// TODO: figure out if and why this wait is needed.
	time.Sleep(5 * time.Second)
	err = emulateTyping(ctx, s, cr, loginTab, "\n")
	// TODO: figure out if and why this wait is needed.
	time.Sleep(10 * time.Second)
	return nil
}

func MemoryPressure(ctx context.Context, s *testing.State) {
	const (
		logIn                     = false
		wprArchiveName            = "wpr_archive_external.txt"
		tabWorkingSetSize         = 5
		newTabMilliseconds        = 3000
		tabCycleDelayMilliseconds = 300
		// newTabMilliseconds        = 20000 // temporary for wpr recording
		// tabCycleDelayMilliseconds = 1000  // temporary for wpr recording
	)

	// Start the Web Page Recorder in replay mode.
	wprArchivePath := s.DataPath(wprArchiveName)
	cmd := testexec.CommandContext(ctx, "wpr", "replay",
		"--http_port=8080", "--https_port=8081", wprArchivePath)
	err := cmd.Start()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Could not start WPR: ", err)
	}

	// Restart chrome for use with WPR.
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs([]string{
			"--host-resolver-rules=\"MAP *:80 127.0.0.1:8080,MAP *:443 127.0.0.1:8081,EXCLUDE localhost\"",
			"--ignore-certificate-errors-spki-list=PhrPvGIaAMmd29hj8BCZOq096yj7uMpRNHpn5PDxI6I="}))
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}

	// Log in.  This isn't working (yet).
	if logIn {
		err := googleLogIn(ctx, s, cr)
		if err != nil {
			s.Fatal("Cannot login to google: ", err)
		}
	}

	// Figure out how many tabs already exist (typically 1).
	initialTabCount := len(getValidTabIDs(ctx, s, cr))
	var openedTabs []*renderer
	// Open enough tabs for a "working set", i.e. the number of tabs that an
	// imaginary user will cycle through in their imaginary workflow.
	for i := 0; i < tabWorkingSetSize; i++ {
		renderer, err := addTabFromList(ctx, s, cr)
		if err != nil {
			s.Fatal("Cannot add initial tab from list")
		}
		openedTabs = append(openedTabs, renderer)
		time.Sleep(newTabMilliseconds * time.Millisecond)
	}
	// Allocate memory by opening more tabs and cycling through recently
	// opened tabs until a tab discard occurs.
	var validTabs []int
	for {
		validTabs = getValidTabIDs(ctx, s, cr)
		s.Logf("Opened %v, present %v, initial %v",
			len(openedTabs), len(validTabs), initialTabCount)
		if len(openedTabs)+initialTabCount > len(validTabs) {
			s.Log("Ending allocation because one or more targets (tabs) have gone")
			break
		}
		for i := 0; i < tabWorkingSetSize; i++ {
			recent := i + len(validTabs) - tabWorkingSetSize
			err := activateTab(ctx, s, cr, validTabs[recent])
			if err != nil {
				// If the error is due to the tab having been
				// discarded (although it is not expected that
				// a discarded tab would cause an error here),
				// we'll catch the discard next time around the
				// loop.  In any case, ignore the error (other
				// than logging it).
				s.Logf("cannot activate tab: %v", err)
			}
			time.Sleep(tabCycleDelayMilliseconds * time.Millisecond)
		}
		renderer, err := addTabFromList(ctx, s, cr)
		if err != nil {
			s.Fatal("Cannot add tab from list")
		}
		openedTabs = append(openedTabs, renderer)
		time.Sleep(newTabMilliseconds * time.Millisecond)
	}

	// Output metrics.
	p := &perf.Values{}
	openedTabsMetric := perf.Metric{
		Name:      "tast_opened_tab_count",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}
	p.Set(openedTabsMetric, float64(len(openedTabs)))
	s.Logf("metrics: opened tab count %v", len(openedTabs))
	s.Logf("metrics: discarded tab count %v",
		len(openedTabs)+initialTabCount-len(validTabs))
	if err = p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
