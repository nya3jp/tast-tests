// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gamecuj contains the test code for Game CUJ.
package gamecuj

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// Run runs the GameCUJ test.
func Run(ctx context.Context, s *testing.State, cr *chrome.Chrome, game GameApp, tabletMode bool) (retErr error) {
	outDir := s.OutDir()
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer func(ctx context.Context) {
		// The screenshot and ui tree dump must been taken before game app is closed.
		faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "ui_dump")
		game.End(ctx)
	}(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	if err := game.Install(ctx); err != nil {
		return errors.Wrap(err, "failed to install Game")
	}

	testing.ContextLog(ctx, "Start to get browser start time")
	browserStartTime, err := cuj.GetBrowserStartTime(ctx, cr, tconn, tabletMode)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}

	recorder, err := cuj.NewRecorder(ctx, cr, nil, cuj.MetricConfigs()...)
	if err != nil {
		return errors.Wrap(err, "failed to create a recorder")
	}
	defer recorder.Close(cleanupCtx)

	// Put battery under discharge in order to collect the power consumption of the test.
	setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 50)
	if err != nil {
		return errors.Wrap(err, "failed to set battery discharge")
	}
	// It's important to ensure setBatteryNormal will be called after the test is done.
	// So make it the first deferred function to use cleanupCtx.
	defer setBatteryNormal(cleanupCtx)

	var appStartTime int64
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		t, err := game.Launch(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to launch game")
		}
		appStartTime = t.Milliseconds()

		return game.Play(ctx, s, tconn)
	}); err != nil {
		return errors.Wrap(err, "failed to run the game scenario")
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))
	if appStartTime > 0 {
		pv.Set(perf.Metric{
			Name:      "Apps.StartTime",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, float64(appStartTime))
	}

	// Use a short timeout value so it can return fast in case of failure.
	recordCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := recorder.Record(recordCtx, pv); err != nil {
		return errors.Wrap(err, "failed to report")
	}
	if err = pv.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to store values")
	}
	if err := recorder.SaveHistograms(outDir); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}

	return nil
}
