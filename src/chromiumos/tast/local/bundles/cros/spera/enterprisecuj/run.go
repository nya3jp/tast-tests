// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package enterprisecuj contains the test code for enterprise CUJ.
package enterprisecuj

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	cx "chromiumos/tast/local/bundles/cros/spera/enterprisecuj/citrix"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

// TestParams stores data common to the tests run in this package.
type TestParams struct {
	OutDir          string
	CitrixUserName  string
	CitrixPassword  string
	CitrixServerURL string
	DesktopName     string
	TabletMode      bool
	TestMode        cx.TestMode
	DataPath        func(string) string
	UIHandler       cuj.UIActionHandler
}

// Run runs the enterprisecuj test.
func Run(ctx context.Context, cr *chrome.Chrome, scenario CitrixScenario, p *TestParams) (retErr error) {
	const desktopTitle = "VDA"
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	// Give 10 seconds to set initial settings. It is critical to ensure
	// cleanupSetting can be executed with a valid context so it has its
	// own cleanup context from other cleanup functions. This is to avoid
	// other cleanup functions executed earlier to use up the context time.
	cleanupSettingsCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanupSetting, err := cuj.InitializeSetting(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to set initial settings")
	}
	defer cleanupSetting(cleanupSettingsCtx)

	testing.ContextLog(ctx, "Start to get browser start time")
	_, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, p.TabletMode, browser.TypeAsh)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}
	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, tconn, nil, options)
	if err != nil {
		return errors.Wrap(err, "failed to create a recorder")
	}
	defer recorder.Close(cleanupCtx)
	if err := cuj.AddPerformanceCUJMetrics(browser.TypeAsh, tconn, nil, recorder); err != nil {
		return errors.Wrap(err, "failed to add metrics to recorder")
	}

	citrix := cx.NewCitrix(tconn, kb, p.DataPath, desktopTitle, p.TabletMode, p.TestMode)
	if err := uiauto.NamedCombine("open and login citrix",
		citrix.Open(),
		citrix.Login(p.CitrixServerURL, p.CitrixUserName, p.CitrixPassword),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to login Citrix")
	}
	defer citrix.Close(ctx)
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, p.OutDir, func() bool { return retErr != nil }, cr, "ui_dump")

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		return scenario.Run(ctx, tconn, kb, citrix, p)
	}); err != nil {
		return errors.Wrap(err, "failed to run the clinician workstation cuj")
	}

	if p.TestMode == cx.RecordMode {
		if err := citrix.SaveRecordFile(ctx, p.OutDir); err != nil {
			return err
		}
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))
	appStartTime := citrix.AppStartTime()
	if appStartTime > 0 {
		pv.Set(perf.Metric{
			Name:      "Apps.StartTime",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, float64(appStartTime))
	}

	if err := recorder.Record(ctx, pv); err != nil {
		return errors.Wrap(err, "failed to record")
	}
	if err = pv.Save(p.OutDir); err != nil {
		return errors.Wrap(err, "failed to store values")
	}
	if err := recorder.SaveHistograms(p.OutDir); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}
	return nil
}
