// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros/lacrosperf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DocsCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Runs Google Docs CUJ against both ash-chrome and lacros-chrome",
		Contacts:     []string{"hidehiko@chromium.org", "tvignatti@igalia.com", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Timeout:      6 * time.Minute,
		Fixture:      "lacros",
	})
}

const (
	// Google Docs with 20+ pages of random text with 50 comments. The URL points to a comment and will skip
	// down to the comment once the page is fully loaded.
	docsURLToComment = "https://docs.google.com/document/d/1MW7lAk9RZ-6zxpObNwF0r80nu-N1sXo5f7ORG4usrJQ/edit?disco=AAAAP6EbSF8"
)

func DocsCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := lacrosperf.SetupPerfTest(ctx, tconn, "lacros.DocsCUJ")
	if err != nil {
		s.Fatal("Failed to set up lacros perf test: ", err)
	}
	defer cleanup(ctx)

	pv := perf.NewValues()

	// Run against ash-chrome.
	if loadTime, visibleLoadTime, err := runDocsPageLoad(ctx, tconn, docsURLToComment, func(ctx context.Context, url string) (*chrome.Conn, lacrosperf.CleanupCallback, error) {
		return lacrosperf.SetupCrosTestWithPage(ctx, cr, url, lacrosperf.StabilizeAfterOpeningURL)
	}); err != nil {
		s.Error("Failed to run ash-chrome benchmark: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "docs.load.ash",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, loadTime.Seconds())

		pv.Set(perf.Metric{
			Name:      "docs.load_and_visible.ash",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, visibleLoadTime.Seconds())
	}

	// Run against lacros.
	if loadTime, visibleLoadTime, err := runDocsPageLoad(ctx, tconn, docsURLToComment, func(ctx context.Context, url string) (*chrome.Conn, lacrosperf.CleanupCallback, error) {
		conn, _, _, cleanup, err := lacrosperf.SetupLacrosTestWithPage(ctx, cr, url, lacrosperf.StabilizeAfterOpeningURL)
		return conn, cleanup, err
	}); err != nil {
		s.Error("Failed to run lacros-chrome benchmark: ", err)
	} else {
		pv.Set(perf.Metric{
			Name:      "docs.load.lacros",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, loadTime.Seconds())

		pv.Set(perf.Metric{
			Name:      "docs.load_and_visible.lacros",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, visibleLoadTime.Seconds())
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

// runDocsPageLoad navigates to the Google Docs URL page and benchmark the time to load it.
// It returns the page loading time (loadTime) and the user-visible milestone of loading the page
// (visibleLoadTime), given the latter really captures the real user experience speacially when
// loading large pages.
// tconn is a test connection to the ash-chrome test connection.
func runDocsPageLoad(
	ctx context.Context,
	tconn *chrome.TestConn,
	url string,
	setup func(ctx context.Context, url string) (*chrome.Conn, lacrosperf.CleanupCallback, error)) (time.Duration, time.Duration, error) {
	conn, cleanup, err := setup(ctx, chrome.BlankURL)
	if err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to open a new tab")
	}
	defer cleanup(ctx)

	w, err := ash.WaitForAnyWindowWithTitle(ctx, tconn, "about:blank")
	if err != nil {
		return 0.0, 0.0, err
	}

	// Maximize browser window (either ash-chrome or lacros) to ensure a consistent state.
	if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized); err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to maximize window")
	}

	start := time.Now()

	// Navigate the blankpage to the document file to be loaded.
	// This blocks until the loading is completed and is a important metric already.
	if err := conn.Navigate(ctx, url); err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to navigate a blankpage to the URL")
	}

	// Save load time perf data as well.
	loadTime := time.Since(start)

	// Check whether comment link is loaded and visible.
	// WaitForExpr has to be used since the comment link is not updated immediately.
	const expr = `document.querySelector("#docos-stream-view > div.docos-docoview-tesla-conflict.docos-docoview-resolve-button-visible.docos-anchoreddocoview.docos-docoview-active.docos-docoview-active-experiment")
	.innerText`
	if err := conn.WaitForExpr(ctx, expr); err != nil {
		return 0.0, 0.0, errors.Wrap(err, "failed to wait the comment link to be loaded and visible")
	}

	visibleLoadTime := time.Since(start)

	return time.Duration(loadTime), time.Duration(visibleLoadTime), nil
}
