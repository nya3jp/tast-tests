// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabSwitchCUJ,
		Desc:         "Measures the performance of tab-switching CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      10 * time.Minute,
		Vars:         []string{"mute"},
	})
}

func TabSwitchCUJ(ctx context.Context, s *testing.State) {
	const numPages = 6
	cr := s.PreValue().(*chrome.Chrome)

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}

	// I got some pages which plays video automatically, which makes a bit noisy
	// when testing locally. When specified, just hit the mute button to stop
	// audio automatically.
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
	defer tconn.Close()

	metricConfigs := map[string]struct {
		jankBoundary int64
		metric       perf.Metric
	}{
		"MPArch.RWH_TabSwitchPaintDuration": {
			jankBoundary: 1000,
			metric: perf.Metric{
				Unit:      "ms",
				Direction: perf.SmallerIsBetter,
			},
		},
	}
	type record struct {
		counts     int64
		total      int64
		jankCounts float64
	}
	records := make(map[string]*record, len(metricConfigs))
	names := make([]string, 0, len(metricConfigs))
	for name := range metricConfigs {
		names = append(names, name)
		records[name] = &record{}
	}
	for _, data := range []struct {
		startURL string
		findURLs string
	}{
		{
			"https://news.google.com/",
			`function(a) {
				let url = new URL(a.href);
				return url.host === 'news.google.com' && url.pathname.indexOf('/articles/') == 0;
			}`,
		},
		{
			"https://cnn.com",
			`function(a) {
				let url = new URL(a.href);
				return url.host === 'www.cnn.com' && url.pathname.match(new RegExp("^/\\d\\d\\d\\d/\\d\\d/\\d\\d/"));
			}`,
		},
	} {
		conns := chrome.Conns(make([]*chrome.Conn, 0, numPages+1))
		firstPage, err := cr.NewConn(ctx, data.startURL, cdputil.WithNewWindow())
		if err != nil {
			s.Fatalf("Failed to open %s: %v", data.startURL, err)
		}
		conns = append(conns, firstPage)
		defer conns.Close()

		urls := make([]string, 0, numPages)
		findURLsExpr := fmt.Sprintf(`
			let anchors = [];
			anchors.push(...document.getElementsByTagName('A'));
			let founds = new Set();
			anchors.filter(a => {
				if (founds.has(a.href)) {
					return false;
				}
				founds.add(a.href);
				try {
					return (%s)(a);
				} catch {
					return false;
				}
			}).slice(0, %d).map(a => a.href);
		`, data.findURLs, numPages)
		if err := firstPage.Eval(ctx, findURLsExpr, &urls); err != nil {
			s.Fatal("Failed to obtain the URL list: ", err)
		}
		if len(urls) == 0 {
			s.Fatalf("No urls found for %s", data.startURL)
		}
		for _, u := range urls {
			c, err := cr.NewConn(ctx, u)
			if err != nil {
				s.Fatalf("Failed to open the URL %s: %v", u, err)
			}
			conns = append(conns, c)
		}

		if err = chrome.WaitUntilAllTabsLoaded(ctx, tconn, time.Minute); err != nil {
			s.Log("Some tabs are still in loading state, but proceed the test: ", err)
		}
		currentTab := len(conns) - 1
		waitForTabVisible := func(ctx context.Context, c *chrome.Conn) error {
			const expr = `
		new Promise(function (resolve, reject) {
			// We wait for two calls to requestAnimationFrame. When the first
			// requestAnimationFrame is called, we know that a frame is in the
			// pipeline. When the second requestAnimationFrame is called, we know that
			// the first frame has reached the screen.
			let frameCount = 0;
			const waitForRaf = function() {
				frameCount++;
				if (frameCount == 2) {
					resolve();
				} else {
					window.requestAnimationFrame(waitForRaf);
				}
			};
			window.requestAnimationFrame(waitForRaf);
		})
		`
			waitCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			return c.EvalPromise(waitCtx, expr, nil)
		}
		hists, err := metrics.Run(ctx, cr, func() error {
			for i := 0; i < (numPages+1)*3+1; i++ {
				if err = kw.Accel(ctx, "Ctrl+Tab"); err != nil {
					return errors.Wrap(err, "failed to hit ctrl-tab")
				}
				currentTab = (currentTab + 1) % len(conns)
				if err = waitForTabVisible(ctx, conns[currentTab]); err != nil {
					return errors.Wrap(err, "failed to wait for the tab to be visible")
				}
			}
			return nil
		}, names...)
		if err != nil {
			s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", err)
		}
		for _, hist := range hists {
			if hist.TotalCount() == 0 {
				s.Fatalf("Expected to have data %s but not found", hist.Name)
			}
			config := metricConfigs[hist.Name]
			record := records[hist.Name]
			record.counts += hist.TotalCount()
			record.total += int64(hist.Mean() * float64(hist.TotalCount()))
			// count janks.
			for _, bucket := range hist.Buckets {
				if config.metric.Direction == perf.BiggerIsBetter {
					if bucket.Max <= config.jankBoundary {
						record.jankCounts += float64(bucket.Count)
					} else if bucket.Min < config.jankBoundary {
						record.jankCounts += float64(bucket.Count) / 2
					}
				} else {
					if bucket.Min >= config.jankBoundary {
						record.jankCounts += float64(bucket.Count)
					} else if bucket.Max > config.jankBoundary {
						// simply estimate about the half hits the jank.
						// TODO(mukai): put a better estimation.
						record.jankCounts += float64(bucket.Count) / 2
					}
				}
			}
		}
		for _, c := range conns {
			if err = c.CloseTarget(ctx); err != nil {
				s.Fatal("Failed to close target: ", err)
			}
		}
	}

	pv := perf.NewValues()
	// Report the average of the all data points.
	for name, record := range records {
		metric := metricConfigs[name].metric
		metric.Name = name
		pv.Set(metric, float64(record.total)/float64(record.counts))
		pv.Set(perf.Metric{
			Name:      fmt.Sprintf("%s.jank_rate", name),
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, record.jankCounts/float64(record.counts))
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
