// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tabswitchcuj contains the test code for TabSwitchCUJ. The test is
// extracted into this package to be shared between TabSwitchCUJRecorder and
// TabSwitchCUJ.
package tabswitchcuj

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

const (
	tabSwitchTimeout = 2 * time.Minute
	clickLinkTimeout = 1 * time.Minute

	replayPageLoadingTimeout    = 2 * time.Minute
	recordingPageLoadingTimeout = 5 * time.Minute
)

// pageLoadingTimeout returns the timeout value when waiting for a page being loaded.
func pageLoadingTimeout(caseLevel Level) time.Duration {
	// In record mode, give more time to loading to ensure web content is fully recorded.
	if caseLevel == Record {
		return recordingPageLoadingTimeout
	}
	return replayPageLoadingTimeout
}

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
	wikipedia webType = "Wikipedia"
	reddit    webType = "Reddit"
	medium    webType = "Medium"
	yahooNews webType = "YahooNews"
	cnn       webType = "CNN"
	espn      webType = "ESPN"
	hulu      webType = "Hulu"
	pinterest webType = "Pinterest"
	youtube   webType = "Youtube"
	netflix   webType = "Netflix"
)

// webPageInfo records a Chrome page's information, including the current browsing page
// and url links (in patterns) for page navigation.
type webPageInfo struct {
	level   Level   // the test level of this link will be used for. Only used for generating targets
	webName webType // current page's website name
	// contentPatterns holds the patterns of the url links embedded in the web page. During
	// tab switch, we find the url of the given pattern in the current page and click it.
	// Links can be clicked back and forth in case multiple rounds of tab switch are executed.
	contentPatterns []string
}

func newPageInfo(level Level, web webType, patterns ...string) *webPageInfo {
	if len(patterns) < 2 {
		panic("Invalid configuration of webPageInfo")
	}

	return &webPageInfo{
		level:           level,
		webName:         web,
		contentPatterns: patterns,
	}
}

// chromeTab holds the information of a Chrome browser tab.
type chromeTab struct {
	conn           *chrome.Conn
	pageInfo       *webPageInfo // static information (e.g. the type of website being visited, which contents to search to click) of this tab
	url            string       // current url of the website being visited
	currentPattern int          // the index of current page's corresponding content pattern within pageInfo
}

func (tab *chromeTab) searchElementWithPatternAndClick(ctx context.Context, pattern string) error {
	if err := tab.conn.Eval(ctx, "window.location.href", &tab.url); err != nil {
		return errors.Wrap(err, "failed to get URL")
	}
	testing.ContextLogf(ctx, "Current URL: %q", tab.url)

	// Find the desired link and trigger navigation by clicking the link.
	// The link might have parameters or tokens. Find the element by matching with pattern (CSS selector, not
	// regular expression).
	//
	// TODO(crbug.com/1193417): remove setTimeout() after the bug is fixed.
	// Click on a link will trigger navigation immediately. If the page is navigated before the CDP returns,
	// the JSObject won't be able to release, and an error will be returned. Therefore timeout is used to
	// allow CDP return and object release before clicking the link.
	//
	// Use 3000 milliseconds timeout value before clicking anchor (bug: https://issuetracker.google.com/185467835).
	script := `(pattern) => {
			const name = "a[href*='" + pattern + "']";
			const els = document.querySelectorAll(name);
			if (els.length === 0) return false;
			setTimeout(() => { els[0].click(); }, 3000);
			return true;
		}`

	// In Case of the page is still loading and the element is not shown, here use polling to call the script
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var done bool
		if err := tab.conn.Call(ctx, &done, script, pattern); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to search and click on an element"))
		}
		if !done {
			return errors.New("element not found")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Minute, Interval: 5 * time.Second}); err != nil {
		return errors.Wrapf(err, "failed to find and click HTML element with pattern [%s] within %v", pattern, time.Minute)
	}

	// Navigation does not happen instantly. Use poll to detect whether it has been navigated to new URL.
	pollOpts := testing.PollOptions{Timeout: 30 * time.Second, Interval: 500 * time.Millisecond}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var urlAfter string
		if err := tab.conn.Eval(ctx, "window.location.href", &urlAfter); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get URL"))
		}
		if urlAfter == tab.url {
			return errors.New("page has not navigated")
		}
		tab.url = urlAfter
		return nil
	}, &pollOpts); err != nil {
		return errors.Wrapf(err, "failed to wait for web page been navigated within %v", pollOpts.Timeout)
	}

	testing.ContextLogf(ctx, "HTML element clicked [%s], page navigates to: %q", pattern, tab.url)
	tab.url = strings.TrimSuffix(tab.url, "/")

	return nil
}

