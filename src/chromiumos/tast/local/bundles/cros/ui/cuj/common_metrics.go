// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/ui/cujrecorder"
)

// AddPerformanceCUJMetrics adds the metrics to the recorder for performance CUJ test.
func AddPerformanceCUJMetrics(tconn, bTconn *chrome.TestConn, recorder *cujrecorder.Recorder) error {
	lacroshMetrics := cujrecorder.BrowserCommonMetricConfigs()
	ashMetrics := cujrecorder.AshCommonMetricConfigs()
	commonMetrics := cujrecorder.AnyChromeCommonMetricConfigs()

	// Collect all metrics from all browsers to make it compatible with the CUJ scores generated
	// from previouse releases, which collects all metrics for all system activities.
	allMetrics := append(commonMetrics, append(lacroshMetrics, ashMetrics...)...)
	if err := recorder.AddCollectedMetrics(tconn, allMetrics...); err != nil {
		errors.Wrap(err, "failed to add metrics for tconn")
	}
	if bTconn != nil {
		if err := recorder.AddCollectedMetrics(bTconn, allMetrics...); err != nil {
			errors.Wrap(err, "failed to add metrics for bTconn")
		}
	}
	return nil
}
