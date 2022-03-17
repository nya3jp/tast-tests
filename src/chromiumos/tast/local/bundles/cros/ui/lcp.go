// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LCP,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the LCP UMA",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{
			{
				Name: "reddit_load", // Load the page
			}, {
				Name: "reddit_load_and_click", // Load the page, and click a link in the page
			}, {
				Name: "reddit_load_and_navigate", // Load the page, and enter another URL in the address bar
			},
		},
	})
}

func LCP(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	name := s.TestName()
	s.Logf("Test name: %q", name)

	clickAnchor := false
	navigate := false
	if strings.HasSuffix(name, "reddit_load_and_click") {
		clickAnchor = true
	} else if strings.HasSuffix(name, "reddit_load_and_navigate") {
		navigate = true
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	metrics := []cuj.MetricConfig{
		cuj.NewCustomMetricConfig("PageLoad.PaintTiming.NavigationToLargestContentfulPaint2", "ms", perf.SmallerIsBetter, []int64{300, 2000}),
		cuj.NewCustomMetricConfig("PageLoad.PaintTiming.NavigationToLargestContentfulPaint", "ms", perf.SmallerIsBetter, []int64{300, 2000}),
		cuj.NewCustomMetricConfig("PageLoad.PaintTiming.NavigationToFirstContentfulPaint", "ms", perf.SmallerIsBetter, []int64{300, 2000}),
	}

	url := "https://www.reddit.com/r/technews/hot/"
	secondURL := "https://www.reddit.com/r/technews/new/"
	anchorLink := "/r/technews/new/"

	recorder, err := cuj.NewRecorder(ctx, cr, nil, metrics...)
	if err != nil {
		s.Fatal("Failed to create a CUJ recorder: ", err)
	}
	defer recorder.Close(ctx)

	pv := perf.NewValues()
	if err = recorder.Run(ctx, func(ctx context.Context) error {
		s.Logf("Lanch chrome and load URL %s", url)
		start := time.Now()
		conn, err := cr.NewConn(ctx, url)
		if err != nil {
			s.Fatal("Failed to navigate to youtube: ", err)
		}
		defer conn.Close()
		defer conn.CloseTarget(ctx)

		if err := waitForPageLoad(ctx, conn, start); err != nil {
			return errors.Wrap(err, "failed to wait for page load")
		}
		clearNotificationPrompt(ctx, tconn)
		testing.Sleep(ctx, 5*time.Second) // Keep the page in the UI for 5 seconds.

		start = time.Now()
		if clickAnchor {
			s.Logf("Click anchor %q inside the page", anchorLink)

			if err := clickPageAnchor(ctx, conn, anchorLink); err != nil {
				return errors.Wrap(err, "failed to click anchor link")
			}
		}
		if navigate {
			s.Logf("Navigate to %q in the current tab", secondURL)

			if err := conn.Navigate(ctx, secondURL); err != nil {
				return errors.Wrap(err, "failed to navigate to second URL")
			}
		}
		if clickAnchor || navigate {
			if err := waitForPageLoad(ctx, conn, start); err != nil {
				return errors.Wrap(err, "failed to wait for page load")
			}
			testing.Sleep(ctx, 5*time.Second) // Keep the page in the UI for 5 seconds.
		}

		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the test scenario: ", err)
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to collect the data from the recorder: ", err)
	}
	if err = recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Fatal("Failed to save histogram raw data from the recorder: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to saving perf data: ", err)
	}

	s.Logf("Please check LCP histograms in recorder_histograms.json file in the test result dir under tests/%s", name)
}

func waitForPageLoad(ctx context.Context, conn *chrome.Conn, start time.Time) error {
	loadTimeout := 1 * time.Minute

	if err := webutil.WaitForRender(ctx, conn, loadTimeout); err != nil {
		return errors.Wrapf(err, "failed to wait for page renderring in %s", time.Now().Sub(start))
	}
	testing.ContextLog(ctx, "Page has been rendered within ", time.Now().Sub(start))
	if err := webutil.WaitForQuiescence(ctx, conn, loadTimeout); err != nil {
		return errors.Wrapf(err, "page hasn't achived quiescence after %v", time.Now().Sub(start))
	}
	testing.ContextLog(ctx, "Page has achieved quiescence within ", time.Now().Sub(start))

	return nil
}

func clickPageAnchor(ctx context.Context, conn *chrome.Conn, pattern string) error {
	var url string
	if err := conn.Eval(ctx, "window.location.href", &url); err != nil {
		return errors.Wrap(err, "failed to get URL")
	}
	testing.ContextLogf(ctx, "Current URL: %q", url)

	script := fmt.Sprintf(`() => {
		if (window.location.href !== '%s') {
			return true
		}
		const pattern = '%s';
		const name = "a[href*='" + pattern + "']";
		const els = document.querySelectorAll(name);
		if (els.length > 0) els[0].click();
		return false;
	}`, url, pattern)

	timeout := 90 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		done := false
		if err := conn.Call(ctx, &done, script); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to execute JavaScript query to click HTML link to navigate"))
		}
		if !done {
			return errors.New("javascript exection failed")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to click HTML anchor and navigate within %v", timeout)
	}

	testing.ContextLogf(ctx, "HTML anchor %q clicked, page navigates away from: %q", pattern, url)
	return nil
}

// clearNotificationPrompt finds and clears the web prompts by allowing notification.
// No error is returned because failing to clear the notification doesn't impact the test.
func clearNotificationPrompt(ctx context.Context, tconn *chrome.TestConn) {
	ui := uiauto.New(tconn)

	tartgetPrompt := nodewith.Name("Allow").Role(role.Button)
	if err := ui.IfSuccessThen(
		ui.WithTimeout(10*time.Second).WaitUntilExists(tartgetPrompt),
		ui.LeftClickUntil(tartgetPrompt, ui.WithTimeout(5*time.Second).WaitUntilGone(tartgetPrompt)),
	)(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to clear notification prompt")
	}
}
