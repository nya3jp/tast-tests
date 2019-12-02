// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJ,
		Desc:         "Measures the performance of tab-switching CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      10 * time.Minute,
		Vars:         []string{"mute"},
	})
}

func getURLs(ctx context.Context, c *chrome.Conn, expr string, numPages int) ([]string, error) {
	urls := make([]string, 0, numPages)
	findURLsExpr := fmt.Sprintf(`(function() {
		const anchors = [...document.getElementsByTagName('A')];
		const founds = new Set();
		const results = [];
		for (let i = 0; i < anchors.length && results.length < %d; i++) {
			const href = anchors[i].href;
			if (founds.has(href)) {
				continue;
			}
			founds.add(href);
			try {
				let url = new URL(href);
				if ((%s)(url))
					results.push(href);
			} catch {
				// do nothing.
			}
		}
		return results;
	})()`, numPages, expr)
	if err := c.Eval(ctx, findURLsExpr, &urls); err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, errors.New("no urls found")
	}
	return urls, nil
}

func waitUntilAllTabsLoaded(ctx context.Context, c *chrome.Conn, timeout time.Duration) error {
	query := map[string]interface{}{
		"status":        "loading",
		"currentWindow": true,
	}
	queryData, err := json.Marshal(query)
	if err != nil {
		return errors.Wrap(err, "failed to marshal query")
	}
	expr := fmt.Sprintf(`tast.promisify(chrome.tabs.query)(%s)`, string(queryData))
	return testing.Poll(ctx, func(ctx context.Context) error {
		var tabs []map[string]interface{}
		if err := c.EvalPromise(ctx, expr, &tabs); err != nil {
			return testing.PollBreak(err)
		}
		if len(tabs) == 0 {
			return nil
		}
		return errors.Errorf("still %d tabs are loading", len(tabs))
	}, &testing.PollOptions{Timeout: timeout})
}

func waitForTabVisible(ctx context.Context, c *chrome.Conn, timeout time.Duration) error {
	const expr = `
	new Promise(function (resolve, reject) {
		// We wait for two calls to requestAnimationFrame. When the first
		// requestAnimationFrame is called, we know that a frame is in the
		// pipeline. When the second requestAnimationFrame is called, we know that
		// the first frame has reached the screen.
		let frameCount = 0;
		const waitForRaf = function() {
			frameCount++;
			if (frameCount === 2) {
				resolve();
			} else {
				window.requestAnimationFrame(waitForRaf);
			}
		};
		window.requestAnimationFrame(waitForRaf);
	})
	`

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.EvalPromise(waitCtx, expr, nil)
}

func TabSwitchCUJ(ctx context.Context, s *testing.State) {
	const numPages = 6
	cr := s.PreValue().(*chrome.Chrome)

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}

	// The custom variable for the deverloper to mute the device before the test,
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

	recorder, err := cuj.NewRecorder(cuj.MetricConfig{
		HistogramName: "MPArch.RWH_TabSwitchPaintDuration",
		Unit:          "ms",
		Category:      cuj.CategoryLatency,
		JankCriteria:  []int64{800, 1600},
	})

	for _, data := range []struct {
		startURL string
		findURLs string
	}{
		{
			"https://news.google.com/",
			`function(url) { return url.host === 'news.google.com' && url.pathname.indexOf('/articles/') == 0; }`,
		},
		{
			"https://cnn.com",
			`function(url) { return url.host === 'www.cnn.com' && url.pathname.match(new RegExp("^/\\d\\d\\d\\d/\\d\\d/\\d\\d/")); }`,
		},
	} {
		conns := chrome.Conns(make([]*chrome.Conn, 0, numPages+1))
		defer conns.Close()
		firstPage, err := cr.NewConn(ctx, data.startURL)
		if err != nil {
			s.Fatalf("Failed to open %s: %v", data.startURL, err)
		}
		conns = append(conns, firstPage)

		urls, err := getURLs(ctx, firstPage, data.findURLs, numPages)
		if err != nil {
			s.Fatalf("Failed to get URLs for %s: %v", data.startURL, err)
		}

		for _, u := range urls {
			c, err := cr.NewConn(ctx, u)
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

		if err = recorder.Run(ctx, cr, func() error {
			for i := 0; i < (numPages+1)*3+1; i++ {
				if err = kw.Accel(ctx, "Ctrl+Tab"); err != nil {
					return errors.Wrap(err, "failed to hit ctrl-tab")
				}
				currentTab = (currentTab + 1) % len(conns)
				if err = waitForTabVisible(ctx, conns[currentTab], tabSwitchTimeout); err != nil {
					s.Log("Failed to wait for the tab to be visible: ", err)
				}
			}
			return nil
		}); err != nil {
			s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", err)
		}
		for _, c := range conns {
			if err = c.CloseTarget(ctx); err != nil {
				s.Fatal("Failed to close target: ", err)
			}
		}
	}

	pv := perf.NewValues()
	if err = recorder.Record(pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
