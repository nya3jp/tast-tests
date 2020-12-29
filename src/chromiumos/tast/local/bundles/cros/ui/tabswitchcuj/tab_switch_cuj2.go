// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tabswitchcuj contains the test code for TabSwitchCUJ. The test is
// extracted into this package to be shared between TabSwitchCUJRecorder and
// TabSwitchCUJ.
//
// Steps to update the test:
//   1. Make changes in this package.
//   2. "tast run $IP ui.TabSwitchCujRecorder" to record the contents.
//      Look for the recorded wpr archive in /tmp/tab_switch_cuj.wprgo.
//   3. Update the recorded wpr archive to cloud storage under
//      gs://chromiumos-test-assets-public/tast/cros/ui/
//      It is recommended to add a date suffix to make it easier to change.
//   4. Update "tab_switch_cuj.wprgo.external" file under ui/data.
//   5. "tast run $IP ui.TabSwitchCuj" locally to make sure tests works
//      with the new recorded contents.
//   6. Submit the changes here with updated external data reference.
package tabswitchcuj

import (
	"context"
	"fmt"
	"math"
	"regexp"
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

type dutMode int

const (
	clamshell dutMode = iota
	tablet
)

// tabSwitchAction define related action of this test case,
// which these actions have to implement in multiple ways due to the UI represent differently on clamshell and tablet.
type tabSwitchAction interface {
	// init executes some actions to ensure the follow action execute properly as initiate process (no resource allocation involve).
	init(ctx context.Context) error
	// launchChrome launches the Chrome browser.
	launchChrome(ctx context.Context) (time.Time, error)
	// showTabList shows the tab list by click a button on the Chrome tool bar.
	showTabList(ctx context.Context) error
	// newTab creates a new tab of Google Chrome.
	newTab(ctx context.Context, cr *chrome.Chrome, url string, newWindow bool) (*chrome.Conn, error)
	// switchWindow switches the Chrome tab from one to another.
	switchWindow(ctx context.Context, idxWindow, numWindows int) error
	// switchTab switches the Chrome tab from one to another.
	switchTab(ctx context.Context, tabIdxSrc, tabIdxDest, tabTotalNum int) error
	// tabExtraActions generate the extra action other than tab switch such as swipe.
	tabExtraActions(ctx context.Context) []func(ctx context.Context) error
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

type urlIndicator int32

const (
	entry urlIndicator = iota
	content1
	content2
)

type urlLink struct {
	level           Level // the corredponding level of this link
	webName         webType
	url             string
	contentPattern1 string // the url of this web site
	contentPattern2 string // the content link inside the page
	indicator       urlIndicator
}

type chromeTab struct {
	conn *chrome.Conn
	link urlLink
}

type chromeWindow struct {
	tabs []chromeTab
}

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

// getTargets sets all web targets according to input Level.
func getTargets(caseLevel Level) []chromeWindow {
	var allLinks = [...]urlLink{
		{Basic, wikipedia, "https://en.wikipedia.org/wiki/Main_Page", `/Main_Page`, `/Wikipedia:Contents`, content1},
		{Basic, wikipedia, "https://en.wikipedia.org/wiki/Portal:Current_events", `/Portal:Current_events`, `/Special:Random`, content1},
		{Basic, wikipedia, "https://en.wikipedia.org/wiki/Wikipedia:About", `/Wikipedia:About`, `/Wikipedia:Contact_us`, content1},
		{Plus, wikipedia, "https://en.wikipedia.org/wiki/Help:Contents", `/Help:Contents`, `/Help:Introduction`, content1},
		{Plus, wikipedia, "https://en.wikipedia.org/wiki/Wikipedia:Community_portal", `/Wikipedia:Community_portal`, `/Special:RecentChanges`, content1},
		{Premium, wikipedia, "https://en.wikipedia.org/wiki/COVID-19_pandemic", `/COVID-19_pandemic`, `/Coronavirus_disease_2019`, content1},

		{Basic, reddit, "https://www.reddit.com/r/wallstreetbets", `/r/wallstreetbets/hot/`, `/r/wallstreetbets/new/`, entry},
		{Basic, reddit, "https://www.reddit.com/r/technews", `/r/technews/hot/`, `/r/technews/new/`, entry},
		{Basic, reddit, "https://www.reddit.com/r/olympics", `/r/olympics/hot/`, `/r/olympics/new/`, entry},
		{Plus, reddit, "https://www.reddit.com/r/programming", `/r/programming/hot/`, `/r/programming/new/`, entry},
		{Plus, reddit, "https://www.reddit.com/r/apple", `/r/apple/hot/`, `/r/apple/new/`, entry},
		{Premium, reddit, "https://www.reddit.com/r/brooklynninenine", `/r/brooklynninenine/hot/`, `/r/brooklynninenine/new/`, entry},

		{Basic, medium, "https://medium.com/topic/business", `/topic/business`, `/topic/money`, content1},
		{Basic, medium, "https://medium.com/topic/startups", `/topic/startups`, `/topic/leadership`, content1},
		{Plus, medium, "https://medium.com/topic/work", `/topic/work`, `/topic/freelancing`, content1},
		{Premium, medium, "https://medium.com/topic/software-engineering", `/software-engineering`, `/topic/programming`, content1},
		{Premium, medium, "https://medium.com/topic/artificial-intelligence", `/artificial-intelligence`, `/topic/technology`, content1},

		{Basic, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGRqTVhZU0FtVnVHZ0pWVXlnQVAB", `second last`, `last`, entry},   // topics: Technology
		{Basic, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNREpxYW5RU0FtVnVHZ0pWVXlnQVAB", `second last`, `last`, entry},   // topics: Entertainment
		{Plus, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp1ZEdvU0FtVnVHZ0pWVXlnQVAB", `second last`, `last`, entry},    // topics: Sports
		{Premium, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp0Y1RjU0FtVnVHZ0pWVXlnQVAB", `second last`, `last`, entry}, // topics: Science
		{Premium, googleNews, "https://news.google.com/topics/CAAqIQgKIhtDQkFTRGdvSUwyMHZNR3QwTlRFU0FtVnVLQUFQAQ", `second last`, `last`, entry},       // topics: Health

		{Basic, cnn, "https://edition.cnn.com/world", `/world`, `/africa`, content1},
		{Basic, cnn, "https://edition.cnn.com/americas", `/americas`, `/asia`, content1},
		{Plus, cnn, "https://edition.cnn.com/australia", `/australia`, `/china`, content1},
		{Premium, cnn, "https://edition.cnn.com/europe", `/europe`, `/india`, content1},
		{Premium, cnn, "https://edition.cnn.com/middle-east", `/middle-east`, `/uk`, content1},

		{Basic, espn, "https://www.espn.com/nfl/", `/nfl/scoreboard`, `/nfl/schedule`, entry},
		{Basic, espn, "https://www.espn.com/nba/", `/nba/scoreboard`, `/nba/schedule`, entry},
		{Plus, espn, "https://www.espn.com/mens-college-basketball/", `/mens-college-basketball/scoreboard`, `/mens-college-basketball/schedule`, entry},
		{Premium, espn, "https://www.espn.com/tennis/", `/tennis/dailyResults`, `/tennis/schedule`, entry},
		{Premium, espn, "https://www.espn.com/soccer/", `/soccer/scoreboard`, `/soccer/schedule`, entry},

		{Plus, hulu, "https://www.hulu.com/hub/movies", `/hub/movies`, `/hub/originals`, content1},

		{Plus, pinterest, "https://www.pinterest.com/ideas/", `/ideas/`, `/ideas/holidays/910319220330/`, content1},

		{Premium, youtube, "https://www.youtube.com", `/`, `/feed/trending`, content1},

		{Premium, netflix, "https://www.netflix.com", `netflix.com`, `help.netflix.com/legal/termsofuse`, content1},
	}

	winNum := 1
	tabNum := 0
	idx := 0

	switch caseLevel {
	default:
	case Basic:
		winNum = 2
		tabNum = 5
	case Plus:
		winNum = 4
		tabNum = 6
	case Premium:
		fallthrough
	case Record:
		winNum = 4
		tabNum = 9
	}

	windows := make([]chromeWindow, winNum)
	for i := range windows {
		window := &windows[i]
		window.tabs = make([]chromeTab, tabNum)
		for j := range window.tabs {
			tab := &window.tabs[j]
			for {
				if idx >= len(allLinks) {
					break
				}
				if allLinks[idx].level <= caseLevel {
					tab.conn = nil
					tab.link = allLinks[idx]
					idx++
					break
				}
				idx++
			}
		}
	}

	return windows
}

// Run2 runs the TabSwitchCUJ test. It is invoked by TabSwitchCujRecorder2 to
// record web contents via WPR and invoked by TabSwitchCUJ2 to exercise the tests
// from the recorded contents. Additional actions will be executed in each tab.
func Run2(ctx context.Context, s *testing.State, cr *chrome.Chrome, caseLevel Level, isTablet bool) {
	var (
		tabSwitchTimeout   = 30 * time.Second
		clickLinkTimeout   = 1 * time.Second
		pageLoadingTimeout = 1 * time.Minute
	)

	// In record mode, wait more time to ensure web content is fully recorded
	if caseLevel == Record {
		pageLoadingTimeout = 5 * time.Minute
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API, error: ", err)
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	if _, ok := s.Var("mute"); ok {
		if err := crastestclient.Mute(ctx); err != nil {
			s.Fatal("Failed to mute: ", err)
		}
		defer crastestclient.Unmute(closeCtx)
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a recorder, error: ", err)
	}
	defer recorder.Close(closeCtx)

	cleanup, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		s.Fatal("Failed to set battery discharge, error: ", err)
	}
	defer cleanup(closeCtx)

	windows := getTargets(caseLevel)

	mode := clamshell
	if isTablet {
		mode = tablet
	}

	var tsAction tabSwitchAction
	switch mode {
	case clamshell:
		pad, err := input.VirtualTrackpad(ctx)
		if err != nil {
			s.Fatal("Failed to create trackpad event writer")
		}
		defer pad.Close()

		touchPad, err := pad.NewMultiTouchWriter(2)
		if err != nil {
			s.Fatal("Failed to create trackpad singletouch writer")
		}
		defer touchPad.Close()

		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to open the keyboard, error: ", err)
		}
		defer kb.Close()

		// The first hit of "Alt+Tab" might not work,
		// so hit "Ctrl+Tab" before any other keyboard event.
		if err := kb.Accel(ctx, "Ctrl+Tab"); err != nil {
			s.Fatal("Failed to do keyboard action")
		}

		tsAction = newClamshellActionHandler(tconn, kb, pad, touchPad)
	case tablet:
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
		if err != nil {
			s.Fatal("Failed to EnsureTabletModeEnabled")
		}
		defer cleanup(ctx)

		touchScreen, err := input.Touchscreen(ctx)
		if err != nil {
			s.Fatal("Failed to create touchscreen event writer")
		}
		defer touchScreen.Close()

		tc, err := pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create the touch controller")
		}
		defer tc.Close()

		tsAction = newTabletActionHandler(tconn, tc)
	default:
		s.Fatal("Invalid mode")
	}

	extraActions := tsAction.tabExtraActions(ctx)

	var (
		chromeLaunchedTime                         time.Time
		browserLaunchElapsed, allTabsOpenedElapsed time.Duration
	)

	if passed := s.Run(ctx, "tab switch action", func(ctx context.Context, s *testing.State) {
		// Open all windows and tabs.
		for idxWindow := range windows {
			window := &windows[idxWindow]
			for idxTab := range window.tabs {
				var (
					tab = &window.tabs[idxTab]
					url = tab.link.url
					c   *chrome.Conn
				)

				if idxWindow == 0 && idxTab == 0 {
					// Launch browser and track the elapsed time.
					launchStart := time.Now()
					if chromeLaunchedTime, err = tsAction.launchChrome(ctx); err != nil {
						s.Fatal("Failed to launch Chrome")
					}
					browserLaunchElapsed = time.Since(launchStart)
					s.Log("Browser start ms: ", browserLaunchElapsed.Milliseconds())

					if err = tsAction.init(ctx); err != nil {
						s.Fatal("Failed to initialize tab switch action")
					}

					if c, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/")); err != nil {
						s.Fatal("Failed to find new tab: ", err)
					}
					if err = c.Navigate(ctx, url); err != nil {
						s.Fatalf("Failed to navigate to %s, error: %+v", url, err)
					}
				} else {
					if c, err = tsAction.newTab(ctx, cr, url, idxTab == 0); err != nil {
						s.Fatal("Failed to create new Chrome tab: ", err)
					}
				}

				// Wait for loading only on Record mode to properly record web content,
				// in replay mode, only have to wait before content clicking.
				if caseLevel == Record {
					if err := webutil.WaitForRender(ctx, c, pageLoadingTimeout); err != nil {
						s.Fatal("Failed to wait for finish render: ", err)
					}
					if err := webutil.WaitForQuiescence(ctx, c, pageLoadingTimeout); err != nil {
						s.Fatal("Failed to wait for tab quiescence: ", err)
					}
				}

				defer func() {
					if err := c.CloseTarget(ctx); err != nil {
						s.Error("Failed to close target, error: ", err)
					}
					if err := c.Close(); err != nil {
						s.Error("Failed to close the connection, error: ", err)
					}
				}()
				tab.conn = c
			}
		}
		// Total time used from beginning to load all pages.
		allTabsOpenedElapsed = time.Since(chromeLaunchedTime)
		s.Log("All tabs opened Elapsed: ", allTabsOpenedElapsed)

		// Switch between windows and tabs.
		if err = recorder.Run(ctx, func(ctx context.Context) error {
			for idx := range windows {
				var (
					window = &windows[idx]
					tabIdx = len(window.tabs) - 1
				)

				s.Log("Switching window")
				if err := tsAction.switchWindow(ctx, idx, len(windows)); err != nil {
					s.Fatal("Failed to switch window: ", err)
				}

				tabTotalNum := len(window.tabs)
				tabIdxPre := tabTotalNum - 1
				for i := 0; i < len(window.tabs)*3+1; i++ {
					s.Log("Switching tab")
					tabIdx = (tabIdx + 1) % len(window.tabs)
					if err := tsAction.switchTab(ctx, tabIdxPre, tabIdx, len(window.tabs)); err != nil {
						s.Fatal("Failed to switch tab: ", err)
					}
					tabIdxPre = tabIdx
					tab := &window.tabs[tabIdx]

					timeStart := time.Now()
					if err = webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
						s.Fatal("Failed to wait for the tab to be visible: ", err)
					}
					renderTime := time.Now().Sub(timeStart)
					s.Logf("Tab rendering time: %s", renderTime)
					if caseLevel == Record {
						if err := webutil.WaitForQuiescence(ctx, tab.conn, pageLoadingTimeout); err != nil {
							s.Fatal("Failed to wait for tab quiescence: ", err)
						}
						quiescenceTime := time.Now().Sub(timeStart)
						s.Logf("Tab rendering time: %s", quiescenceTime)
					}

					// Do actions only on these specific website.
					if tab.link.webName == wikipedia || tab.link.webName == hulu || tab.link.webName == youtube {
						for _, act := range extraActions {
							if err = act(ctx); err != nil {
								s.Fatal("Failed to execute action: ", err)
							}
							// Make sure the whole web content is recorded only under Recording.
							if caseLevel == Record {
								if err := webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
									s.Fatal("Failed to wait for finish render: ", err)
								}
								if err := webutil.WaitForQuiescence(ctx, tab.conn, pageLoadingTimeout); err != nil {
									s.Fatal("Failed to wait for tab quiescence: ", err)
								}
							}
						}
					}

					// Click on 1 link per 2 tabs, or click on 1 link for every tab under Record mode to ensure all links are
					// accessible under any other levels.
					if tabIdx%2 == 0 || caseLevel == Record {
						var pattern string
						var indicator urlIndicator
						switch tab.link.indicator {
						case entry:
							fallthrough
						case content1:
							pattern = tab.link.contentPattern2
							indicator = content2
						case content2:
							pattern = tab.link.contentPattern1
							indicator = content1
						}

						ts := time.Now()
						if err := testing.Poll(ctx, func(ctx context.Context) error {
							if e := clickAnchor(ctx, tab.conn, tab.link.webName, pattern); e != nil {
								te := time.Now().Sub(ts)
								s.Logf("Click anchor failed, retry, elapsed: %s", te)
							}
							return nil
						}, &testing.PollOptions{Timeout: pageLoadingTimeout, Interval: time.Second}); err != nil {
							s.Errorf("Failed to click anchor, current page: %s, error: %s", tab.link.url, err.Error())
						} else {
							tab.link.indicator = indicator
						}

						if caseLevel == Record {
							// the content of google news web site is not a common link,
							// the URL on navigation bar changed on click,
							// needs to refresh to properly record web content of google news.
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

					// Update current url.
					var href string
					if err := tab.conn.Call(ctx, &href, `() => { return window.location.href; }`); err != nil {
						s.Error("Error on getting current url: ", err)
					} else {
						s.Logf("Current URL: %s", href)
						tab.link.url = href
					}
				}
			}
			return nil
		}); err != nil {
			s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", err)
		}

		// Wait for the last tab to finish loading to stabilize last clicking action.
		lastWindow := windows[len(windows)-1]
		lastTab := lastWindow.tabs[len(lastWindow.tabs)-1]
		if err := webutil.WaitForQuiescence(ctx, lastTab.conn, pageLoadingTimeout); err != nil {
			s.Error("Failed to wait for tab quiescence: ", err)
		}
	}); !passed {
		s.Fatal("Failed to complete tab switch actions")
	}

	pv := perf.NewValues()

	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserLaunchElapsed.Milliseconds()))

	pv.Set(perf.Metric{
		Name:      "TabSwitchCUJ.ElapsedTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(allTabsOpenedElapsed.Milliseconds()))

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

func clickAnchor(ctx context.Context, c *chrome.Conn, wt webType, pattern string) error {
	var script string
	if wt == googleNews {
		// Customization made for Google News - the content is not a common link, can't match them by href.
		pos := 1 // the last one
		if pattern == `second last` {
			pos = 2 // the second lase one
		}
		script = fmt.Sprintf(`() => {
			var size = document.getElementsByClassName("ThdJC kaAt2 GFO5Jd").length;
			if ( size >= 2 ) {
				setTimeout(function(){ document.getElementsByClassName("ThdJC kaAt2 GFO5Jd")[size-%d].click(); }, 300);
				return true;
			}
			return false;
		}`, pos)
	} else {
		// Some link is with parameter or token, therefore, we can only find the element by
		// match with pattern (CSS selector, not regular expression).
		script = `(pattern) => {
			var name = "a[href*='" + pattern + "']";
			var els = document.querySelectorAll(name);
			if ( els.length > 0 ) {
				// Allow the CDP to be returned before going to new page.
				setTimeout(function(){ els[0].click(); }, 300);
				return true;
			}
			return false;
		}`
	}

	var done bool
	if err := c.Call(ctx, &done, script, pattern); err != nil {
		return err
	}
	// There's a timeout in above script to let js resources to be released before
	// going to new page. Here wait a short time too to ensure click is triggered.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep to wait for click on link")
	}

	if !done {
		return errors.Errorf("failed to click HTML element with pattern [%v]", pattern)
	}

	return nil
}

// init executes some actions to ensure the follow action execute properly as initiate
// process (no resource allocation involve).
func (t tabletActionHandler) init(ctx context.Context) error {
	// Tablet orientation is portrait by default, but dut is display as landscape,
	// which will cause swipe-down become swipe-right.
	// Therefore, set the proper orientation here to ensure the swipe direction is correct.
	orientation, err := display.GetOrientation(ctx, t.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the display rotation")
	}
	if err = t.touchScreen.SetRotation(-orientation.Angle); err != nil {
		return errors.Wrap(err, "failed to set rotation")
	}

	return nil
}

// launchChrome launches the Chrome browser.
func (t tabletActionHandler) launchChrome(ctx context.Context) (time.Time, error) {
	return cuj.LaunchAppFromHotseat(ctx, t.tconn, "Google Chrome")
}

// showTabList shows the tab list by click a button on the Chrome tool bar.
func (t tabletActionHandler) showTabList(ctx context.Context) error {
	// Swipe down a little bit to ensure Chrome's tool bar is shown.
	x := t.touchScreen.Width() / 2
	y0 := t.touchScreen.Height() / 2
	y1 := y0 + t.touchScreen.Height()/10
	if err := t.stw.Swipe(ctx, x, y0, x, y1, 500*time.Millisecond); err != nil {
		return errors.Wrap(err, "failed to Swipe down")
	}
	if err := t.stw.End(); err != nil {
		return errors.Wrap(err, "failed to end a touch")
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
	if err := ash.SwipeUpHotseatAndWaitForCompletion(ctx, t.tconn, t.stw, t.tcc); err != nil {
		return errors.Wrap(err, "failed to show hotseat")
	}
	if err := ui.WaitForLocationChangeCompleted(ctx, t.tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location changes")
	}

	// Click Google Chrome on hotseat.
	params := ui.FindParams{Name: "Google Chrome", ClassName: "ash/ShelfAppButton"}
	uiPollOpt := testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second}
	if err := t.clickHandler.StableFindAndClick(ctx, t.tconn, params, &uiPollOpt); err != nil {
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
	if err := ui.WaitForLocationChangeCompleted(ctx, t.tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location changes")
	}

	// Target tab item could be out of screen.
	// Scroll the tab list to ensure the item is visible.
	testing.ContextLog(ctx, "Scrolling the tab list before tab switch")
	var (
		root, tabListContainer           *ui.Node
		tabListContainers, tabs, tabsNew ui.NodeSlice
		err                              error
	)

	if root, err = ui.Root(ctx, t.tconn); err != nil {
		return errors.Wrap(err, "failed to find ui root")
	}
	defer root.Release(ctx)

	// Find all tab list.
	if tabListContainers, err = ui.FindAll(ctx, t.tconn, ui.FindParams{Role: ui.RoleTypeTabList}); err != nil {
		return errors.Wrap(err, "failed to find tab list container")
	}
	defer tabListContainers.Release(ctx)

	// Looking for the node located at top of the screen and the width is same as ui root.
	for _, tl := range tabListContainers {
		if tl.Location.Top == 0 && tl.Location.Left == 0 && tl.Location.Width == root.Location.Width {
			tabListContainer = tl
			break
		}
	}
	if tabListContainer == nil {
		return errors.Wrap(err, "failed to find tab list container")
	}

	// Find tab items under tabListContainer.
	if tabs, err = tabListContainer.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeTab}); err != nil {
		return errors.Wrap(err, "failed to find current focused tab item")
	}
	defer tabs.Release(ctx)
	if len(tabs) < tabTotalNum {
		return errors.New("error on finding tab items")
	}

	var (
		swipeDistance input.TouchCoord
		swipeVector   input.TouchCoord = 1 // the direction of swipe (horizontal, +x or -x)
	)

	// Find two adjacency item and which are both fully in-screen (in order to get correct width)
	// to calculate the swipe distance.
	succ := false
	for i := 0; i < len(tabs)-1; i++ {
		item1 := tabs[i]
		item2 := tabs[i+1]
		state1 := item1.State[ui.StateTypeOffscreen]
		state2 := item2.State[ui.StateTypeOffscreen]
		if (!state1 && !state2) && (item1.Location.Width == item2.Location.Width) {
			x0, _ := t.tcc.ConvertLocation(tabs[i].Location.CenterPoint())
			x1, _ := t.tcc.ConvertLocation(tabs[i+1].Location.CenterPoint())
			swipeDistance = x1 - x0
			succ = true
			break
		}
	}
	if !succ {
		return errors.Wrap(err, "failed to find two adjacency tab item within screen")
	}

	if tabIdxDest < tabIdxSrc {
		swipeVector = input.TouchCoord(-1)
	}

	var (
		swipeTimes = int(math.Abs(float64(tabIdxDest - tabIdxSrc)))
		ptSrc      = tabListContainer.Location.CenterPoint()
		x0, y0     = t.tcc.ConvertLocation(ptSrc)
		x1, y1     = x0 + swipeDistance*swipeVector, y0
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

	if err := ui.WaitForLocationChangeCompleted(ctx, t.tconn); err != nil {
		return errors.Wrap(err, "failed to wait for location changes")
	}
	testing.ContextLog(ctx, "Scroll complete, ready for tab switch")

	// Find tab items again since the position is changed after scroll.
	if tabsNew, err = tabListContainer.Descendants(ctx, ui.FindParams{Role: ui.RoleTypeTab}); err != nil {
		return errors.Wrap(err, "failed to find current focused tab item")
	}
	defer tabsNew.Release(ctx)
	if len(tabsNew) < tabTotalNum {
		return errors.New("error on finding tab items")
	}

	return t.clickHandler.LeftClick(ctx, tabsNew[tabIdxDest])
}

// pageRefresh refresh a web page (current focus page).
func (t tabletActionHandler) pageRefresh(ctx context.Context) error {
	params := ui.FindParams{Name: "Reload", Role: ui.RoleTypeButton, ClassName: "ReloadButton"}
	return t.clickHandler.StableFindAndClick(ctx, t.tconn, params, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second})
}

// tabExtraActions generate the extra action other than tab switch such as swipe.
func (t tabletActionHandler) tabExtraActions(ctx context.Context) []func(ctx context.Context) error {
	var (
		x      = t.touchScreen.Width() / 2
		ystart = t.touchScreen.Height() / 4 * 3 // 75% of screen height
		yend   = t.touchScreen.Height() / 4     // 25% of screen height
	)

	// Swipe the page down.
	swipeDown := func(ctx context.Context) error {
		if err := t.stw.Swipe(ctx, x, ystart, x, yend, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		if err := t.stw.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	// Swipe the page up.
	swipeUp := func(ctx context.Context) error {
		if err := t.stw.Swipe(ctx, x, yend, x, ystart, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to Swipe down")
		}
		if err := t.stw.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	return []func(ctx context.Context) error{
		swipeDown,
		swipeUp,
		swipeUp,
	}
}

// init executes some actions to ensure the follow action execute properly as initiate process
// (no resource allocation involve).
func (cl clamshellActionHandler) init(ctx context.Context) error {
	displayInfo, err := display.GetInternalInfo(ctx, cl.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get internal display info")
	}
	// Move the mouse cursor to the center so the scrolling will be effected on the web page.
	if err = mouse.Move(ctx, cl.tconn, displayInfo.Bounds.CenterPoint(), time.Second); err != nil {
		return errors.Wrap(err, "failed to move the mouse cursor to the center")
	}

	return nil
}

// launchChrome launches the Chrome browser.
func (cl clamshellActionHandler) launchChrome(ctx context.Context) (time.Time, error) {
	return cuj.LaunchAppFromShelf(ctx, cl.tconn, "Google Chrome")
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

// tabExtraActions generate the extra action other than tab switch such as swipe.
func (cl clamshellActionHandler) tabExtraActions(ctx context.Context) []func(ctx context.Context) error {
	var (
		x      = cl.pad.Width() / 2
		ystart = cl.pad.Height() / 4
		yend   = cl.pad.Height() / 4 * 3
	)

	// Swipe the page down.
	doubleSwipeDown := func(ctx context.Context) error {
		if err := cl.touchPad.DoubleSwipe(ctx, x, ystart, x, yend, 8, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe down")
		}
		if err := cl.touchPad.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	// Swipe the page up.
	doubleSwipeUp := func(ctx context.Context) error {
		if err := cl.touchPad.DoubleSwipe(ctx, x, yend, x, ystart, 8, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to DoubleSwipe up")
		}
		if err := cl.touchPad.End(); err != nil {
			return errors.Wrap(err, "failed to end a touch")
		}
		return nil
	}

	return []func(ctx context.Context) error{
		doubleSwipeDown,
		doubleSwipeUp,
		doubleSwipeUp,
	}
}
