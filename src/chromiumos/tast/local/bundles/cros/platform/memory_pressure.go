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
	"chromiumos/tast/testing"
)

const ()

func init() {
	testing.AddTest(&testing.Test{
		Func:    MemoryPressure,
		Desc:    "Measure stuff under memory pressure",
		Attr:    []string{"informational"},
		Timeout: 600 * time.Second,
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
	"https://drive.google.com",
	"https://photos.google.com",
	"https://news.google.com",
	"https://plus.google.com",
	"https://maps.google.com",
	"https://play.google.com/store",
	"https://play.google.com/music",
	"https://youtube.com",
	"https://www.nytimes.com",
	"https://www.whitehouse.gov",
	"https://www.wsj.com",
	"http://www.newsweek.com", // seriously, http?
	"https://www.washingtonpost.com",
	"https://www.foxnews.com",
	"https://www.nbc.com",
	"https://www.amazon.com",
	"https://www.walmart.com",
	"https://www.target.com",
	"https://www.facebook.com",
	"https://www.cnn.com",
	"https://www.cnn.com/us",
	"https://www.cnn.com/world",
	"https://www.cnn.com/politics",
	"https://www.cnn.com/money",
	"https://www.cnn.com/opinion",
	"https://www.cnn.com/health",
	"https://www.cnn.com/entertainment",
	"https://www.cnn.com/tech",
	"https://www.cnn.com/style",
	"https://www.cnn.com/travel",
	"https://www.cnn.com/sports",
	"https://www.cnn.com/video",
}

var nextURLIndex = 0

func nextURL() string {
	url := googleURLs[nextURLIndex]
	nextURLIndex++
	if nextURLIndex >= len(googleURLs) {
		nextURLIndex = 0
	}
	return url
}

// Connects to Chrome and executes JS promise.  Returns in |out| a value whose
// type must match the type of the object returned by the "resolve" call in the
// promise.
func evalPromiseBody(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	promiseBody string, out interface{}) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create test API connection")
	}
	promise := fmt.Sprintf("new Promise((resolve, reject) => { %s });", promiseBody)
	s.Log("PROMISE: ", promiseBody)
	if err := tconn.EvalPromise(ctx, promise, out); err != nil {
		return errors.Wrapf(err, "cannot execute promise \"%s\"", promise)
	}
	return nil
}

// Similar to the above function: Connects to Chrome and executes a JS promise
// which does not return a value.
func evalVoidPromiseBody(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	promiseBody string) error {
	return evalPromiseBody(ctx, s, cr, promiseBody, nil)
}

// Wraps code into a promise body which returns nothing.
func wrapAsPromiseBody(code string) string {
	return fmt.Sprintf("%s, () => { resolve() }", code)
}

// Prepares code for injection into a tab.
func wrapAsPromiseForTab(code string, tab int) string {
	return fmt.Sprintf(`chrome.tabs.executeScript(%d, {code: "%s"}, () => { resolve() })`,
		tab, code)
}

func addTabFromList(ctx context.Context, s *testing.State, cr *chrome.Chrome) (int, error) {
	return addTab(ctx, s, cr, nextURL())
}

// Opens a new tab with a URL from tabURLs.  Returns the ID of the tab.
func addTab(ctx context.Context, s *testing.State, cr *chrome.Chrome, url string) (int, error) {
	body := fmt.Sprintf(`
chrome.tabs.create({"url": "%s"}, (tab) => {
	resolve(tab["id"]);
});
`, url)
	var tabId int
	err := evalPromiseBody(ctx, s, cr, body, &tabId)
	if err != nil {
		return 0, errors.Wrapf(err, "cannot open URL %v", url)
	}
	return tabId, nil
}

// Activates the tab for tabId.
func activateTab(ctx context.Context, s *testing.State, cr *chrome.Chrome, tabId int) error {
	code := wrapAsPromiseBody(fmt.Sprintf(`chrome.tabs.update(%d, {"active": true})`, tabId))
	return evalVoidPromiseBody(ctx, s, cr, code)
}

// Returns a list of non-discarded tab IDs.
func getValidTabIds(ctx context.Context, s *testing.State, cr *chrome.Chrome) []int {
	var out []int
	err := evalPromiseBody(ctx, s, cr, `
chrome.tabs.query({"discarded": false}, function(tab_list) {
	resolve(tab_list.map((tab) => { return tab["id"]; }))
});
`, &out)
	if err != nil {
		s.Fatal("Cannot query tab list: ", err)
	}
	return out
}

