// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tabswitchcuj contains the test code for TabSwitchCUJ. The test is
// extracted into this package to be shared between TabSwitchCUJRecorder and
// TabSwitchCUJ.
package tabswitchcuj

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// Level indicate how intensive of this test case is going to execute.
type Level uint8

// Level indicate how intensive of this test case is going to execute.
//
// Basic is the level to use to run this case in basic level
// Plus is the level to use to run this case in plus level
// Premium is the level to use to run this case in basic level
// Record is the level to use to run this case in *record mode*
const (
	Basic Level = iota
	Plus
	Premium
	Record
)

// webType define all web site is involved in this test case
type webType string

const (
	wikipedia  webType = "Wikipedia"
	reddit     webType = "Reddit"
	medium     webType = "Medium"
	googleNews webType = "GoogleNews"
	cnn        webType = "CNN"
	espn       webType = "ESPN"
	hulu       webType = "Hulu"
	pinterest  webType = "Pinterest"
	youtube    webType = "Youtube"
	netflix    webType = "Netflix"
)

type urlLink struct {
	level   Level   // the corredponding level of this link
	webName webType // current page's web site name
	url     string  // the url of current page
	// contentPatterns holds the patterns of web content elements to search to click on. During
	// tab switch, we find the pattern in the current page and click it. Links can be clicked
	// back and forth in case multiple rounds of tab switch are executed.
	contentPatterns []string
	currentPattern  int // the index of current page's corresponding content pattern
}

func newURLLink(level Level, web webType, url string, patterns []string) urlLink {
	if len(patterns) < 2 {
		panic("Invalid configuration of urlLink")
	}

	return urlLink{
		level:           level,
		webName:         web,
		url:             url,
		contentPatterns: patterns,
		currentPattern:  0,
	}
}

type chromeTab struct {
	conn       *chrome.Conn
	link       urlLink
	currentURL string
}

func (tab *chromeTab) searchElementWithPatternAndClick(ctx context.Context, pattern string) error {
	if err := tab.conn.Eval(ctx, "window.location.href", &tab.currentURL); err != nil {
		return errors.Wrap(err, "failed to get URL before click on a element")
	}
	testing.ContextLogf(ctx, "Current URL: %q", tab.currentURL)

	// Find the desired link and trigger click.
	// The link might have parameters or tokens. Find the element by matching with pattern (CSS selector, not
	// regular expression).
	// Click on a link will trigger redirect immediately. If the page is redirect first before the CDP returns,
	// the JSObject won't be able to release, and an error will be returned. Therefore timeout is used to
	// allow CDP return and object release before clicking the link.

	script := `(pattern) => {
			var name = "a[href*='" + pattern + "']";
			var els = document.querySelectorAll(name);
			if (els.length == 0)
				return false;
			var ele = els[0];
			setTimeout(() => { ele.click(); }, 500);
			return true;
		}`

	var done bool
	if err := tab.conn.Call(ctx, &done, script, pattern); err != nil || !done {
		return errors.Wrap(err, "failed to search and click on a element")
	}

	// Redirecting does not happen instantly. Use poll to detect whether it has been redirected.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var urlAfter string
		if err := tab.conn.Eval(ctx, "window.location.href", &urlAfter); err != nil {
			return errors.Wrap(err, "failed to get URL after click on a element")
		}
		if urlAfter == tab.currentURL {
			return errors.New("page is not redirect")
		}
		tab.currentURL = urlAfter
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
		return errors.Wrapf(err, "failed to click HTML element with pattern [%v]", pattern)
	}

	testing.ContextLogf(ctx, "HTML element clicked [%s], page is redirect to: %q", pattern, tab.currentURL)
	tab.currentURL = strings.TrimSuffix(tab.currentURL, "/")

	return nil
}

