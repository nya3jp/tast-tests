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
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

// to make this test case simpler, all links are fixed
const (
	link1Wiki      = `https://en.wikipedia.org/wiki/COVID-19_pandemic`
	link2Wiki      = `https://en.wikipedia.org/wiki/Coronavirus_disease_2019`
	link1Reddit    = `https://www.reddit.com/`
	link2Reddit    = `https://www.reddit.com/r/politics/comments/l2ao8s/right_on_schedule_republicans_pretend_to_care/`
	link1Medium    = `https://medium.com/topic/economy`
	link2Medium    = `https://medium.com/topic/business`
	link1GooNews   = `https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGRqTVhZU0FtVnVHZ0pWVXlnQVAB`
	link2GooNews   = `https://news.google.com/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRGx1YlY4U0FtVnVHZ0pWVXlnQVAB`
	link1CNN       = `https://edition.cnn.com/asia`
	link2CNN       = `https://edition.cnn.com/americas`
	link1ESPN      = `https://www.espn.com/nba/schedule`
	link2ESPN      = `https://www.espn.com/nba/standings`
	link1Hulu      = `https://www.hulu.com/hub/movies`
	link2Hulu      = `https://www.hulu.com/hub/originals`
	link1Pinterest = `https://www.pinterest.com/ideas/small-studio-apartment/938830059670/`
	link2Pinterest = `https://www.pinterest.com/ideas/world-cuisine/936568516643/`
	link1Youtube   = `https://www.youtube.com`
	link2Youtube   = `https://www.youtube.com/feed/trending`
	link1Netflix   = `https://www.netflix.com`
	link2Netflix   = `https://help.netflix.com`
)

type websiteType struct {
	name  string // the name of this website
	link1 string // one of the link to click while browsering this website
	link2 string // one of the link to click while browsering this website
}

// Level indicate how intensive of this test case is going to execute
type Level uint8

// Level indicate how intensive of this test case is going to execute
//
//  Basic is the level to use to run this case in basic level
//  Plus is the level to use to run this case in plus level
//  Premium is the level to use to run this case in basic level
//  Record is the level to use to run this case in *record mode*
const (
	basic Level = 1 << iota
	plus
	premium
	record

	Basic   = basic
	Plus    = Basic | plus
	Premium = Plus | premium
	Record  = record
)

// TestOption indicate the optional parameter of tabswitchcuj test case
//   TestLevel indicate how intensive of this test is going to execute
//   TabActions sets the extra action between switching tabs
type TestOption struct {
	TestLevel  Level
	TabActions []func(context.Context) error
}

type urlLink struct {
	level Level       // the corredponding level of this link
	wtype websiteType // the type of this web site
	url   string      // the url of this web site
}

type chromeTab struct {
	conn *chrome.Conn
	link urlLink
}

type chromeWindow struct {
	tabs []chromeTab
}

var (
	wikipedia  = websiteType{name: "Wikipedia", link1: link1Wiki, link2: link2Wiki}
	reddit     = websiteType{name: "Reddit", link1: link1Reddit, link2: link2Reddit}
	medium     = websiteType{name: "Medium", link1: link1Medium, link2: link2Medium}
	googlenews = websiteType{name: "GoogleNews", link1: link1GooNews, link2: link2GooNews}
	cnn        = websiteType{name: "CNN", link1: link1CNN, link2: link2CNN}
	espn       = websiteType{name: "ESPN", link1: link1ESPN, link2: link2ESPN}
	hulu       = websiteType{name: "Hulu", link1: link1Hulu, link2: link2Hulu}
	pinterest  = websiteType{name: "Pinterest", link1: link1Pinterest, link2: link2Pinterest}
	youtube    = websiteType{name: "Youtube", link1: link1Youtube, link2: link2Youtube}
	netflix    = websiteType{name: "Netflix", link1: link1Netflix, link2: link2Netflix}
)

var allLinks = [...]urlLink{
	{basic, wikipedia, link1Wiki},
	{basic, wikipedia, link1Wiki},
	{basic, wikipedia, link1Wiki},
	{plus, wikipedia, link1Wiki},
	{plus, wikipedia, link1Wiki},
	{premium, wikipedia, link1Wiki},
	{record, wikipedia, link1Wiki},

	{basic, reddit, link1Reddit},
	{basic, reddit, link1Reddit},
	{basic, reddit, link1Reddit},
	{plus, reddit, link1Reddit},
	{plus, reddit, link1Reddit},
	{premium, reddit, link1Reddit},
	{record, reddit, link1Reddit},

	{basic, medium, link1Medium},
	{basic, medium, link1Medium},
	{plus, medium, link1Medium},
	{premium, medium, link1Medium},
	{premium, medium, link1Medium},
	{record, medium, link1Medium},

	{basic, googlenews, link1GooNews},
	{basic, googlenews, link1GooNews},
	{plus, googlenews, link1GooNews},
	{premium, googlenews, link1GooNews},
	{premium, googlenews, link1GooNews},
	{record, googlenews, link1GooNews},

	{basic, cnn, link1CNN},
	{basic, cnn, link1CNN},
	{plus, cnn, link1CNN},
	{premium, cnn, link1CNN},
	{premium, cnn, link1CNN},
	{record, cnn, link1CNN},

	{basic, espn, link1ESPN},
	{basic, espn, link1ESPN},
	{plus, espn, link1ESPN},
	{premium, espn, link1ESPN},
	{premium, espn, link1ESPN},
	{record, espn, link1ESPN},

	{plus, hulu, link1Hulu},
	{record, hulu, link1Hulu},

	{plus, pinterest, link1Pinterest},
	{record, pinterest, link1Pinterest},

	{premium, youtube, link1Youtube},
	{record, youtube, link1Youtube},

	{premium, netflix, link1Netflix},
	{record, netflix, link1Netflix},
}

