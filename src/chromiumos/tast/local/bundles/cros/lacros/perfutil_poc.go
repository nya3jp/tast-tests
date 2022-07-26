// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfutilPoc,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "PoC for moving bundles/cros/ui/perfutil some levels up to share its code with bundles/cros/lacros",
		Contacts:     []string{"tvignatti@igalia.com", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Timeout:      chrome.GAIALoginTimeout + 10*time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

func PerfutilPoc(ctx context.Context, s *testing.State) {
	const (
		iterationCount = 2

		docsURLToComment = "https://docs.google.com/document/d/1U6pghj7AaMLnhS7rqQHeecZ7f7fF6bLGaPVxP5xEPuQ/edit?disco=AAAAP6EbSF8"
	)

	cleanupCtx := ctx
	opts := []chrome.Option{
		chrome.DisableFeatures("FirmwareUpdaterApp"),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
	}

	opts, err := lacrosfixt.NewConfig(lacrosfixt.ChromeOptions(opts...)).Opts()
	if err != nil {
		s.Fatal("Failed to get default options: ", err)
	}

	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, browser.TypeAsh, lacrosfixt.NewConfig(), opts...)
	if err != nil {
		s.Fatal("Failed to connect to the browser: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := lacrosperf.SetupPerfTest(ctx, tconn, "lacros.PerfutilPoc")
	if err != nil {
		s.Fatal("Failed to set up lacros perf test: ", err)
	}
	defer cleanup(ctx)

	pvv := perfutil.RunMultiple(ctx, s, cr.Browser(), perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
		// Create a new desk other than the default desk, activate it, then remove it.
		if err = ash.CreateNewDesk(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create a new desk")
		}
		if err = ash.ActivateDeskAtIndex(ctx, tconn, 1); err != nil {
			return errors.Wrap(err, "failed to activate the second desk")
		}
		if err = ash.RemoveActiveDesk(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to remove the active desk")
		}
		return nil
	},
		"Ash.Desks.AnimationSmoothness.DeskActivation",
		"Ash.Desks.AnimationSmoothness.DeskRemoval"),
		perfutil.StoreSmoothness)

	if err := pvv.Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}

	pv := perf.NewValues()

	singleMetrics := make(map[perf.Metric][]float64)

	var variantPv *perf.Values
	for i := 0; i < iterationCount; i++ {
		testing.ContextLogf(ctx, "Running lacros browser, iteration %d/%d", i+1, iterationCount)
		if variantPv, err = runPerfutilPageLoad(ctx, docsURLToComment, func(ctx context.Context, url string) (*chrome.Chrome, *chrome.Conn, lacrosperf.CleanupCallback, error) {
			conn, _, _, cleanup, err := lacrosperf.SetupLacrosTestWithPage(ctx, cr, url, lacrosperf.StabilizeAfterOpeningURL)
			return cr, conn, cleanup, err
		}); err != nil {
			s.Error("Failed to run lacros-chrome benchmark: ", err)
		} else {
			appendPerfutilValues(ctx, variantPv, "lacros", pv, singleMetrics)
		}
	}

	for k, values := range singleMetrics {
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		pv.Set(k, sum/float64(len(values)))
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}

func runPerfutilPageLoad(
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

	cujRecorder, err := cujrecorder.NewRecorder(testCtx, cr, nil, cujrecorder.NewPerformanceCUJOptions())
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
		Multiple:  true,
	}, time.Duration(loadTime).Seconds())

	pv.Set(perf.Metric{
		Name:      "docs.load_and_visible",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, time.Duration(visibleLoadTime).Seconds())

	if err := cujRecorder.Record(testCtx, pv); err != nil {
		return nil, errors.Wrap(err, "failed to collect the data from the recorder")
	}

	return pv, nil
}

func appendPerfutilValues(ctx context.Context, variantPv *perf.Values, suffix string, pv *perf.Values, singleMetrics map[perf.Metric][]float64) {
	for _, m := range variantPv.Proto().GetValues() {
		metric := perf.Metric{
			Name:      m.Name + "." + suffix,
			Unit:      m.Unit,
			Direction: perf.Direction(m.Direction),
			Multiple:  m.Multiple,
		}
		if m.Multiple {
			pv.Append(metric, m.Value...)
		} else {
			singleMetrics[metric] = append(singleMetrics[metric], m.Value...)
		}
	}
}
