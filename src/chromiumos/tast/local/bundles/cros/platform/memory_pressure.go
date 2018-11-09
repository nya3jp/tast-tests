// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"github.com/mafredri/cdp/devtool"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const ()

func init() {
	testing.AddTest(&testing.Test{
		Func: MemoryPressure,
		Desc: "Measure stuff under memory pressure",
		Attr: []string{"informational"},
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

// Opens a new tab with the given URL.  Returns the tab as a devtools target.
func addTab(ctx context.Context, s *testing.State, cr *chrome.Chrome) *devtool.Target {
	URL := nextURL()
	target, err := cr.DevTools().CreateURL(ctx, URL)
	if err != nil {
		s.Fatalf("Cannot open URL %v: %v", URL, err)
	}
	return target
}

// Activates the tab for target.
func activateTab(ctx context.Context, cr *chrome.Chrome, target *devtool.Target) error {
	return cr.DevTools().Activate(ctx, target)
}

// Returns the number of "devtools targets", which tracks the number of tabs.
func getTargets(ctx context.Context, s *testing.State, cr *chrome.Chrome) []*devtool.Target {
	targets, err := cr.DevTools().List(ctx)
	if err != nil {
		s.Fatal("Cannot list renderers: ", err)
	}
	return targets
}

func MemoryPressure(ctx context.Context, s *testing.State) {
	const (
		tabWorkingSetSize = 5
	)
	// Restart chrome.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Could not start Chrome: ", err)
	}
	// Figure out how many devtools targets initially exist.  Every new tab
	// adds one target, so we can track of the number of tabs.
	initialWindowCount := len(getTargets(ctx, s, cr))

	var openedTabs []*devtool.Target
	// Open an initial set of tabs.
	for i := 0; i < tabWorkingSetSize; i++ {
		openedTabs = append(openedTabs, addTab(ctx, s, cr))
	}
	// Allocate memory by opening more tabs and cycling through them until
	// a tab discard occurs.
allocationLoop:
	for {
		for i := 0; i < tabWorkingSetSize; i++ {
			recent := len(openedTabs) - tabWorkingSetSize + i
			err := activateTab(ctx, cr, openedTabs[recent])
			if err != nil {
				// Assume error is due to tab having been
				// discarded.
				break allocationLoop
			}
			time.Sleep(300 * time.Millisecond)
		}
		openedTabs = append(openedTabs, addTab(ctx, s, cr))
		presentTabs := getTargets(ctx, s, cr)
		if len(openedTabs)+initialWindowCount < len(presentTabs) {
			break allocationLoop
		}
	}
}