func (tab *chromeTab) clickAnchor(ctx context.Context, timeout time.Duration, tconn *chrome.TestConn) error {
	p := tab.currentPattern
	pn := p + 1
	if pn == len(tab.pageInfo.contentPatterns) {
		pn = 0
	}

	if err := webutil.WaitForQuiescence(ctx, tab.conn, timeout); err != nil {
		return errors.Wrapf(err, "failed to wait for tab quiescence before clicking anchor within %v", timeout)
	}
	pattern := tab.pageInfo.contentPatterns[pn]
	testing.ContextLogf(ctx, "Click link and navigate from %q to %q", tab.pageInfo.contentPatterns[p], pattern)
	if err := tab.searchElementWithPatternAndClick(ctx, pattern); err != nil {
		// Check whether the failure to search and click pattern was due to issues on the content site.
		if contentSiteErr := contentSiteUnavailable(ctx, tconn); contentSiteErr != nil {
			return errors.Wrapf(contentSiteErr, "failed to show content on page %s", tab.url)
		}
		return errors.Wrapf(err, "failed to click anchor on page %s", tab.url)
	}
	tab.currentPattern = pn
	return nil
}

func contentSiteUnavailable(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	errorMessages := []string{
		"503 Service Temporarily Unavailable",
		"504 Gateway Time-out",
		"HTTP ERROR 404",
		"Our CDN was unable to reach our servers",
		"Apologies, but something went wrong on our end.",
		"This site can’t provide a secure connection",
		"This site can’t be reached",
	}

	for _, m := range errorMessages {
		node := nodewith.Name(m).Role(role.StaticText)
		if err := ui.Exists(node)(ctx); err == nil {
			return errors.Errorf("content site error - %s", m)
		}
	}
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
	tab.conn = nil
}

// chromeWindow is the struct for Chrome browser window. It holds multiple tabs.
type chromeWindow struct {
	tabs []*chromeTab
}