func keyCode(c byte) int {
	// Pardon the double zeros, but the formatter is pig-headed.
	var eventCodes = []int{
		//  1   2   3   4   5   6   7   8   9
		00, 00, 00, 00, 00, 00, 00, 00, 00, 00, // 0
		28, 00, 00, 00, 00, 00, 00, 00, 00, 00, // 10
		00, 00, 00, 00, 00, 00, 00, 00, 00, 00, // 20
		00, 00, 00, 00, 00, 00, 00, 00, 00, 00, // 30
		00, 00, 00, 00, 00, 00, 52, 00, 00, 00, // 40
		00, 00, 00, 00, 00, 00, 00, 00, 00, 00, // 50
		00, 00, 00, 00, 00, 00, 00, 00, 00, 00, // 60
		00, 00, 00, 00, 00, 00, 00, 00, 00, 00, // 70
		00, 00, 00, 00, 00, 00, 00, 00, 00, 00, // 80
		00, 00, 00, 00, 00, 00, 00, 30, 48, 46, // 90
		32, 18, 33, 34, 35, 23, 36, 37, 38, 50, // 100
		49, 24, 25, 16, 19, 31, 20, 22, 47, 17, // 110
		45, 21, 44, 00, 00, 00, 00, 00, 00, 00, // 120
	}
	return eventCodes[c]
}

func emulateTyping(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	tabId int, text string) error {
	for i := 0; i < len(text); i++ {
		c := text[i]
		kc := keyCode(c)
		code := fmt.Sprintf(`
{
    var kbEvent = document.createEvent('KeyboardEvent');
    var initKind = typeof kbEvent.initKeyboardEvent !== 'undefined' ?
			'initKeyboardEvent' : 'initKeyEvent';
    kbEvent[initKind]('keydown',
                      true,
                      true,
                      window,
                      false,
                      false,
                      false,
                      false,
                      %d,
                      0);
    document.body.dispatchEvent(keyboardEvent);
}`, kc)
		promiseBody := wrapAsPromiseForTab(code, tabId)
		err := evalVoidPromiseBody(ctx, s, cr, promiseBody)
		if err != nil {
			return errors.Wrapf(err, "cannot send key %d (keycode %d)", keyCode)
		}
	}
	return nil
}

func googleLogIn(ctx context.Context, s *testing.State, cr *chrome.Chrome) error {
	loginUrl := "https://accounts.google.com/ServiceLogin?continue=https%3A%2F%2Faccounts.google.com%2FManageAccount"
	loginTab, err := addTab(ctx, s, cr, loginUrl)
	if err != nil {
		return errors.Wrapf(err, "Cannot add login tab", err)
	}
	emailSelector := "input[type=email]:not([aria-hidden=true]),#Email:not(.hidden)"
	queryCode := fmt.Sprintf("document.querySelector('%s') !== null", emailSelector)
	promiseBody := wrapAsPromiseForTab(queryCode, loginTab)

	// Wait for login page.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		var pageReady bool
		err := evalPromiseBody(ctx, s, cr, promiseBody, &pageReady)
		if err != nil {
			return errors.Wrap(err, "cannot determine login page status")
		}
		if pageReady {
			return nil
		} else {
			return errors.New("login page not ready")
		}
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 100 * time.Millisecond,
	})

	// Get focus on email field.
	focusCode := fmt.Sprintf("document.querySelector('%s').focus()", emailSelector)
	promiseBody = wrapAsPromiseForTab(focusCode, loginTab)
	err = evalVoidPromiseBody(ctx, s, cr, promiseBody)
	if err != nil {
		return errors.Wrap(err, "cannot focus on email entry field")
	}

	// Enter email.
	err = emulateTyping(ctx, s, cr, loginTab, "wpr.memory.pressure.test@gmail.com\n")
	if err != nil {
		return errors.Wrap(err, "cannot enter login name")
	}
	// Enter password.
	err = emulateTyping(ctx, s, cr, loginTab, "google.memory.chrome\n")
	if err != nil {
		return errors.Wrap(err, "cannot enter password")
	}
	return nil
}

func MemoryPressure(ctx context.Context, s *testing.State) {
	const (
		tabWorkingSetSize         = 5
		newTabMilliseconds        = 3000
		tabCycleDelayMilliseconds = 300
	)
	// Restart chrome.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Cannot start Chrome: ", err)
	}

	// Log in.
	err = googleLogIn(ctx, s, cr)
	if err != nil {
		s.Fatal("Cannot login to google: ", err)
	}

	// Figure out how many tabs already exist (typically 1).
	initialTabCount := len(getValidTabIds(ctx, s, cr))
	var openedTabs []int
	// Open enough tabs for a "working set", i.e. the number of tabs that an
	// imaginary user will cycle through in their imaginary workflow.
	for i := 0; i < tabWorkingSetSize; i++ {
		tab, err := addTabFromList(ctx, s, cr)
		if err != nil {
			s.Fatal("Cannot add initial tab from list")
		}
		openedTabs = append(openedTabs, tab)
		time.Sleep(newTabMilliseconds * time.Millisecond)
	}
	// Allocate memory by opening more tabs and cycling through recently
	// opened tabs until a tab discard occurs.
	for {
		validTabs := getValidTabIds(ctx, s, cr)
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
		tab, err := addTabFromList(ctx, s, cr)
		if err != nil {
			s.Fatal("Cannot add tab from list")
		}
		openedTabs = append(openedTabs, tab)
		time.Sleep(newTabMilliseconds * time.Millisecond)
	}
}
