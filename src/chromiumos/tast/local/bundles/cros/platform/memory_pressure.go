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
		Timeout: 300 * time.Second,
	})
}

var tabURLs = [...]string{
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
	url := tabURLs[nextURLIndex]
	nextURLIndex++
	if nextURLIndex >= len(tabURLs) {
		nextURLIndex = 0
	}
	return url
}

// Connects to Chrome and executes JS code in the context of a promise.
func evalPromiseBody(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	promiseBody string, out interface{}) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Errorf("cannot create test API connection: %v", err)
	}
	promise := fmt.Sprintf("new Promise((resolve, reject) => { %s });", promiseBody)
	if err := tconn.EvalPromise(ctx, promise, out); err != nil {
		return errors.Errorf("cannot execute promise \"%s\": %v ", promise, err)
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

func MemoryPressure(ctx context.Context, s *testing.State) {
	const (
		tabWorkingSetSize         = 10
		newTabSeconds             = 3
		tabCycleDelayMilliseconds = 300
	)
	// Restart chrome.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Could not start Chrome: ", err)
	}
	// Figure out how many tabs already exist (typically 1).
	initialTabCount := len(getValidTabIDs(ctx, s, cr))
	var openedTabs []int
	// Open enough tabs for a "working set", i.e. the number of tabs that an
	// imaginary user will cycle through in their imaginary workflow.
	for i := 0; i < tabWorkingSetSize; i++ {
		openedTabs = append(openedTabs, addTab(ctx, s, cr))
		time.Sleep(newTabSeconds * time.Second)
	}
	// Allocate memory by opening more tabs and cycling through recently
	// opened tabs until a tab discard occurs.
allocationLoop:
	for {
		validTabs := getValidTabIDs(ctx, s, cr)
		s.Logf("Opened %v, present %v, initial %v", len(openedTabs),
			len(validTabs), initialTabCount)
		if len(openedTabs)+initialTabCount < len(validTabs) {
			s.Log("Ending allocation because one or more targets (tabs) have gone")
			break allocationLoop
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
		time.Sleep(newTabSeconds * time.Second)
	}
}