var allTargets = []struct {
	url  string
	info *webPageInfo
}{
	{"https://en.wikipedia.org/wiki/Main_Page", newPageInfo(Basic, wikipedia, `/Main_Page`, `/Wikipedia:Contents`)},
	{"https://en.wikipedia.org/wiki/Portal:Current_events", newPageInfo(Basic, wikipedia, `/Portal:Current_events`, `/Special:Random`)},
	{"https://en.wikipedia.org/wiki/Wikipedia:About", newPageInfo(Basic, wikipedia, `/Wikipedia:About`, `/Wikipedia:Contact_us`)},
	{"https://en.wikipedia.org/wiki/Help:Contents", newPageInfo(Plus, wikipedia, `/Help:Contents`, `/Help:Introduction`)},
	{"https://en.wikipedia.org/wiki/Wikipedia:Community_portal", newPageInfo(Plus, wikipedia, `/Wikipedia:Community_portal`, `/Special:RecentChanges`)},
	{"https://en.wikipedia.org/wiki/COVID-19_pandemic", newPageInfo(Premium, wikipedia, `/COVID-19_pandemic`, `/Coronavirus_disease_2019`)},

	{"https://www.reddit.com/r/wallstreetbets", newPageInfo(Basic, reddit, `/r/wallstreetbets/hot/`, `/r/wallstreetbets/new/`)},
	{"https://www.reddit.com/r/technews", newPageInfo(Basic, reddit, `/r/technews/hot/`, `/r/technews/new/`)},
	{"https://www.reddit.com/r/olympics", newPageInfo(Basic, reddit, `/r/olympics/hot/`, `/r/olympics/new/`)},
	{"https://www.reddit.com/r/programming", newPageInfo(Plus, reddit, `/r/programming/hot/`, `/r/programming/new/`)},
	{"https://www.reddit.com/r/apple", newPageInfo(Plus, reddit, `/r/apple/hot/`, `/r/apple/new/`)},
	{"https://www.reddit.com/r/brooklynninenine", newPageInfo(Premium, reddit, `/r/brooklynninenine/hot/`, `/r/brooklynninenine/new/`)},

	{"https://medium.com/tag/business", newPageInfo(Basic, medium, `/business`, `/entrepreneurship`)},
	{"https://medium.com/tag/startup", newPageInfo(Basic, medium, `/startup`, `/leadership`)},
	{"https://medium.com/tag/work", newPageInfo(Plus, medium, `/work`, `/productivity`)},
	{"https://medium.com/tag/software-engineering", newPageInfo(Premium, medium, `/software-engineering`, `/programming`)},
	{"https://medium.com/tag/artificial-intelligence", newPageInfo(Premium, medium, `/artificial-intelligence`, `/technology`)},

	{"https://news.yahoo.com/us/", newPageInfo(Basic, yahooNews, `/us/`, `/politics/`)},
	{"https://news.yahoo.com/world/", newPageInfo(Basic, yahooNews, `/world/`, `/health/`)},
	{"https://news.yahoo.com/science/", newPageInfo(Plus, yahooNews, `/science/`, `/tagged/skullduggery/`)},
	{"https://news.yahoo.com/originals/", newPageInfo(Premium, yahooNews, `/originals/`, `/videos`)},
	{"https://news.yahoo.com/videos/in-depth/", newPageInfo(Premium, yahooNews, `/videos/in-depth/`, `/videos/ideas-election/`)},

	{"https://edition.cnn.com/world", newPageInfo(Plus, cnn, `/world`, `/africa`)},
	{"https://edition.cnn.com/americas", newPageInfo(Plus, cnn, `/americas`, `/asia`)},
	{"https://edition.cnn.com/australia", newPageInfo(Plus, cnn, `/australia`, `/china`)},
	{"https://edition.cnn.com/europe", newPageInfo(Premium, cnn, `/europe`, `/india`)},
	{"https://edition.cnn.com/middle-east", newPageInfo(Premium, cnn, `/middle-east`, `/uk`)},

	{"https://www.espn.com/nfl/", newPageInfo(Plus, espn, `/nfl/scoreboard`, `/nfl/schedule`)},
	{"https://www.espn.com/nba/", newPageInfo(Plus, espn, `/nba/scoreboard`, `/nba/schedule`)},
	{"https://www.espn.com/mens-college-basketball/", newPageInfo(Plus, espn, `/mens-college-basketball/scoreboard`, `/mens-college-basketball/schedule`)},
	{"https://www.espn.com/tennis/", newPageInfo(Premium, espn, `/tennis/dailyResults`, `/tennis/schedule`)},
	{"https://www.espn.com/soccer/", newPageInfo(Premium, espn, `/soccer/scoreboard`, `/soccer/schedule`)},

	{"https://www.hulu.com/hub/movies", newPageInfo(Plus, hulu, `/hub/movies`, `/hub/originals`)},

	{"https://www.pinterest.com/ideas/", newPageInfo(Plus, pinterest, `/ideas/`, `/ideas/holidays/910319220330/`)},

	{"https://help.netflix.com/en/", newPageInfo(Premium, netflix, `/en`, `/en/legal/termsofuse`)},

	{"https://www.youtube.com", newPageInfo(Premium, youtube, `/`, `/feed/explore`)},
}

// generateTabSwitchTargets sets all web targets according to the input Level.
func generateTabSwitchTargets(caseLevel Level) ([]*chromeWindow, error) {
	winNum := 1
	tabNum := 0
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

	var targets []struct {
		url  string
		info *webPageInfo
	}

	for _, tgt := range allTargets {
		if tgt.info.level <= caseLevel {
			targets = append(targets, tgt)
		}
	}
	if len(targets) < winNum*tabNum {
		return nil, errors.New("no enough web page targets to construct tabs")
	}
	// Shuffle the URLs to random order.
	rand.Shuffle(len(targets), func(i, j int) { targets[i], targets[j] = targets[j], targets[i] })
	idx := 0
	windows := make([]*chromeWindow, winNum)
	for i := range windows {
		tabs := make([]*chromeTab, tabNum)
		for j := range tabs {
			tabs[j] = &chromeTab{conn: nil, pageInfo: targets[idx].info, url: targets[idx].url, currentPattern: 0}
			idx++
		}
		windows[i] = &chromeWindow{tabs: tabs}
	}

	return windows, nil
}

