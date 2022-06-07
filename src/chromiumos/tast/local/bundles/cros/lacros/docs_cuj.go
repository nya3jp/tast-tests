// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosperf"
	"chromiumos/tast/local/ui/cujrecorder"
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
		Timeout:      chrome.GAIALoginTimeout + 6*time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

const (
	// Google Docs with 20+ pages of random text with 50 comments. The URL points to a comment and
	// will skip down to the comment once the page is fully loaded.
	// The access to this document is restricted to the default pool of GAIA accounts only in order
	// to avoid the "Some tools might be unavailable due to heavy traffic in this file" flakiness.
	docsURLToComment = "https://docs.google.com/document/d/1U6pghj7AaMLnhS7rqQHeecZ7f7fF6bLGaPVxP5xEPuQ/edit?disco=AAAAP6EbSF8"
)

func DocsCUJ(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	opts := []chrome.Option{
		chrome.DisableFeatures("FirmwareUpdaterApp"),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
	}

	cfg := lacrosfixt.NewConfig()
	defaultOpts, err := cfg.Opts()
	if err != nil {
		s.Fatal("Failed to get default options: ", err)
	}
	opts = append(opts, defaultOpts...)

	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, browser.TypeAsh, cfg, opts...)
	if err != nil {
		s.Fatal("Failed to connect to the browser: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

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
	if ashPerfValues, err := runDocsPageLoad(ctx, docsURLToComment, func(ctx context.Context, url string) (*chrome.Chrome, *chrome.Conn, lacrosperf.CleanupCallback, error) {
		conn, cleanup, err := lacrosperf.SetupCrosTestWithPage(ctx, cr, url, lacrosperf.StabilizeAfterOpeningURL)
		return cr, conn, cleanup, err
	}); err != nil {
		s.Error("Failed to run ash-chrome benchmark: ", err)
	} else {
		pv.MergeWithSuffix(".ash", ashPerfValues)
	}

	// Run against lacros.
	if lacrosPerfValues, err := runDocsPageLoad(ctx, docsURLToComment, func(ctx context.Context, url string) (*chrome.Chrome, *chrome.Conn, lacrosperf.CleanupCallback, error) {
		conn, _, _, cleanup, err := lacrosperf.SetupLacrosTestWithPage(ctx, cr, url, lacrosperf.StabilizeAfterOpeningURL)
		return cr, conn, cleanup, err
	}); err != nil {
		s.Error("Failed to run lacros-chrome benchmark: ", err)
	} else {
		pv.MergeWithSuffix(".lacros", lacrosPerfValues)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

// runDocsPageLoad navigates to the Google Docs URL page and benchmark the time to load it.
// It returns the page loading time (loadTime) and the user-visible milestone of loading the page
// (visibleLoadTime), given the latter really captures the real user experience speacially when
// loading large pages.
func runDocsPageLoad(
	ctx context.Context,
	url string,
	setup func(ctx context.Context, url string) (*chrome.Chrome, *chrome.Conn, lacrosperf.CleanupCallback, error)) (*perf.Values, error) {
	cr, conn, cleanup, err := setup(ctx, chrome.BlankURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open a new tab")
	}
	defer cleanup(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test API")
	}

	w, err := ash.WaitForAnyWindowWithTitle(ctx, tconn, "about:blank")
	if err != nil {
		return nil, err
	}

	// Maximize browser window (either ash-chrome or lacros) to ensure a consistent state.
	if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized); err != nil {
		return nil, errors.Wrap(err, "failed to maximize window")
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	testCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	options := cujrecorder.NewPerformanceCUJOptions()
	cujRecorder, err := cujrecorder.NewRecorder(testCtx, cr, nil, options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a CUJ recorder")
	}
	defer func(ctx context.Context) {
		if err := cujRecorder.Close(ctx); err != nil {
			testing.ContextLog(ctx, "ERROR: Failed to close recorder: ", err)
		}
	}(closeCtx)

	// The callback that is passed as a second parameter to the
	// cujRecorder.Run() function is the main focus point of this test. It
	// runs the test scenario and the TPS score will be calculated
	// automatically by the cujrecorder. The performance metrics are
	// calculated below after the cujRecorder.Run().
	var loadTime time.Duration
	var visibleLoadTime time.Duration
	if err := cujRecorder.Run(testCtx, func(ctx context.Context) error {
		start := time.Now()

		// Navigate the blankpage to the document file to be loaded.
		// This blocks until the loading is completed and is a important metric already.
		if err := conn.Navigate(ctx, url); err != nil {
			return errors.Wrap(err, "failed to navigate a blankpage to the URL")
		}

		// Save load time perf data as well.
		loadTime = time.Since(start)

		// Check whether comment link is loaded and visible.
		// WaitForExpr has to be used since the comment link is not updated immediately.
		const expr = `document.querySelector("#docos-stream-view > div.docos-docoview-tesla-conflict.docos-docoview-resolve-button-visible.docos-anchoreddocoview.docos-docoview-active.docos-docoview-active-experiment")
		.innerText`
		if err := conn.WaitForExpr(ctx, expr); err != nil {
			return errors.Wrap(err, "failed to wait the comment link to be loaded and visible")
		}

		visibleLoadTime = time.Since(start)
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to run the test scenario")
	}

	pv := perf.NewValues()

	pv.Set(perf.Metric{
		Name:      "docs.load",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}, time.Duration(loadTime).Seconds())

	pv.Set(perf.Metric{
		Name:      "docs.load_and_visible",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}, time.Duration(visibleLoadTime).Seconds())

	if err := cujRecorder.Record(testCtx, pv); err != nil {
		return nil, errors.Wrap(err, "failed to collect the data from the recorder")
	}

	return pv, nil
}