func (tab *chromeTab) clickAnchor(ctx context.Context, timeout time.Duration) error {
	p := tab.link.currentPattern
	pn := p + 1
	if pn == len(tab.link.contentPatterns) {
		pn = 0
	}

	pattern := tab.link.contentPatterns[pn]

	testing.ContextLogf(ctx, "Click link and navigate from %q to %q", tab.link.contentPatterns[p], pattern)

	// The web page might be still loading at this point, which could fail to find the anchor.
	// Use poll here to ensure the anchor is found and then click it.
	ts := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if e := tab.searchElementWithPatternAndClick(ctx, pattern); e != nil {
			testing.ContextLogf(ctx, "Click anchor failed, retry, elapsed: %s, error: %v", time.Since(ts), e)
			return errors.Wrap(e, "failed to click anchor")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to click anchor, current page: %s", tab.link.url)
	}

	tab.link.currentPattern = pn

	return nil
}

func (tab *chromeTab) close(ctx context.Context, s *testing.State) {
	if tab.conn == nil {
		return
	}

	if err := tab.conn.CloseTarget(ctx); err != nil {
		s.Error("Failed to close target, error: ", err)
	}
	if err := tab.conn.Close(); err != nil {
		s.Error("Failed to close the connection, error: ", err)
	}
}

type chromeWindow struct {
	tabs []*chromeTab
}