// Run2 runs the TabSwitchCUJ test. It is invoked by TabSwitchCujRecorder2 to
// record web contents via WPR and invoked by TabSwitchCUJ2 to execute the tests
// from the recorded contents. Additional actions will be executed in each tab.
func Run2(ctx context.Context, s *testing.State, cr *chrome.Chrome, caseLevel Level, isTablet bool) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API, error: ", err)
	}

	// Shorten the context to cleanup crastestclient and resume battery charging.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if _, ok := s.Var("ui.cuj_mute"); ok {
		if err := crastestclient.Mute(ctx); err != nil {
			s.Fatal("Failed to mute: ", err)
		}
		defer crastestclient.Unmute(cleanUpCtx)
	}

	// Put battery under discharge in order to collect the power consumption of the test.
	setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 50)
	if err != nil {
		s.Fatal("Failed to set battery discharge: ", err)
	}
	defer setBatteryNormal(cleanUpCtx)

	// Shorten the context to cleanup recorder.
	cleanUpRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	recorder, err := cuj.NewRecorder(ctx, cr, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a recorder, error: ", err)
	}
	defer recorder.Close(cleanUpRecorderCtx)

	windows, err := generateTabSwitchTargets(caseLevel)
	if err != nil {
		s.Fatal("Failed to generate tab targets: ", err)
	}

	var tsAction cuj.UIActionHandler
	if isTablet {
		if tsAction, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create tablet action handler: ", err)
		}
	} else {
		if tsAction, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create clamshell action handler: ", err)
		}
	}
	defer tsAction.Close()
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

	pv := perf.NewValues()

	timeTabsOpenStart := time.Now()
	// Launch browser and track the elapsed time.
	browserStartTime, err := cuj.GetBrowserStartTime(ctx, cr, tconn, isTablet)
	if err != nil {
		s.Fatal("Failed to launch Chrome: ", err)
	}
	s.Log("Browser start ms: ", browserStartTime)

	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))

	// Open all windows and tabs.
	if err := openAllWindowsAndTabs(ctx, cr, &windows, tsAction, caseLevel); err != nil {
		s.Fatal("Failed to open targets for tab switch: ", err)
	}

	// Total time used from beginning to load all pages.
	timeElapsed := time.Since(timeTabsOpenStart)
	s.Log("All tabs opened Elapsed: ", timeElapsed)

	pv.Set(perf.Metric{
		Name:      "TabSwitchCUJ.ElapsedTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(timeElapsed.Milliseconds()))

	// Shorten context a bit to allow for cleanup if Run fails.
	shorterCtx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	if err = recorder.Run(shorterCtx, func(ctx context.Context) error {
		return tabSwitchAction(ctx, cr, tconn, &windows, tsAction, caseLevel)
	}); err != nil {
		s.Fatal("Failed to execute tab switch action: ", err)
	}

	// Use a short timeout value so it can return fast in case of failure.
	recordCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := recorder.Record(recordCtx, pv); err != nil {
		s.Fatal("Failed to report, error: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values, error: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Error("Failed to save histogram raw data: ", err)
	}
}

func openAllWindowsAndTabs(ctx context.Context, cr *chrome.Chrome, targets *[]*chromeWindow, tsAction cuj.UIActionHandler, caseLevel Level) (err error) {
	windows := (*targets)
	plTimeout := pageLoadingTimeout(caseLevel)
	for idxWindow, window := range windows {
		for idxTab, tab := range window.tabs {
			testing.ContextLogf(ctx, "Opening window %d, tab %d", idxWindow+1, idxTab+1)

			if tab.conn, err = tsAction.NewChromeTab(ctx, cr, tab.url, idxTab == 0); err != nil {
				return errors.Wrap(err, "failed to create new Chrome tab")
			}

			if err := webutil.WaitForRender(ctx, tab.conn, plTimeout); err != nil {
				return errors.Wrap(err, "failed to wait for render to finish")
			}

			// In replay mode, user won't be able to know whether the page is quiescence or not,
			// and it is not necessary to wait for quiescence in replay mode.
			// In record mode, needs to wait for quiescence to properly record web content.
			if caseLevel == Record {
				if err := webutil.WaitForQuiescence(ctx, tab.conn, plTimeout); err != nil {
					return errors.Wrapf(err, "failed to wait for tab to achieve quiescence within %v", plTimeout)
				}
			}
		}
	}

	return nil
}

func tabSwitchAction(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, targets *[]*chromeWindow, tsAction cuj.UIActionHandler, caseLevel Level) error {
	windows := (*targets)
	scrollActions := tsAction.ScrollChromePage(ctx)
	plTimeout := pageLoadingTimeout(caseLevel)

	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to check installed chrome browser")
	}

	for idx, window := range windows {
		testing.ContextLogf(ctx, "Switching to window #%d", idx+1)
		if err := tsAction.SwitchToAppWindowByIndex(chromeApp.Name, idx)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch window")
		}

		tabTotalNum := len(window.tabs)
		for tabIdx := 0; tabIdx < tabTotalNum; tabIdx++ {
			testing.ContextLogf(ctx, "Switching tab to window %d, tab %d", idx+1, tabIdx+1)

			if err := tsAction.SwitchToChromeTabByIndex(tabIdx)(ctx); err != nil {
				return errors.Wrap(err, "failed to switch tab")
			}
			tab := window.tabs[tabIdx]

			timeStart := time.Now()
			if err := webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
				return errors.Wrap(err, "failed to wait for the tab to be visible")
			}
			renderTime := time.Since(timeStart)
			// Debugging purpose message, to observe which tab takes unusual long time to render.
			testing.ContextLog(ctx, "Tab rendering time after switching: ", renderTime)
			if caseLevel == Record {
				if err := webutil.WaitForQuiescence(ctx, tab.conn, plTimeout); err != nil {
					return errors.Wrapf(err, "failed to wait for tab to achieve quiescence within %v", plTimeout)
				}
				quiescenceTime := time.Now().Sub(timeStart)
				// Debugging purpose message, to observe which tab takes unusual long time to quiescence
				testing.ContextLog(ctx, "Tab quiescence time after switching: ", quiescenceTime)
			}

			// To reduce total execution time of this test case,
			// these specific websites has been chosen to do scroll actions as per requirement.
			if tab.pageInfo.webName == wikipedia || tab.pageInfo.webName == hulu || tab.pageInfo.webName == youtube {
				for _, act := range scrollActions {
					if err := act(ctx); err != nil {
						return errors.Wrap(err, "failed to execute action")
					}
					// Make sure the whole web content is recorded only under Recording.
					if caseLevel == Record {
						if err := webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
							return errors.Wrap(err, "failed to wait for render to finish after scroll")
						}
						if err := webutil.WaitForQuiescence(ctx, tab.conn, plTimeout); err != nil {
							return errors.Wrapf(err, "failed to wait for tab to achieve quiescence after scroll within %v", plTimeout)
						}
					}
				}
			}

			// Click on 1 link per 2 tabs, or click on 1 link for every tab under Record mode to ensure all links are
			// accessible under any other levels.
			if tabIdx%2 == 0 || caseLevel == Record {
				if err := tab.clickAnchor(ctx, plTimeout, tconn); err != nil {
					return errors.Wrap(err, "failed to click anchor")
				}
				if caseLevel == Record {
					// Ensure contents are renderred in recording mode.
					if err := webutil.WaitForRender(ctx, tab.conn, plTimeout); err != nil {
						return errors.Wrap(err, "failed to wait for render to finish")
					}
					if err := webutil.WaitForQuiescence(ctx, tab.conn, plTimeout); err != nil {
						return errors.Wrap(err, "failed to wait for tab to achieve quiescence")
					}
				} else {
					// It is normal that tabs might remain loading, hence no handle error here.
					webutil.WaitForQuiescence(ctx, tab.conn, clickLinkTimeout)
				}
			}
		}
	}
	return nil
}
