// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

const (
	// WPRArchiveName is used as the external file name of the wpr archive for
	// TabSwitchCuj and as the output filename under "/tmp" for
	// TabSwitchCujRecorder.
	WPRArchiveName = "tab_switch_cuj.wprgo"
)

// TabSwitchParam holds parameters of tab switch cuj test variations.
type TabSwitchParam struct {
	BrowserType browser.Type // Chrome type.
	Tracing     bool         // Whether to turn on tracing.
	Validation  bool         // Whether to add extra cpu loads before collecting metrics.
}

// findAnchorURLs returns the unique URLs of the anchors, which matches the pattern.
// If it finds more than limit, returns the first limit elements.
func findAnchorURLs(ctx context.Context, c *chrome.Conn, pattern string, limit int) ([]string, error) {
	var urls []string
	if err := c.Call(ctx, &urls, `(pattern, limit) => {
		const anchors = [...document.getElementsByTagName('A')];
		const founds = new Set();
		const results = [];
		const regexp = new RegExp(pattern);
		for (let i = 0; i < anchors.length && results.length < limit; i++) {
		  const href = new URL(anchors[i].href).toString();
		  if (founds.has(href)) {
		    continue;
		  }
		  founds.add(href);
		  if (regexp.test(href)) {
		    results.push(href);
		  }
		}
		return results;
	}`, pattern, limit); err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, errors.New("no urls found")
	}
	return urls, nil
}

func waitUntilAllTabsLoaded(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	query := map[string]interface{}{
		"status":        "loading",
		"currentWindow": true,
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		var tabs []map[string]interface{}
		if err := tconn.Call(ctx, &tabs, `tast.promisify(chrome.tabs.query)`, query); err != nil {
			return testing.PollBreak(err)
		}
		if len(tabs) != 0 {
			return errors.Errorf("still %d tabs are loading", len(tabs))
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// Run runs the TabSwitchCUJ test. It is invoked by TabSwitchCujRecorder to
// record web contents via WPR and invoked by TabSwitchCUJ to exercise the tests
// from the recorded contents.
func Run(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome
	var cs ash.ConnSource
	var bBrowser *browser.Browser

	param := s.Param().(TabSwitchParam)
	if param.BrowserType == browser.TypeAsh {
		cr = s.PreValue().(*chrome.Chrome)
		cs = cr
		bBrowser = cr.Browser()
	} else {
		var l *lacros.Lacros
		var err error
		cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), param.BrowserType)
		if err != nil {
			s.Fatal("Failed to initialize test: ", err)
		}
		defer lacros.CloseLacros(ctx, l)
		bBrowser = l.Browser()
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kw.Close()

	// The custom variable for the developer to mute the device before the test,
	// so it doesn't make any noise when some of the visited pages play video.
	if _, ok := s.Var("mute"); ok {
		topRow, err := input.KeyboardTopRowLayout(ctx, kw)
		if err != nil {
			s.Fatal("Failed to obtain the top-row layout: ", err)
		}
		if err = kw.Accel(ctx, topRow.VolumeMute); err != nil {
			s.Fatal("Failed to mute: ", err)
		}
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}

	if err := recorder.AddCollectedMetrics(bBrowser, cujrecorder.DeprecatedMetricConfigs()...); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}

	if param.Tracing {
		recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))
	}
	defer recorder.Close(closeCtx)

	if param.Validation {
		validationHelper := cuj.NewTPSValidationHelper(closeCtx)
		if err := validationHelper.Stress(); err != nil {
			s.Fatal("Failed to stress: ", err)
		}
		defer func() {
			if err := validationHelper.Release(); err != nil {
				s.Fatal("Failed to release validationHelper: ", err)
			}
		}()
	}

	for _, data := range []struct {
		name       string
		startURL   string
		urlPattern string
	}{
		{
			"CNN",
			"https://cnn.com",
			`^.*://www.cnn.com/\d{4}/\d{2}/\d{2}/`,
		},
		{
			"Reddit",
			"https://reddit.com",
			`^.*://www.reddit.com/r/[^/]+/comments/[^/]+/`,
		},
	} {
		s.Run(ctx, data.name, func(ctx context.Context, s *testing.State) {
			const numPages = 6
			conns := make([]*chrome.Conn, 0, numPages+1)
			defer func() {
				for _, c := range conns {
					if err = c.CloseTarget(ctx); err != nil {
						s.Error("Failed to close target: ", err)
					}
					if err = c.Close(); err != nil {
						s.Error("Failed to close the connection: ", err)
					}
				}
			}()
			firstPage, err := cs.NewConn(ctx, data.startURL)
			if err != nil {
				s.Fatalf("Failed to open %s: %v", data.startURL, err)
			}
			conns = append(conns, firstPage)

			urls, err := findAnchorURLs(ctx, firstPage, data.urlPattern, numPages)
			if err != nil {
				s.Fatalf("Failed to get URLs for %s: %v", data.startURL, err)
			}

			for _, u := range urls {
				c, err := cs.NewConn(ctx, u)
				if err != nil {
					s.Fatalf("Failed to open the URL %s: %v", u, err)
				}
				conns = append(conns, c)
			}

			if err = waitUntilAllTabsLoaded(ctx, tconn, time.Minute); err != nil {
				s.Log("Some tabs are still in loading state, but proceed the test: ", err)
			}
			currentTab := len(conns) - 1
			const tabSwitchTimeout = 20 * time.Second

			if err = recorder.Run(ctx, func(ctx context.Context) error {
				for i := 0; i < (numPages+1)*3+1; i++ {
					if err = kw.Accel(ctx, "Ctrl+Tab"); err != nil {
						return errors.Wrap(err, "failed to hit ctrl-tab")
					}
					currentTab = (currentTab + 1) % len(conns)
					if err := webutil.WaitForRender(ctx, conns[currentTab], tabSwitchTimeout); err != nil {
						s.Log("Failed to wait for the tab to be visible: ", err)
					}
				}
				return nil
			}); err != nil {
				s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", err)
			}
		})
	}

	pv := perf.NewValues()
	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
