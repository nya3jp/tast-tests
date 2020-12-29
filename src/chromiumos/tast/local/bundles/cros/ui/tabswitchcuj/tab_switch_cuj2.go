// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tabswitchcuj contains the test code for TabSwitchCUJ. The test is
// extracted into this package to be shared between TabSwitchCUJRecorder and
// TabSwitchCUJ.
package tabswitchcuj

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// tabSwitchAction define related action of this test case,
// which these actions have to implement in multiple ways due to the UI represent differently on clamshell and tablet.
type tabSwitchAction interface {
	// launchChrome launches the Chrome browser.
	launchChrome(ctx context.Context) (time.Time, error)
	// newTab creates a new tab of Google Chrome.
	newTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error)
	// switchWindow switches the Chrome tab from one to another.
	switchWindow(ctx context.Context, idxWindow, numWindows int) error
	// switchTab switches the Chrome tab from one to another.
	switchTab(ctx context.Context, tabIdxSrc, tabIdxDest, tabTotalNum int) error
	// scrollPage generate the scroll actions.
	scrollPage() []func(ctx context.Context, conn *chrome.Conn) error
	// pageRefresh refresh a web page (current focus page).
	pageRefresh(ctx context.Context) error
}

// clamshellActionHandler define the action on clamshell devices.
type clamshellActionHandler struct {
	tconn        *chrome.TestConn
	kb           *input.KeyboardEventWriter
	pad          *input.TrackpadEventWriter
	touchPad     *input.TouchEventWriter
	clickHandler cuj.InputAction
}

