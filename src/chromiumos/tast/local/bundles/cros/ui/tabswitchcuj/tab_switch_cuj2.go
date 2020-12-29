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
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// Level indicate how intensive of this test case is going to execute
type Level uint8

// Level indicate how intensive of this test case is going to execute
//
//  Basic is the level to use to run this case in basic level
//  Plus is the level to use to run this case in plus level
//  Premium is the level to use to run this case in basic level
//  Record is the level to use to run this case in *record mode*
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
	entryURL        string
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

// getTargets sets all web targets according to input Level
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

		{Basic, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGRqTVhZU0FtVnVHZ0pWVXlnQVAB", `second last`, `last`, entry},
		{Basic, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNREpxYW5RU0FtVnVHZ0pWVXlnQVAB", `second last`, `last`, entry},
		{Plus, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp1ZEdvU0FtVnVHZ0pWVXlnQVAB", `second last`, `last`, entry},
		{Premium, googleNews, "https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFp0Y1RjU0FtVnVHZ0pWVXlnQVAB", `second last`, `last`, entry},
		{Premium, googleNews, "https://news.google.com/topics/CAAqIQgKIhtDQkFTRGdvSUwyMHZNR3QwTlRFU0FtVnVLQUFQAQ", `second last`, `last`, entry},

		{Basic, cnn, "https://edition.cnn.com/world", `/world`, `/africa`, content1},
		{Basic, cnn, "https://edition.cnn.com/americas", `/americas`, `/asia`, content1},
		{Plus, cnn, "https://edition.cnn.com/australia", `/australia`, `/china`, content1},
		{Premium, cnn, "https://edition.cnn.com/europe", `/europe`, `/india`, content1},
		{Premium, cnn, "https://edition.cnn.com/middle-east", `/middle-east`, `/uk`, content1},

		{Basic, espn, "https://www.espn.com/nba/", `/nba/`, `/nba/scoreboard`, content1},
		{Basic, espn, "https://www.espn.com/nba/schedule", `/nba/schedule`, `/nba/standings`, content1},
		{Plus, espn, "https://www.espn.com/nfl/", `/nfl/`, `/nfl/draft/news`, content1},
		{Premium, espn, "https://www.espn.com/nfl/scoreboard", `/nfl/scoreboard`, `/nfl/schedule`, content1},
		{Premium, espn, "https://www.espn.com/soccer/", `/soccer/`, `/soccer/scoreboard`, content1},

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
	const (
		tabSwitchTimeout         = 20 * time.Second
		clickLinkTimeout         = 1 * time.Second
		recordModeLoadingTimeout = 5 * time.Minute // in record mode, wait more time to ensure web content is fully recorded
	)

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

	// traces to debug the large UMA value issues.
	defer cr.StopTracing(ctx)
	if err := cr.StartTracing(ctx, []string{"benchmark", "cc", "gpu", "input", "toplevel", "ui", "views", "viz"}); err != nil {
		s.Log("Failed to start tracing, error: ", err)
		return
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

	// resources for tablet or clamshell
	var (
		kb          *input.KeyboardEventWriter
		screen      *input.TouchscreenEventWriter
		pad         *input.TrackpadEventWriter
		touchScreen *input.SingleTouchEventWriter
		touchPad    *input.TouchEventWriter
	)

	// prepare resources for tablet or clamshell
	switch isTablet {
	case true:
		screen, err = input.Touchscreen(ctx)
		if err != nil {
			s.Fatal("Failed to create touchscreen event writer")
		}
		defer screen.Close()

		touchScreen, err = screen.NewSingleTouchWriter()
		if err != nil {
			s.Fatal("Failed to create touchscreen singletouch writer")
		}
		defer touchScreen.Close()
	case false:
		screen, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get internal display info")
		}
		// Move the mouse cursor to the center so the scrolling will be effected on the web page
		if err = mouse.Move(ctx, tconn, screen.Bounds.CenterPoint(), time.Second); err != nil {
			s.Fatal("Failed to move the mouse cursor to the center")
		}

		pad, err = input.VirtualTrackpad(ctx)
		if err != nil {
			s.Fatal("Failed to create trackpad event writer")
		}
		defer pad.Close()

		touchPad, err = pad.NewMultiTouchWriter(2)
		if err != nil {
			s.Fatal("Failed to create trackpad singletouch writer")
		}
		defer touchPad.Close()

		kb, err = input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to open the keyboard, error: ", err)
		}
		defer kb.Close()

		// The first hit of "Alt+Tab" might not work,
		// so hit "Ctrl+Tab" before any other keyboard event
		if err := kb.Accel(ctx, "Ctrl+Tab"); err != nil {
			s.Fatal("Failed to do keyboard action, error: ", err)
		}
	}

	extraActions := tabExtraActions(ctx, s, tconn, isTablet, screen, pad, touchScreen, touchPad)

	if passed := s.Run(ctx, "tab switch action", func(ctx context.Context, s *testing.State) {
		// open all windows and tabs
		for idx := range windows {
			window := &windows[idx]
			for i := range window.tabs {
				var (
					tab = &window.tabs[i]
					url = tab.link.entryURL
					c   *chrome.Conn
				)

				if i == 0 {
					if c, err = cr.NewConn(ctx, url, cdputil.WithNewWindow()); err != nil {
						s.Fatal("Failed to create new Chrome window: ", err)
					}
				} else {
					if err = createNewTab(ctx, isTablet, kb); err != nil {
						s.Fatal("Failed to open new tab: ", err)
					}
					if c, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/")); err != nil {
						s.Fatal("Failed to find new tab: ", err)
					}
					if err = c.Navigate(ctx, url); err != nil {
						s.Fatalf("Failed to navigate to %s, error: %+v", url, err)
					}
					if caseLevel == Record {
						if err := webutil.WaitForRender(ctx, c, recordModeLoadingTimeout); err != nil {
							s.Fatal("Failed to wait for finish render: ", err)
						}
						if err := webutil.WaitForQuiescence(ctx, c, recordModeLoadingTimeout); err != nil {
							s.Fatal("Failed to wait for tab quiescence: ", err)
						}
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
		// switch between windows and tabs
		if err = recorder.Run(ctx, func(ctx context.Context) error {
			for idx := range windows {
				var (
					window = &windows[idx]
					tabIdx = len(window.tabs) - 1
				)

				s.Log("Switching window")
				if err := switchWindow(ctx, len(windows), isTablet, kb); err != nil {
					s.Fatal("Failed to switch window: ", err)
				}

				for i := 0; i < len(window.tabs)*3+1; i++ {
					s.Log("Switching tab")
					if err := switchTab(ctx, isTablet, kb); err != nil {
						s.Fatal("Failed to switch tab: ", err)
					}
					tabIdx = (tabIdx + 1) % len(window.tabs)
					tab := &window.tabs[tabIdx]

					if err = webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
						s.Fatal("Failed to wait for the tab to be visible: ", err)
					}
					if caseLevel == Record {
						if err := webutil.WaitForQuiescence(ctx, tab.conn, recordModeLoadingTimeout); err != nil {
							s.Fatal("Failed to wait for tab quiescence: ", err)
						}
					}

					// do actions only on these specific website.
					if tab.link.webName == wikipedia || tab.link.webName == hulu || tab.link.webName == youtube {
						for _, act := range extraActions {
							if err = act(ctx); err != nil {
								s.Fatal("Failed to execute action: ", err)
							}
							// make sure the whole web content is recorded only under Recording
							if caseLevel == Record {
								if err := webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
									s.Fatal("Failed to wait for finish render: ", err)
								}
								if err := webutil.WaitForQuiescence(ctx, tab.conn, recordModeLoadingTimeout); err != nil {
									s.Fatal("Failed to wait for tab quiescence: ", err)
								}
							}
						}
					}

					// click on 1 link per 2 tabs,
					// or click on 1 link for every tab under Record mode to ensure all links are accessible under other level.
					if tabIdx%2 == 0 || caseLevel == Record {
						switch tab.link.indicator {
						case entry:
							fallthrough
						case content1:
							if err := clickAnchor(ctx, tab.conn, tab.link.webName, tab.link.contentPattern2); err != nil {
								s.Error("Faile to click anchor: ", err)
							} else {
								tab.link.indicator = content2
							}
						case content2:
							if err := clickAnchor(ctx, tab.conn, tab.link.webName, tab.link.contentPattern1); err != nil {
								s.Error("Faile to click anchor: ", err)
							} else {
								tab.link.indicator = content1
							}
						}
						if caseLevel == Record {
							if err := webutil.WaitForRender(ctx, tab.conn, recordModeLoadingTimeout); err != nil {
								s.Fatal("Failed to wait for finish render: ", err)
							}
							if err := webutil.WaitForQuiescence(ctx, tab.conn, recordModeLoadingTimeout); err != nil {
								s.Fatal("Failed to wait for tab quiescence: ", err)
							}
						} else {
							// it is normal that tabs might remain loading, hence no handle error here
							webutil.WaitForQuiescence(ctx, tab.conn, clickLinkTimeout)
						}
					}
				}
			}
			return nil
		}); err != nil {
			s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", err)
		}
	}); !passed {
		s.Fatal("Failed to complete tab switch actions")
	}

	pv := perf.NewValues()
	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report, error: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values, error: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Error("Failed to save histogram raw data: ", err)
	}

	tr, err := cr.StopTracing(ctx)
	if err != nil {
		s.Log("Failed to stop tracing, error: ", err)
		return
	}
	if tr == nil || len(tr.Packet) == 0 {
		s.Log("No trace data is collected")
		return
	}
	filename := fmt.Sprintf("trace.%s.data.gz", time.Now().Format("20060102-150405"))
	if err := chrome.SaveTraceToFile(ctx, tr, filepath.Join(s.OutDir(), filename)); err != nil {
		s.Error("Failed to save trace to file, error: ", err)
		return
	}
}

func clickAnchor(ctx context.Context, c *chrome.Conn, wt webType, pattern string) error {
	var script string
	if wt == googleNews {
		// custome made for Google News, click div
		pos := 1 // the last one
		if pattern == `second last` {
			pos = 2 // the second lase one
		}
		script = fmt.Sprintf(`() => {
			var size = document.getElementsByClassName("ThdJC kaAt2 GFO5Jd").length;
			if ( size >= 2 ) {
				document.getElementsByClassName("ThdJC kaAt2 GFO5Jd")[size-%d].click();
				return true;
			}
			return false;
		}`, pos)
	} else {
		// some link is with parameter or token, therefore,
		// can only find the element by match with pattern (CSS selector, not regular expression)
		script = `(pattern) => {
			var name = "a[href*='" + pattern + "']";
			var els = document.querySelectorAll(name);
			if ( els.length > 0 ) {
				setTimeout(function(){ els[0].click(); }, 200);
				return true;
			}
			return false;
		}`
	}

	var done bool
	if err := c.Call(ctx, &done, script, pattern); err != nil {
		return err
	}
	// there's a short stop to let js resources release
	// here wait short time too to ensure click is triggered
	testing.Sleep(ctx, 300*time.Millisecond)

	if !done {
		return errors.Errorf("failed to click HTML element with pattern [%v]", pattern)
	}

	return nil
}

func tabExtraActions(ctx context.Context, s *testing.State, tconn *chrome.TestConn, isTablet bool, screen *input.TouchscreenEventWriter, pad *input.TrackpadEventWriter, touchScreen *input.SingleTouchEventWriter, touchPad *input.TouchEventWriter) []func(ctx context.Context) error {
	if isTablet && screen != nil && touchScreen != nil {
		var (
			x      = screen.Width() / 2
			ystart = screen.Height() / 4 * 3 // 75% of screen height
			yend   = screen.Height() / 4     // 25% of screen height
		)

		swipeDown := func(ctx context.Context) error {
			if err := touchScreen.Swipe(ctx, x, ystart, x, yend, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to Swipe down")
			}
			if err := touchScreen.End(); err != nil {
				return errors.Wrap(err, "failed to end a touch")
			}
			return nil
		}

		return []func(ctx context.Context) error{
			swipeDown,
		}
	}

	if !isTablet && pad != nil && touchPad != nil {
		var (
			x      = pad.Width() / 2
			ystart = pad.Height() / 4
			yend   = pad.Height() / 4 * 3
		)

		// swipe the page down
		doubleSwipeDown := func(ctx context.Context) error {
			if err := touchPad.DoubleSwipe(ctx, x, ystart, x, yend, 8, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to DoubleSwipe down")
			}
			if err := touchPad.End(); err != nil {
				return errors.Wrap(err, "failed to end a touch")
			}
			return nil
		}

		// swipe the page up
		doubleSwipeUp := func(ctx context.Context) error {
			if err := touchPad.DoubleSwipe(ctx, x, yend, x, ystart, 8, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to DoubleSwipe up")
			}
			if err := touchPad.End(); err != nil {
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

	return nil
}

func createNewTab(ctx context.Context, isTablet bool, kw *input.KeyboardEventWriter) error {
	// TODO: add tablet support
	if isTablet {
		return nil
	}

	if !isTablet && kw != nil {
		if err := kw.Accel(ctx, "Ctrl+T"); err != nil {
			return errors.Wrap(err, "failed to hit Ctrl-T")
		}
	}

	return nil
}

func switchTab(ctx context.Context, isTablet bool, kw *input.KeyboardEventWriter) error {
	// TODO: add tablet support
	if isTablet {
		return nil
	}

	if !isTablet && kw != nil {
		if err := kw.Accel(ctx, "Ctrl+Tab"); err != nil {
			return errors.Wrap(err, "failed to hit ctrl-tab")
		}
	}

	return nil
}

func switchWindow(ctx context.Context, numWindows int, isTablet bool, kw *input.KeyboardEventWriter) error {
	// TODO: add tablet support
	if isTablet {
		return nil
	}

	if !isTablet && kw != nil {
		if err := kw.AccelPress(ctx, "Alt"); err != nil {
			return errors.Wrap(err, "failed to execute key event")
		}
		for i := 1; i < numWindows; i++ {
			if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			if err := kw.AccelPress(ctx, "Tab"); err != nil {
				return errors.Wrap(err, "failed to execute key event")
			}
			if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}
			if err := kw.AccelRelease(ctx, "Tab"); err != nil {
				return errors.Wrap(err, "failed to execute key event")
			}
		}
		if err := kw.AccelRelease(ctx, "Alt"); err != nil {
			return errors.Wrap(err, "failed to execute key event")
		}
	}

	return nil
}
