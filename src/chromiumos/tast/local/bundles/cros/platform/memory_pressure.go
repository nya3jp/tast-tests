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
	"chromiumos/tast/local/testexec"
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

// Connects to Chrome and executes JS code in the context of a promise.
func evalPromiseBody(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	promiseBody string, out interface{}) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot create test API connection")
	}
	promise := fmt.Sprintf("new Promise((resolve, reject) => { %s });", promiseBody)
	if err := tconn.EvalPromise(ctx, promise, out); err != nil {
		return errors.Wrapf(err, "cannot execute promise \"%s\"", promise)
	}
	println(out)
	return nil
}

// Opens a new tab with a URL from tabURLs.  Returns the tabID of the tab.
func addTab(ctx context.Context, s *testing.State, cr *chrome.Chrome) int {
	URL := nextURL()
	body := fmt.Sprintf(`
chrome.tabs.create({"url": "%s"}, (tab) => {
	resolve(tab["id"]);
});
`, URL)
	var tabID int
	err := evalPromiseBody(ctx, s, cr, body, &tabID)
	if err != nil {
		s.Fatalf("Cannot open URL %v: %v", URL, err)
	}
	return tabID
}

// Activates the tab for tabID.
func activateTab(ctx context.Context, s *testing.State, cr *chrome.Chrome, tabID int) error {
	return evalPromiseBody(ctx, s, cr, fmt.Sprintf(`
chrome.tabs.update(%d, {"active": true}, (tab) => {
	resolve();
});
`, tabID), nil)
}

// Returns a list of non-discarded tab IDs.
func getValidTabIDs(ctx context.Context, s *testing.State, cr *chrome.Chrome) []int {
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

var EventCodes = []int{
	// 1  2  3  4  5  6  7  8  9
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 0
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 10
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 20
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 30
	0, 0, 0, 0, 0, 0, 52, 0, 0, 0, // 40
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 50
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 60
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 70
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 80
	0, 0, 0, 0, 0, 0, 0, 30, 48, 46, // 90
	32, 18, 33, 34, 35, 23, 36, 37, 38, 50, // 100
	49, 24, 25, 16, 19, 31, 20, 22, 47, 17, // 110
	45, 21, 44, 0, 0, 0, 0, 0, 0, 0, // 120
}

func emulateTyping(s *testing.State, text string) error {
	emuCmd := testexec.CommandContext(ctx, "evemu-play", "/dev/input/event3")
	emuStdin := emuCmd.StartWithStdin()
	timestamp := 0.0
	for i := 0; i < len(text); i++ {
		eventString := fmt.Sprintf(
			"E: %.6f 0004 0004 %d\nE: %.6f 0001 001f 1\nE: %.6f 0000 0000 0\n",
			timestamp, EventCodes[text[i]], timestamp, timestamp)
		emuStdin.WriteString(eventString)
		// fast typist
		timestamp += 0.1
	}
	emuStdin.Close()
}

func logIn(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	login_url := "https://accounts.google.com/ServiceLogin?continue=https%3A%2F%2Faccounts.google.com%2FManageAccount"
	login_tab := addTab(login_url)
	email_selector := "input[type=email]:not([aria-hidden=true]),#Email:not(.hidden)"
	query_code := fmt.Sprintf("document.querySelector(\"%s\") !== null", email_selector)
	promise_body := fmt.Sprintf(`chrome.tabs.executeScript(%d, {code: "%s"}, resolve() => {})`,
		login_tab, query_code)
	err = testing.Poll(ctx, s, cr, func(ctx context.Context) error {
		var page_ready bool
		err := evalPromiseBody(promise_body, &page_ready)
		if err != nil {
			return err
		}
		if page_ready {
			return nil
		} else {
			return errors.New("login page not ready")
		}
	}, &testing.PollOptions{
		Timeout:  5 * time.Second,
		Interval: 100 * time.Millisecond,
	})

	// Get focus on email field.
	focus_code = fmt.Sprintf("document.querySelector(\"%s\").focus()", email_selector)
	promise_body = fmt.Sprintf(`chrome.tabs.executeScript(%d, {code: "%s"}, resolve() => {})`,
		login_tab, focus_code)
	err := evalPromiseBody(promise_body)
	if err != nil {
		s.Fatal("Cannot focus on email entry field: ", err)
	}

	// Enter email.
	err := emulateTyping("wpr.memory.pressure.test@gmail.com")
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
		s.Fatal("Could not start Chrome: ", err)
	}

	// Log in.
	logIn(ctx, s, cr)

	// Figure out how many tabs already exist (typically 1).
	initialTabCount := len(getValidTabIDs(ctx, s, cr))
	var openedTabs []int
	// Open enough tabs for a "working set", i.e. the number of tabs that an
	// imaginary user will cycle through in their imaginary workflow.
	for i := 0; i < tabWorkingSetSize; i++ {
		openedTabs = append(openedTabs, addTab(ctx, s, cr))
		time.Sleep(newTabMilliseconds * time.Millisecond)
	}
	// Allocate memory by opening more tabs and cycling through recently
	// opened tabs until a tab discard occurs.
	for {
		validTabs := getValidTabIDs(ctx, s, cr)
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
		openedTabs = append(openedTabs, addTab(ctx, s, cr))
		time.Sleep(newTabMilliseconds * time.Millisecond)
	}
}