// getTargets sets all web targets according to the input Level.
func getTargets(caseLevel Level) []*chromeWindow {
	allLinks := []urlLink{

		newURLLink(Basic, wikipedia, "https://en.wikipedia.org/wiki/Main_Page", []string{`/Main_Page`, `/Wikipedia:Contents`}),
		newURLLink(Basic, wikipedia, "https://en.wikipedia.org/wiki/Portal:Current_events", []string{`/Portal:Current_events`, `/Special:Random`}),
		newURLLink(Basic, wikipedia, "https://en.wikipedia.org/wiki/Wikipedia:About", []string{`/Wikipedia:About`, `/Wikipedia:Contact_us`}),
		newURLLink(Plus, wikipedia, "https://en.wikipedia.org/wiki/Help:Contents", []string{`/Help:Contents`, `/Help:Introduction`}),
		newURLLink(Plus, wikipedia, "https://en.wikipedia.org/wiki/Wikipedia:Community_portal", []string{`/Wikipedia:Community_portal`, `/Special:RecentChanges`}),
		newURLLink(Premium, wikipedia, "https://en.wikipedia.org/wiki/COVID-19_pandemic", []string{`/COVID-19_pandemic`, `/Coronavirus_disease_2019`}),

		newURLLink(Basic, reddit, "https://www.reddit.com/r/wallstreetbets", []string{`/r/wallstreetbets/hot/`, `/r/wallstreetbets/new/`}),
		newURLLink(Basic, reddit, "https://www.reddit.com/r/technews", []string{`/r/technews/hot/`, `/r/technews/new/`}),
		newURLLink(Basic, reddit, "https://www.reddit.com/r/olympics", []string{`/r/olympics/hot/`, `/r/olympics/new/`}),
		newURLLink(Plus, reddit, "https://www.reddit.com/r/programming", []string{`/r/programming/hot/`, `/r/programming/new/`}),
		newURLLink(Plus, reddit, "https://www.reddit.com/r/apple", []string{`/r/apple/hot/`, `/r/apple/new/`}),
		newURLLink(Premium, reddit, "https://www.reddit.com/r/brooklynninenine", []string{`/r/brooklynninenine/hot/`, `/r/brooklynninenine/new/`}),

		newURLLink(Basic, medium, "https://medium.com/topic/business", []string{`/topic/business`, `/topic/money`}),
		newURLLink(Basic, medium, "https://medium.com/topic/startups", []string{`/topic/startups`, `/topic/leadership`}),
		newURLLink(Plus, medium, "https://medium.com/topic/work", []string{`/topic/work`, `/topic/freelancing`}),
		newURLLink(Premium, medium, "https://medium.com/topic/software-engineering", []string{`/software-engineering`, `/topic/programming`}),
		newURLLink(Premium, medium, "https://medium.com/topic/artificial-intelligence", []string{`/artificial-intelligence`, `/topic/technology`}),

		newURLLink(Basic, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGRqTVhZU0FtVnVHZ0pWVXlnQVAB", // topics: Technology
			[]string{`./topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGRqTVhZU0FtVnVHZ0pWVXlnQVAB`,
				`./topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGx1YlY4U0FtVnVHZ0pWVXlnQVAB`}), // topics: World
		newURLLink(Basic, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNREpxYW5RU0FtVnVHZ0pWVXlnQVAB", // topics: Entertainment
			[]string{`./topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNREpxYW5RU0FtVnVHZ0pWVXlnQVAB`,
				`./topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGx6TVdZU0FtVnVHZ0pWVXlnQVAB`}), // topics: Business
		newURLLink(Plus, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp1ZEdvU0FtVnVHZ0pWVXlnQVAB", // topics: Sports
			[]string{`./topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp1ZEdvU0FtVnVHZ0pWVXlnQVAB`,
				`./topics/CAAqIggKIhxDQkFTRHdvSkwyMHZNREZqY0hsNUVnSmxiaWdBUAE`}), // topics: Covid-19
		newURLLink(Premium, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp0Y1RjU0FtVnVHZ0pWVXlnQVAB", // topics: Science
			[]string{`./topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp0Y1RjU0FtVnVHZ0pWVXlnQVAB`,
				`./topics/CAAqIQgKIhtDQkFTRGdvSUwyMHZNR3QwTlRFU0FtVnVLQUFQAQ`}), // topics: Health
		newURLLink(Premium, googleNews, "https://news.google.com/topstories", // top stories
			[]string{`/topstories`,
				`./topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFZxYUdjU0FtVnVHZ0pWVXlnQVAB`}), // Headlines

		newURLLink(Plus, cnn, "https://edition.cnn.com/world", []string{`/world`, `/africa`}),
		newURLLink(Plus, cnn, "https://edition.cnn.com/americas", []string{`/americas`, `/asia`}),
		newURLLink(Plus, cnn, "https://edition.cnn.com/australia", []string{`/australia`, `/china`}),
		newURLLink(Premium, cnn, "https://edition.cnn.com/europe", []string{`/europe`, `/india`}),
		newURLLink(Premium, cnn, "https://edition.cnn.com/middle-east", []string{`/middle-east`, `/uk`}),

		newURLLink(Plus, espn, "https://www.espn.com/nfl/", []string{`/nfl/scoreboard`, `/nfl/schedule`}),
		newURLLink(Plus, espn, "https://www.espn.com/nba/", []string{`/nba/scoreboard`, `/nba/schedule`}),
		newURLLink(Plus, espn, "https://www.espn.com/mens-college-basketball/", []string{`/mens-college-basketball/scoreboard`, `/mens-college-basketball/schedule`}),
		newURLLink(Premium, espn, "https://www.espn.com/tennis/", []string{`/tennis/dailyResults`, `/tennis/schedule`}),
		newURLLink(Premium, espn, "https://www.espn.com/soccer/", []string{`/soccer/scoreboard`, `/soccer/schedule`}),

		newURLLink(Plus, hulu, "https://www.hulu.com/hub/movies", []string{`/hub/movies`, `/hub/originals`}),

		newURLLink(Plus, pinterest, "https://www.pinterest.com/ideas/", []string{`/ideas/`, `/ideas/holidays/910319220330/`}),

		newURLLink(Premium, youtube, "https://www.youtube.com", []string{`/`, `/feed/trending`}),

		newURLLink(Premium, netflix, "https://www.netflix.com", []string{`netflix.com`, `help.netflix.com/legal/termsofuse`}),
	}

	winNum := 1
	tabNum := 0
	idx := 0

	switch caseLevel {
	case Basic:
		winNum = 2
		tabNum = 5
	case Plus:
		winNum = 4
		tabNum = 6
	case Premium, Record:
		winNum = 4
		tabNum = 9
	}

	windows := make([]*chromeWindow, winNum)
	for i, window := range windows {
		window = &chromeWindow{}
		window.tabs = make([]*chromeTab, tabNum)
		for j, tab := range window.tabs {
			tab = &chromeTab{}
			for idx < len(allLinks) {
				if allLinks[idx].level <= caseLevel {
					tab.conn = nil
					tab.link = allLinks[idx]
					idx++
					break
				}
				idx++
			}
			window.tabs[j] = tab
		}
		windows[i] = window
	}

	return windows
}