// getTargets sets of all web targets up according to input Level
func getTargets(level Level) []chromeWindow {
	var (
		winNum = 1
		tabNum = 0
		idx    = 0
	)

	switch level {
	default:
	case Basic:
		winNum = 2
		tabNum = 5
	case Plus:
		winNum = 4
		tabNum = 6
	case Premium:
		winNum = 4
		tabNum = 9
	case Record:
		winNum = 1
		for _, l := range allLinks {
			if l.level == record {
				tabNum++
			}
		}
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
				if (allLinks[idx].level & level) != 0 {
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
func Run2(ctx context.Context, s *testing.State, cr *chrome.Chrome, opt ...TestOption) {
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

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard, error: ", err)
	}
	defer kw.Close()

	// The custom variable for the developer to mute the device before the test,
	// so it doesn't make any noise when some of the visited pages play video.
	if _, ok := s.Var("mute"); ok {
		topRow, err := input.KeyboardTopRowLayout(ctx, kw)
		if err != nil {
			s.Fatal("Failed to obtain the top-row layout, error: ", err)
		}
		if err = kw.Accel(ctx, topRow.VolumeMute); err != nil {
			s.Fatal("Failed to mute, error: ", err)
		}
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

	// The first hit of "Alt+Tab" might not work, so hit "Ctrl+Tab" before running the case
	if err := kw.Accel(ctx, "Ctrl+Tab"); err != nil {
		s.Fatal("Failed to do keyboard action, error: ", err)
	}

	level := Basic
	if len(opt) > 0 {
		level = opt[0].TestLevel
	}
	var windows = getTargets(level)

	if passed := s.Run(ctx, "tab switch action", func(ctx context.Context, s *testing.State) {
		// open all windows and tabs
		for idx := range windows {
			window := &windows[idx]
			for i := range window.tabs {
				var (
					tab = &window.tabs[i]
					url = tab.link.url
					c   *chrome.Conn
				)

				if i == 0 {
					if c, err = cr.NewConn(ctx, url, cdputil.WithNewWindow()); err != nil {
						s.Fatal("Failed to create new Chrome window: ", err)
					}
				} else {
					// open tab by hit Ctrl+T
					if err = kw.Accel(ctx, "Ctrl+T"); err != nil {
						s.Fatal("Failed to hit Ctrl-T: ", err)
					}
					if c, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/")); err != nil {
						s.Fatal("Failed to find new tab: ", err)
					}
					if err = c.Navigate(ctx, url); err != nil {
						s.Fatalf("Failed to navigate to %s, error: %+v", url, err)
					}
					if level == Record {
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

				// switch window
				s.Log("Executing action: switch window")
				if err := kw.AccelPress(ctx, "Alt"); err != nil {
					s.Fatal("Failed to execute key event: ", err)
				}
				for i := 1; i < len(windows); i++ {
					if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
						s.Fatal("Failed to sleep: ", err)
					}
					if err := kw.AccelPress(ctx, "Tab"); err != nil {
						s.Fatal("Failed to execute key event: ", err)
					}
					if err := testing.Sleep(ctx, 200*time.Millisecond); err != nil {
						s.Fatal("Failed to sleep: ", err)
					}
					if err := kw.AccelRelease(ctx, "Tab"); err != nil {
						s.Fatal("Failed to execute key event: ", err)
					}
				}
				if err := kw.AccelRelease(ctx, "Alt"); err != nil {
					s.Fatal("Failed to execute key event: ", err)
				}

				for i := 0; i < len(window.tabs)*3+1; i++ {
					// switch tab by hit Ctrl+Tab
					s.Log("Executing action: switch tab")
					if err = kw.Accel(ctx, "Ctrl+Tab"); err != nil {
						return errors.Wrap(err, "failed to hit ctrl-tab")
					}
					tabIdx = (tabIdx + 1) % len(window.tabs)
					tab := &window.tabs[tabIdx]

					if err = webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
						s.Fatal("Failed to wait for the tab to be visible: ", err)
					}
					if level == Record {
						if err := webutil.WaitForQuiescence(ctx, tab.conn, recordModeLoadingTimeout); err != nil {
							s.Fatal("Failed to wait for tab quiescence: ", err)
						}
					}

					// if the case option is provided
					if len(opt) > 0 {
						// do actions only on these specific website.
						if tab.link.wtype == wikipedia || tab.link.wtype == hulu || tab.link.wtype == youtube {
							for _, act := range opt[0].TabActions {
								if err = act(ctx); err != nil {
									s.Fatal("Failed to execute action: ", err)
								}
								// make sure the whole web content is recorded only under Recording
								if level == Record {
									if err := webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
										s.Fatal("Failed to wait for finish render: ", err)
									}
									if err := webutil.WaitForQuiescence(ctx, tab.conn, recordModeLoadingTimeout); err != nil {
										s.Fatal("Failed to wait for tab quiescence: ", err)
									}
								}
							}
						}
					}

					// click on 1 link per 2 tabs,
					// or click on 1 link for every tab under Record mode to ensure all links are accessible under other level.
					if tabIdx%2 == 0 || level == Record {
						var url string
						switch tab.link.url {
						case tab.link.wtype.link1:
							url = tab.link.wtype.link2
						case tab.link.wtype.link2:
							url = tab.link.wtype.link1
						}
						if err = tab.conn.Navigate(ctx, url); err != nil {
							s.Fatalf("Failed to navigate to %s", url)
						}
						tab.link.url = url
						if level == Record {
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
