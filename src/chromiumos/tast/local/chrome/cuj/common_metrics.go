// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/ui/cujrecorder"
)

// AddPerformanceCUJMetrics adds the metrics to the recorder for performance CUJ test.
func AddPerformanceCUJMetrics(bt browser.Type, tconn, bTconn *chrome.TestConn, recorder *cujrecorder.Recorder) error {
	ashMetrics := cujrecorder.AshCommonMetricConfigs()
	lacrosMetrics := cujrecorder.CUJLacrosCommonMetricConfigs()
	browserMetrics := cujrecorder.BrowserCommonMetricConfigs()
	commonMetrics := cujrecorder.AnyChromeCommonMetricConfigs()

	// Collect all metrics from all browsers to make it compatible with the CUJ scores generated
	// from previouse releases, which collects all metrics for all system activities.
	allMetrics := append(commonMetrics, append(ashMetrics, browserMetrics...)...)
	if err := recorder.AddCollectedMetrics(tconn, browser.TypeAsh, allMetrics...); err != nil {
		errors.Wrap(err, "failed to add metrics for tconn")
	}
	if bt == browser.TypeLacros {
		allMetrics = append(allMetrics, lacrosMetrics...)
		if err := recorder.AddCollectedMetrics(bTconn, browser.TypeLacros, allMetrics...); err != nil {
			errors.Wrap(err, "failed to add metrics for bTconn")
		}
	}
	return nil
}