// clamshellActionHandler define the action on tablet devices.
type tabletActionHandler struct {
	tconn        *chrome.TestConn
	touchScreen  *input.TouchscreenEventWriter
	stw          *input.SingleTouchEventWriter
	tc           *pointer.TouchController
	tcc          *input.TouchCoordConverter
	clickHandler cuj.InputAction
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
		return urlLink{}
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

type chromeWindow struct {
	tabs []*chromeTab
}

// getTargets sets all web targets according to input Level.
func getTargets(caseLevel Level) []*chromeWindow {
	var allLinks = []urlLink{
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

		newURLLink(Basic, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGRqTVhZU0FtVnVHZ0pWVXlnQVAB", []string{`second last`, `last`}),   // topics: Technology
		newURLLink(Basic, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNREpxYW5RU0FtVnVHZ0pWVXlnQVAB", []string{`second last`, `last`}),   // topics: Entertainment
		newURLLink(Plus, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp1ZEdvU0FtVnVHZ0pWVXlnQVAB", []string{`second last`, `last`}),    // topics: Sports
		newURLLink(Premium, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp0Y1RjU0FtVnVHZ0pWVXlnQVAB", []string{`second last`, `last`}), // topics: Science
		newURLLink(Premium, googleNews, "https://news.google.com/topics/CAAqIQgKIhtDQkFTRGdvSUwyMHZNR3QwTlRFU0FtVnVLQUFQAQ", []string{`second last`, `last`}),       // topics: Health

		newURLLink(Basic, cnn, "https://edition.cnn.com/world", []string{`/world`, `/africa`}),
		newURLLink(Basic, cnn, "https://edition.cnn.com/americas", []string{`/americas`, `/asia`}),
		newURLLink(Plus, cnn, "https://edition.cnn.com/australia", []string{`/australia`, `/china`}),
		newURLLink(Premium, cnn, "https://edition.cnn.com/europe", []string{`/europe`, `/india`}),
		newURLLink(Premium, cnn, "https://edition.cnn.com/middle-east", []string{`/middle-east`, `/uk`}),

		newURLLink(Basic, espn, "https://www.espn.com/nfl/", []string{`/nfl/scoreboard`, `/nfl/schedule`}),
		newURLLink(Basic, espn, "https://www.espn.com/nba/", []string{`/nba/scoreboard`, `/nba/schedule`}),
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
// record web contents via WPR and invoked by TabSwitchCUJ2 to exercise the tests
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

	var tsAction tabSwitchAction

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

	scrollActions := tsAction.scrollPage()

	var (
		timeElapsedBrowserLaunch, timeElapsedTabsOpened time.Duration
		timeBrowserLaunchStart                          time.Time
		timeTabsOpenStart                               = time.Now()
	)

	// Launch browser and track the elapsed time.
	if timeBrowserLaunchStart, err = tsAction.launchChrome(ctx); err != nil {
		s.Fatal("Failed to launch Chrome: ", err)
	}
	timeElapsedBrowserLaunch = time.Since(timeBrowserLaunchStart)
	s.Log("Browser start ms: ", timeElapsedBrowserLaunch.Milliseconds())

	// Open all windows and tabs.
	for idxWindow, window := range windows {
		for idxTab, tab := range window.tabs {
			var c *chrome.Conn
			s.Logf("Opening window %d, tab %d", idxWindow+1, idxTab+1)
			if idxWindow == 0 && idxTab == 0 {
				if c, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/")); err != nil {
					s.Fatal("Failed to find new tab: ", err)
				}
				if err = c.Navigate(ctx, tab.link.url); err != nil {
					s.Fatalf("Failed to navigate to %s, error: %+v", tab.link.url, err)
				}
			} else {
				if c, err = tsAction.newTab(ctx, cr, tab.link.url, idxTab == 0); err != nil {
					s.Fatal("Failed to create new Chrome tab: ", err)
				}
			}

			if err := webutil.WaitForRender(ctx, c, pageLoadingTimeout); err != nil {
				s.Fatal("Failed to wait for finish render: ", err)
			}

			// In replay mode, user won't able to know whether the page is quiescence or not,
			// and it is not necessary to wait for quiescence in replay mode.
			// In record mode, needs to wait for quiescence to properly record web content.
			if caseLevel == Record {
				if err := webutil.WaitForQuiescence(ctx, c, pageLoadingTimeout); err != nil {
					s.Fatal("Failed to wait for tab quiescence: ", err)
				}
			}

			tab.conn = c
			// Close the tab before finishing the test.
			defer tab.close(ctx, s)
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
			s.Logf("Switching to window %d", idx+1)
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
				// Debugging purpose message, to observe which tab takes unusual time to render.
				s.Logf("Tab rendering time after switching: %s", renderTime)
				if caseLevel == Record {
					if err := webutil.WaitForQuiescence(ctx, tab.conn, pageLoadingTimeout); err != nil {
						s.Fatal("Failed to wait for tab quiescence: ", err)
					}
					quiescenceTime := time.Now().Sub(timeStart)
					// Debugging purpose message, to observe which tab takes unusual time to quiescence
					s.Logf("Tab quiescence time after switching: %s", quiescenceTime)
				}

				// To reduce total execution time of this test case,
				// these specific websites has been chosen to do scroll action as per requirement.
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

					if caseLevel == Record {
						// The chosen content of google news web site is not a link, it a <div>,
						// when a click happen, the page is not redirect, this cause blank content on Replay mode
						// Here trigger redirection by refresh the page, otherwise on Replay mode
						// when after click on a element, the content will never shows up.
						if tab.link.webName == googleNews {
							if err := tsAction.pageRefresh(ctx); err != nil {
								s.Error("Failed to refresh: ", err)
							}
						}
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

func clickAnchor(ctx context.Context, c *chrome.Conn, tab *chromeTab, pattern string) error {
	if err := c.Eval(ctx, "window.location.href", &tab.currentURL); err != nil {
		return errors.Wrap(err, "failed to get URL before click anchor")
	}
	testing.ContextLogf(ctx, "Current URL: %s", tab.currentURL)

	// Click on a link will trigger redirect,
	// if the page is redirect first and then the CDP return, the JSObject won't able to release,
	// therefore, allow CDP return first is necessary.

	var script string
	if tab.link.webName == googleNews {
		// Customization made for Google News - the content is not a common link, can't match them by href.
		pos := 1 // the last one
		if pattern == `second last` {
			pos = 2 // the second lase one
		}
		script = fmt.Sprintf(`() => {
			var els = document.getElementsByClassName("ThdJC kaAt2 GFO5Jd")
			var size = els.length;
			if (size < 2)
				return false;
			setTimeout(() => { els[size-%d].click(); }, 500);
			return true;
		}`, pos)
	} else {
		// Link might with parameter or token, therefore, better find the element by
		// match with pattern (CSS selector, not regular expression).
		script = `(pattern) => {
			var name = "a[href*='" + pattern + "']";
			var els = document.querySelectorAll(name);
			if (els.length == 0)
				return false;
			setTimeout(() => { els[0].click(); }, 500);
			return true;
		}`
	}

	var done bool
	if err := c.Call(ctx, &done, script, pattern); err != nil || !done {
		return errors.Wrap(err, "failed to click anchor")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var urlAfter string
		if err := c.Eval(ctx, "window.location.href", &urlAfter); err != nil {
			return errors.Wrap(err, "failed to get URL before click anchor")
		}
		if urlAfter == tab.currentURL {
			return errors.New("page is not redirect")
		}
		tab.currentURL = urlAfter
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
		return errors.Wrapf(err, "failed to click HTML element with pattern [%v]", pattern)
	}

	testing.ContextLogf(ctx, "Anchor clicked [%s], page is redirect to: %s", pattern, tab.currentURL)
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

	ts := time.Now()
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if e := clickAnchor(ctx, tab.conn, tab, pattern); e != nil {
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
	if err := tab.conn.CloseTarget(ctx); err != nil {
		s.Error("Failed to close target, error: ", err)
	}
	if err := tab.conn.Close(); err != nil {
		s.Error("Failed to close the connection, error: ", err)
	}
}

// newTabletActionHandler return the action handler which is responsible for handle tabSwitchAction on tablet.
func newTabletActionHandler(tconn *chrome.TestConn, tc *pointer.TouchController) *tabletActionHandler {
	stw := tc.EventWriter()
	tcc := tc.TouchCoordConverter()
	return &tabletActionHandler{
		tconn:        tconn,
		touchScreen:  tc.Touchscreen(),
		stw:          stw,
		tc:           tc,
		tcc:          tcc,
		clickHandler: cuj.NewTouchActionHandler(stw, tcc),
	}
}

// launchChrome launches the Chrome browser.
func (t tabletActionHandler) launchChrome(ctx context.Context) (time.Time, error) {
	return t.clickChromeOnHotseat(ctx)
}

func (t tabletActionHandler) clickChromeOnHotseat(ctx context.Context) (time.Time, error) {
	return cuj.LaunchAppFromHotseat(ctx, t.tconn, "Chrome", "Chromium")
}

// showTabList shows the tab list by click a button on the Chrome tool bar.
func (t tabletActionHandler) showTabList(ctx context.Context) error {
	// Swipe down a little bit to ensure Chrome's tool bar is shown.
	x := t.touchScreen.Width() / 2
	y0 := t.touchScreen.Height() / 2
	y1 := y0 + t.touchScreen.Height()/10
	if err := t.stw.Swipe(ctx, x, y0, x, y1, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to swipe down")
	}
	if err := t.stw.End(); err != nil {
		return errors.Wrap(err, "failed to end swipe-down")
	}

	// Find button on the tool bar and click it.
	paramToggle := ui.FindParams{
		Role:       ui.RoleTypeButton,
		Attributes: map[string]interface{}{"name": regexp.MustCompile("toggle tab strip")},
	}
	nodes, err := ui.FindAll(ctx, t.tconn, paramToggle)
	if err != nil || len(nodes) <= 0 {
		return errors.Wrap(err, "failed to find toggle button")
	}
	defer nodes.Release(ctx)

	// Click the last found one.
	return t.clickHandler.LeftClick(ctx, nodes[len(nodes)-1])
}

// newTab creates a new tab of Google Chrome.
// newWindow decide this new tab should open in current Chrome window or open in new Chrome window.
func (t tabletActionHandler) newTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	if newWindow {
		return cr.NewConn(ctx, url, cdputil.WithNewWindow())
	}

	if err := t.showTabList(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to open the tab list")
	}

	param := ui.FindParams{
		Role: ui.RoleTypeButton,
		Name: "New tab",
	}
	if err := t.clickHandler.StableFindAndClick(ctx, t.tconn, param, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to find and click new tab button")
	}

	c, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to find new tab")
	}
	if err = c.Navigate(ctx, url); err != nil {
		return c, errors.Wrapf(err, "failed to navigate to %s, error: %v", url, err)
	}

	return c, nil
}

// switchWindow switches the Chrome tab from one to another.
func (t tabletActionHandler) switchWindow(ctx context.Context, idxWindow, numWindows int) error {
	// No need to switch if there is only one window exist.
	if numWindows <= 1 {
		return nil
	}

	// Ensure hotseat is shown.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, t.tconn, t.stw, t.tcc); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to show hotseat")
	}

	uiPollOpt := testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}
	// Click Google Chrome on hotseat.
	if _, err := t.clickChromeOnHotseat(ctx); err != nil {
		return errors.Wrap(err, "failed to find and click new app button on hotseat")
	}

	// Find list of opened windows of Google Chrome.
	menuFindParams := ui.FindParams{ClassName: "MenuItemView"}
	if err := ui.WaitUntilExists(ctx, t.tconn, menuFindParams, uiPollOpt.Timeout); err != nil {
		return errors.Wrap(err, "expected to see menu items, but not seen")
	}
	items, err := ui.FindAll(ctx, t.tconn, menuFindParams)
	if err != nil {
		return errors.Wrap(err, "can't find the menu items")
	}
	defer items.Release(ctx)

	// There should be (numWindows+1) of items on the menu list:
	// the first is "Google Chrome", and the rest should be opened windows (order by open time).
	if idxWindow > len(items)-1 {
		return errors.New("windows number is not match")
	}

	// skip the first one
	return t.clickHandler.LeftClick(ctx, items[idxWindow+1])
}

// switchTab switches the Chrome tab from one to another.
// Assume that tablet is only show one window at a time.
func (t tabletActionHandler) switchTab(ctx context.Context, tabIdxSrc, tabIdxDest, tabTotalNum int) error {
	// No need to switch if there is only one tab exist.
	if tabTotalNum <= 1 {
		return nil
	}
	if tabTotalNum < tabIdxSrc || tabTotalNum < tabIdxDest {
		return errors.New("invalid parameters for switch tab")
	}

	// Open tab list.
	if err := t.showTabList(ctx); err != nil {
		return errors.Wrap(err, "failed to open the tab list")
	}
	// Target tab item could be out of screen.
	// Scroll the tab list to ensure the item is visible.
	testing.ContextLog(ctx, "Scrolling the tab list before tab switch")

	// Find all tab list.
	tabListContainers, err := ui.FindAll(ctx, t.tconn, ui.FindParams{Role: ui.RoleTypeTabList})
	if err != nil {
		return errors.Wrap(err, "failed to find tab list container")
	}
	defer tabListContainers.Release(ctx)

	var tabListContainer *ui.Node
	// Looking for the node located at top left of the screen, which is the current window.
	for _, tl := range tabListContainers {
		if tl.Location.Top == 0 && tl.Location.Left == 0 {
			tabListContainer = tl
			break
		}
	}
	if tabListContainer == nil {
		return errors.Wrap(err, "failed to find active tab list container")
	}
	// Wait for change completed before clicking.
	if err := ui.WaitForLocationChangeCompletedOnNode(ctx, t.tconn, tabListContainer); err != nil {
		return errors.Wrap(err, "failed to wait for location changes completed on tab list")
	}

	// Find tab items under tabListContainer.
	tabs, err := tabListContainer.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeTab})
	if err != nil {
		return errors.Wrap(err, "failed to find current focused tab item")
	}
	defer tabs.Release(ctx)
	if len(tabs) < tabTotalNum {
		return errors.New("error on finding tab items")
	}

	var (
		swipeDistance    input.TouchCoord
		onscreenTabWidth int
	)
	succ := false
	// Find two adjacent items which are both fully in-screen to calculate the swipe distance.
	for i := 0; i < len(tabs)-1; i++ {
		onscreen1 := !tabs[i].State[ui.StateTypeOffscreen]
		onscreen2 := !tabs[i+1].State[ui.StateTypeOffscreen]
		width1 := tabs[i].Location.Width
		width2 := tabs[i+1].Location.Width
		if onscreen1 && onscreen2 && width1 == width2 {
			x0, _ := t.tcc.ConvertLocation(tabs[i].Location.CenterPoint())
			x1, _ := t.tcc.ConvertLocation(tabs[i+1].Location.CenterPoint())
			swipeDistance = x1 - x0
			onscreenTabWidth = width1
			succ = true
			break
		}
	}
	if !succ {
		return errors.Wrap(err, "failed to find two adjacency tab items within screen")
	}

	tabsNew := tabs
	// Check if swipe is required to show the target tab.
	if tabs[tabIdxDest].State[ui.StateTypeOffscreen] || tabs[tabIdxDest].Location.Width < onscreenTabWidth {
		swipeDirection := 1 // The direction of swipe. Default is right swipe.
		if tabIdxDest < tabIdxSrc {
			// Left swipe.
			swipeDirection = -1
		}

		var (
			swipeTimes = int(math.Abs(float64(tabIdxDest - tabIdxSrc)))
			ptSrc      = tabListContainer.Location.CenterPoint()
			x0, y0     = t.tcc.ConvertLocation(ptSrc)
			x1, y1     = x0 + swipeDistance*input.TouchCoord(swipeDirection), y0
		)

		// Do scroll.
		// The total swipe distance might greater than screen size, which means the destination point might out of screen
		// needs to separate them otherwise the swipe won't work.
		for i := 0; i < swipeTimes; i++ {
			if err := t.stw.Swipe(ctx, x1, y1, x0, y0, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to Swipe down")
			}
			if err := t.stw.End(); err != nil {
				return errors.Wrap(err, "failed to end a touch")
			}
		}

		if err := ui.WaitForLocationChangeCompletedOnNode(ctx, t.tconn, tabListContainer); err != nil {
			return errors.Wrap(err, "failed to wait for location changes")
		}
		testing.ContextLog(ctx, "Scroll complete, ready for tab switch")
		// Find tab items again since the position is changed after scroll.
		if tabsNew, err = tabListContainer.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeTab}); err != nil {
			return errors.Wrap(err, "failed to find current focused tab items after scroll")
		}
		defer tabsNew.Release(ctx)
	}

	if len(tabsNew) < tabTotalNum {
		return errors.Errorf("tab num %d is different with expected number %d", len(tabsNew), tabTotalNum)
	}

	return t.clickHandler.LeftClick(ctx, tabsNew[tabIdxDest])
}

// pageRefresh refresh a web page (current focus page).
func (t tabletActionHandler) pageRefresh(ctx context.Context) error {
	params := ui.FindParams{Name: "Reload", Role: ui.RoleTypeButton, ClassName: "ReloadButton"}
	return t.clickHandler.StableFindAndClick(ctx, t.tconn, params, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second})
}

// scrollPage generate the scroll action.
func (t tabletActionHandler) scrollPage() []func(ctx context.Context, conn *chrome.Conn) error {
	var (
		x      = t.touchScreen.Width() / 2
		ystart = t.touchScreen.Height() / 4 * 3 // 75% of screen height
		yend   = t.touchScreen.Height() / 4     // 25% of screen height
	)

	// Swipe the page down.
	swipeDown := func(ctx context.Context, conn *chrome.Conn) error {
		if err := t.stw.Swipe(ctx, x, ystart, x, yend, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		if err := t.stw.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	// Swipe the page up.
	swipeUp := func(ctx context.Context, conn *chrome.Conn) error {
		if err := t.stw.Swipe(ctx, x, yend, x, ystart, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		if err := t.stw.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	return []func(ctx context.Context, conn *chrome.Conn) error{
		swipeDown,
		swipeUp,
		swipeUp,
	}
}

// newClamshellActionHandler return the action handler which is responsible for handle tabSwitchAction on clamshell.
func newClamshellActionHandler(tconn *chrome.TestConn, kb *input.KeyboardEventWriter, pad *input.TrackpadEventWriter, touchPad *input.TouchEventWriter) *clamshellActionHandler {
	return &clamshellActionHandler{
		tconn:        tconn,
		kb:           kb,
		pad:          pad,
		touchPad:     touchPad,
		clickHandler: cuj.NewMouseActionHandler(),
	}
}

// launchChrome launches the Chrome browser.
func (cl clamshellActionHandler) launchChrome(ctx context.Context) (time.Time, error) {
	return cuj.LaunchAppFromShelf(ctx, cl.tconn, "Chrome", "Chromium")
}

// showTabList shows the tab list by click a button on the Chrome tool bar,
// which is useless in clamshell (assume Chrome is not into fullscreen mode).
func (cl clamshellActionHandler) showTabList(ctx context.Context) error {
	return nil
}

// newTab creates a new tab of Google Chrome.
// newWindow decide this new tab should open in current Chrome window or open in new Chrome window.
func (cl clamshellActionHandler) newTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error) {
	if newWindow {
		return cr.NewConn(ctx, url, cdputil.WithNewWindow())
	}

	if err := cl.kb.Accel(ctx, "Ctrl+T"); err != nil {
		return nil, errors.Wrap(err, "failed to hit Ctrl-T")
	}

	c, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to find new tab")
	}
	if err = c.Navigate(ctx, url); err != nil {
		return c, errors.Wrapf(err, "failed to navigate to %s, error: %v", url, err)
	}

	return c, nil
}

// switchWindow switches the Chrome tab from one to another.
func (cl clamshellActionHandler) switchWindow(ctx context.Context, idxWindow, numWindows int) error {
	// No need to switch if there is only one window exist.
	if numWindows <= 1 {
		return nil
	}

	if err := cl.kb.AccelPress(ctx, "Alt"); err != nil {
		return errors.Wrap(err, "failed to execute key event")
	}
	for i := 1; i < numWindows; i++ {
		if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
		if err := cl.kb.AccelPress(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to execute key event")
		}
		if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
		if err := cl.kb.AccelRelease(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to execute key event")
		}
	}

	return cl.kb.AccelRelease(ctx, "Alt")
}

// switchTab switches the Chrome tab from one to another.
func (cl clamshellActionHandler) switchTab(ctx context.Context, tabIdxSrc, tabIdxDest, tabTotalNum int) error {
	// No need to switch if there is only one tab exist.
	if tabTotalNum <= 1 {
		return nil
	}
	if tabTotalNum < tabIdxSrc || tabTotalNum < tabIdxDest {
		return errors.New("invalid parameters for switch tab")
	}

	if err := cl.kb.Accel(ctx, "Ctrl+Tab"); err != nil {
		return errors.Wrap(err, "failed to hit ctrl-tab")
	}

	return nil
}

// pageRefresh refresh a web page (current focus page).
func (cl clamshellActionHandler) pageRefresh(ctx context.Context) error {
	return cl.kb.Accel(ctx, "refresh")
}

// scrollPage generate the scroll action.
func (cl clamshellActionHandler) scrollPage() []func(ctx context.Context, conn *chrome.Conn) error {
	var (
		x      = cl.pad.Width() / 2
		ystart = cl.pad.Height() / 4
		yend   = cl.pad.Height() / 4 * 3
		d      = cl.pad.Width() / 8 // x-axis distance between two fingers
	)

	// Move the mouse cursor to the center so the scrolling will be effected on the web page.
	prepare := func(ctx context.Context, conn *chrome.Conn) error {
		if err := cl.mouseMoveToTabCenter(ctx, conn); err != nil {
			if err = cl.mouseMoveToScreenCenter(ctx); err != nil {
				return errors.Wrap(err, "failed to prepare DoubleSwipe")
			}
		}
		return nil
	}

	// Swipe the page down.
	doubleSwipeDown := func(ctx context.Context, conn *chrome.Conn) error {
		if err := cl.touchPad.DoubleSwipe(ctx, x, ystart, x, yend, d, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe down")
		}
		if err := cl.touchPad.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	// Swipe the page up.
	doubleSwipeUp := func(ctx context.Context, conn *chrome.Conn) error {
		if err := cl.touchPad.DoubleSwipe(ctx, x, yend, x, ystart, d, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe up")
		}
		if err := cl.touchPad.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	return []func(ctx context.Context, conn *chrome.Conn) error{
		prepare,
		doubleSwipeDown,
		doubleSwipeUp,
		doubleSwipeUp,
	}
}

func (cl clamshellActionHandler) mouseMoveToScreenCenter(ctx context.Context) error {
	displayInfo, err := display.GetInternalInfo(ctx, cl.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}
	if err = mouse.Move(ctx, cl.tconn, displayInfo.Bounds.CenterPoint(), 0); err != nil {
		return errors.Wrap(err, "failed to move the mouse cursor to the center")
	}

	return nil
}

func (cl clamshellActionHandler) mouseMoveToTabCenter(ctx context.Context, conn *chrome.Conn) error {
	var title string
	if err := conn.Eval(ctx, "document.title", &title); err != nil {
		return errors.Wrap(err, "failed to get current tab's title")
	}

	paramsWindow := ui.FindParams{Role: ui.RoleTypeWindow, Name: title}
	window, err := ui.StableFind(ctx, cl.tconn, paramsWindow, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second})
	if err != nil {
		return errors.Wrap(err, "failed to find current tab window")
	}
	defer window.Release(ctx)

	center := window.Location.CenterPoint()
	if err := mouse.Move(ctx, cl.tconn, center, 0); err != nil {
		return errors.Wrap(err, "failed to move mouse")
	}

	return nil
}