// Run2 runs the TabSwitchCUJ test. It is invoked by TabSwitchCujRecorder2 to
// record web contents via WPR and invoked by TabSwitchCUJ2 to execute the tests
// from the recorded contents. Additional actions will be executed in each tab.
func Run2(ctx context.Context, s *testing.State, cr *chrome.Chrome, caseLevel Level, isTablet bool) {
	var (
		tabSwitchTimeout   = 2 * time.Minute
		clickLinkTimeout   = 1 * time.Minute
		pageLoadingTimeout = 2 * time.Minute
	)
	// In record mode, give more time to ensure web content is fully recorded.
	if caseLevel == Record {
		pageLoadingTimeout = 5 * time.Minute
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API, error: ", err)
	}

	if _, ok := s.Var("mute"); ok {
		if err := crastestclient.Mute(ctx); err != nil {
			s.Fatal("Failed to mute: ", err)
		}
		defer crastestclient.Unmute(ctx)
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a recorder, error: ", err)
	}
	defer recorder.Close(ctx)

	cleanup, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		s.Fatal("Failed to set battery discharge, error: ", err)
	}
	defer cleanup(ctx)

	windows := getTargets(caseLevel)

	var tsAction uiActionHandler

	if isTablet {
		tc, err := pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create the touch controller: ", err)
		}
		defer tc.Close()

		tsAction = newTabletActionHandler(tconn, tc)
	} else {
		pad, err := input.Trackpad(ctx)
		if err != nil {
			s.Fatal("Failed to create trackpad event writer: ", err)
		}
		defer pad.Close()

		touchPad, err := pad.NewMultiTouchWriter(2)
		if err != nil {
			s.Fatal("Failed to create trackpad singletouch writer: ", err)
		}
		defer touchPad.Close()

		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to open the keyboard, error: ", err)
		}
		defer kb.Close()

		tsAction = newClamshellActionHandler(tconn, kb, pad, touchPad)
	}

	defer func() {
		// To make debug easier, if something goes wrong, take screen shot before tabs are closed.
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		// Close all opened tabs before finishing the test.
		for _, window := range windows {
			for _, tab := range window.tabs {
				tab.close(ctx, s)
			}
		}
	}()

	scrollActions := tsAction.scrollPage()

	var (
		timeElapsedBrowserLaunch time.Duration
		timeElapsedTabsOpened    time.Duration
		timeBrowserLaunchStart   time.Time
		timeTabsOpenStart        = time.Now()
	)

	// Launch browser and track the elapsed time.
	if timeBrowserLaunchStart, err = tsAction.launchChrome(ctx); err != nil {
		s.Fatal("Failed to launch Chrome: ", err)
	}
	timeElapsedBrowserLaunch = time.Since(timeBrowserLaunchStart)
	s.Log("Browser start ms: ", timeElapsedBrowserLaunch)

	// Open all windows and tabs.
	for idxWindow, window := range windows {
		for idxTab, tab := range window.tabs {
			s.Logf("Opening window %d, tab %d", idxWindow+1, idxTab+1)

			if idxWindow == 0 && idxTab == 0 {
				if tab.conn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/")); err != nil {
					// If failed to match the very first tab here, no way to close the tab either.
					s.Fatal("Failed to find new tab: ", err)
				}
				if err = tab.conn.Navigate(ctx, tab.link.url); err != nil {
					s.Fatalf("Failed to navigate to %s, error: %+v", tab.link.url, err)
				}
			} else {
				if tab.conn, err = tsAction.newTab(ctx, cr, tab.link.url, idxTab == 0); err != nil {
					s.Fatal("Failed to create new Chrome tab: ", err)
				}
			}

			if err := webutil.WaitForRender(ctx, tab.conn, pageLoadingTimeout); err != nil {
				s.Fatal("Failed to wait for finish render: ", err)
			}

			// In replay mode, user won't be able to know whether the page is quiescence or not,
			// and it is not necessary to wait for quiescence in replay mode.
			// In record mode, needs to wait for quiescence to properly record web content.
			if caseLevel == Record {
				if err := webutil.WaitForQuiescence(ctx, tab.conn, pageLoadingTimeout); err != nil {
					s.Fatal("Failed to wait for tab quiescence: ", err)
				}
			}

			tab.currentURL = strings.TrimSuffix(tab.currentURL, "/")
		}
	}

	// Total time used from beginning to load all pages.
	timeElapsedTabsOpened = time.Since(timeTabsOpenStart)
	s.Log("All tabs opened Elapsed: ", timeElapsedTabsOpened)

	// Shorten context a bit to allow for cleanup if Run fails.
	shorterCtx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()
	// Switch between windows and tabs.
	if err = recorder.Run(shorterCtx, func(ctx context.Context) error {
		for idx, window := range windows {
			s.Logf("Switching to window #%d", idx+1)
			if err := tsAction.switchWindow(ctx, idx, len(windows)); err != nil {
				s.Fatal("Failed to switch window: ", err)
			}

			tabTotalNum := len(window.tabs)
			tabIdxPre := tabTotalNum - 1 // Last tab is still active.
			for i := 0; i < tabTotalNum; i++ {
				tabIdx := i % tabTotalNum
				s.Logf("Switching tab to window %d, tab %d", idx+1, tabIdx+1)

				if err := tsAction.switchTab(ctx, tabIdxPre, tabIdx, tabTotalNum); err != nil {
					s.Fatal("Failed to switch tab: ", err)
				}
				tabIdxPre = tabIdx
				tab := window.tabs[tabIdx]

				timeStart := time.Now()
				if err = webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
					s.Fatal("Failed to wait for the tab to be visible: ", err)
				}
				renderTime := time.Now().Sub(timeStart)
				// Debugging purpose message, to observe which tab takes unusual long time to render.
				s.Log("Tab rendering time after switching: ", renderTime)
				if caseLevel == Record {
					if err := webutil.WaitForQuiescence(ctx, tab.conn, pageLoadingTimeout); err != nil {
						s.Fatal("Failed to wait for tab quiescence: ", err)
					}
					quiescenceTime := time.Now().Sub(timeStart)
					// Debugging purpose message, to observe which tab takes unusual long time to quiescence
					s.Log("Tab quiescence time after switching: ", quiescenceTime)
				}

				// To reduce total execution time of this test case,
				// these specific websites has been chosen to do scroll actions as per requirement.
				if tab.link.webName == wikipedia || tab.link.webName == hulu || tab.link.webName == youtube {
					for _, act := range scrollActions {
						if err = act(ctx, tab.conn); err != nil {
							s.Fatal("Failed to execute action: ", err)
						}
						// Make sure the whole web content is recorded only under Recording.
						if caseLevel == Record {
							if err := webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
								s.Fatal("Failed to wait for finish render after link clicking: ", err)
							}
							if err := webutil.WaitForQuiescence(ctx, tab.conn, pageLoadingTimeout); err != nil {
								s.Fatal("Failed to wait for tab quiescence after link clicking: ", err)
							}
						}
					}
				}

				// Click on 1 link per 2 tabs, or click on 1 link for every tab under Record mode to ensure all links are
				// accessible under any other levels.
				if tabIdx%2 == 0 || caseLevel == Record {
					if err := tab.clickAnchor(ctx, pageLoadingTimeout); err != nil {
						s.Fatal("Failed to click anchor: ", err)
					}
					// Google news web site is a single page application. Need to refresh
					// the page to let the page reload.
					if tab.link.webName == googleNews {
						if err := tsAction.pageRefresh(ctx); err != nil {
							s.Error("Failed to refresh: ", err)
						}
					}
					if caseLevel == Record {
						// Ensure contents are renderred in recording mode.
						if err := webutil.WaitForRender(ctx, tab.conn, pageLoadingTimeout); err != nil {
							s.Fatal("Failed to wait for finish render: ", err)
						}
						if err := webutil.WaitForQuiescence(ctx, tab.conn, pageLoadingTimeout); err != nil {
							s.Fatal("Failed to wait for tab quiescence: ", err)
						}
					} else {
						// It is normal that tabs might remain loading, hence no handle error here.
						webutil.WaitForQuiescence(ctx, tab.conn, clickLinkTimeout)
					}
				}
			}
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", err)
	}

	pv := perf.NewValues()

	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(timeElapsedBrowserLaunch.Milliseconds()))

	pv.Set(perf.Metric{
		Name:      "TabSwitchCUJ.ElapsedTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(timeElapsedTabsOpened.Milliseconds()))

	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report, error: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values, error: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Error("Failed to save histogram raw data: ", err)
	}
}
